/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"time"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	cachev1alpha1 "github.com/abdelghanimeliani/migrator_operator/api/v1alpha1"
	"github.com/abdelghanimeliani/migrator_operator/models"
	"github.com/containers/buildah"
	"github.com/containers/common/pkg/config"
	is "github.com/containers/image/v5/storage"
	"github.com/containers/image/v5/transports/alltransports"
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// MigritorReconciler reconciles a Migritor object
type MigritorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cache.eurecom.com,resources=migritors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.eurecom.com,resources=migritors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cache.eurecom.com,resources=migritors/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Migritor object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *MigritorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	migrator := &cachev1alpha1.Migritor{}
	err := r.Get(ctx, req.NamespacedName, migrator)
	if err != nil {
		println("this is from the Get request", err)
	}
	podName := &migrator.Spec.SourcePodName
	containerName := &migrator.Spec.SourcePodContainer
	sourcePodNamespace := &migrator.Spec.SourcePodNameSpace
	sourceNodeid := &migrator.Spec.SourceNodeId

	println("these are the source pod infrmations :", *sourcePodNamespace, *podName, *containerName)

	// Load client certificate and key
	println("Loading the client key and certificate")
	cert, err := tls.LoadX509KeyPair("/vagrant/pki/apiserver-kubelet-client.crt", "/vagrant/pki/apiserver-kubelet-client.key")
	if err != nil {
		panic(err)
	}
	// Load CA certificate
	caCert, err := ioutil.ReadFile("/vagrant/pki/ca.crt")
	if err != nil {
		panic(err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Create HTTPS client with certificate and key authentication
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}
	transport := &http.Transport{TLSClientConfig: tlsConfig}
	httpClient := &http.Client{Transport: transport}

	println("Start Checkpointing ...")
	start := time.Now()
	// Send HTTPS POST request
	checkpointurl := "https://" + *sourceNodeid + ":10250/checkpoint/" + *sourcePodNamespace + "/" + *podName + "/" + *containerName
	checkpointPostRequest, err := http.NewRequest("POST", checkpointurl, nil)
	if err != nil {
		panic(err)
	}
	resp, err := httpClient.Do(checkpointPostRequest)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// Print response status code and body
	fmt.Println(resp.Status)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(body))

	checkpointTime:= time.Since(start)
	var items models.CheckpointResponse

	if err := json.Unmarshal(body, &items); err != nil { // Parse []byte to the go struct pointer
		fmt.Println("Can not unmarshal JSON")
	}

	checkpointPath := items.Path[0]
	fmt.Println("the checkpoint path is : ", checkpointPath)

	fmt.Println("checkpointing done ... ✅")
	fmt.Println("checkpoint time is : ", checkpointTime)


	// trying to build

	startbuild := time.Now()
	fmt.Println("start building ...")

	buildStoreOptions, err := storage.DefaultStoreOptionsAutoDetectUID()
	if err != nil {
		panic(err)
	}

	buildStore, err := storage.GetStore(buildStoreOptions)
	if err != nil {
		panic(err)
	}
	println("this is the buildstore object", buildStore)
	defer buildStore.Shutdown(false)

	conf, err := config.Default()
	if err != nil {
		panic(err)
	}
	capabilitiesForRoot, err := conf.Capabilities("root", nil, nil)
	if err != nil {
		panic(err)
	}
	imageRef, err := is.Transport.ParseStoreReference(buildStore, "localhost/built_with_oprator")
	if err != nil {
		print("failed to parse image name")
		panic(errors.New("failed to parse image name"))

	}

	// Build an image scratch
	builderOptions := buildah.BuilderOptions{
		FromImage:    "scratch",
		Capabilities: capabilitiesForRoot,
	}
	importBuilder, err := buildah.NewBuilder(context.TODO(), buildStore, builderOptions)
	if err != nil {
		print("failed to create a builder object")
		panic(err)

	}
	// Clean up buildah working container
	defer func() {
		if err := importBuilder.Delete(); err != nil {
			logrus.Errorf("Image builder delete failed: %v", err)
		}
	}()

	addAndCopyOptions := buildah.AddAndCopyOptions{}
	if err := importBuilder.Add("", true, addAndCopyOptions, checkpointPath); err != nil {
		fmt.Println("can not add the checkpoint to the container")
		fmt.Println("this is the error", err)
		panic(err)
	}

	importBuilder.SetAnnotation("io.kubernetes.cri-o.annotations.checkpoint.name", containerName)
	commitOptions := buildah.CommitOptions{
		Squash:        true,
		SystemContext: &types.SystemContext{},
	}

	// Create checkpoint image
	id, _, _, err := importBuilder.Commit(context.TODO(), imageRef, commitOptions)
	buildTime:= time.Since(startbuild)
	if err != nil {
		print("can not commit the image")
		panic(err)
	}
	logrus.Debugf("Created checkpoint image: %s", id)
	fmt.Println("build finish successfully ✅")
	fmt.Println("build time is : ", buildTime)

	//end of the build

	//start pushing
	startPush := time.Now()
	destImageRef, err := alltransports.ParseImageName("docker://" + migrator.Spec.Destination)

	if err != nil {

		fmt.Println(err)
	}

	// Build an image scratch
	s1, s2, err := buildah.Push(context.TODO(), "localhost/built_with_oprator", destImageRef, buildah.PushOptions{

		SystemContext: &types.SystemContext{
			DockerDaemonInsecureSkipTLSVerify: true,
			DockerInsecureSkipTLSVerify:       types.OptionalBoolTrue,
			OCIInsecureSkipTLSVerify:          true,
			DockerAuthConfig: &types.DockerAuthConfig{
				Username: migrator.Spec.RegistryUsername,
				Password: migrator.Spec.RegistryPassword,
			},
		},
		Store: buildStore,
	})
	pushTime:= time.Since(startPush)
	if err != nil {
		fmt.Println("can't push the image :", err)
		panic(err)
	}

	fmt.Println("push finish successfully ✅")

	fmt.Println("time to push image: ", pushTime)

    totalTime:= time.Since(start)
	fmt.Println("total time is", totalTime)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigritorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.Migritor{}).
		Complete(r)
}