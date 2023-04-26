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
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
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
	"github.com/containers/image/v5/types"
	"github.com/containers/storage"
	dockertypes "github.com/docker/docker/api/types"
	d "github.com/docker/docker/client"
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
		println("********his is from the get*********", err)
	}
	podName := &migrator.Spec.SourcePodName
	containerName := &migrator.Spec.SourcePodContainer
	sourcePodNamespace := &migrator.Spec.SourcePodNameSpace

	println("these are the source pod infrmations :", *sourcePodNamespace, *podName, *containerName)

	// Load client certificate and key
	cert, err := tls.LoadX509KeyPair("/etc/kubernetes/pki/apiserver-kubelet-client.crt", "/etc/kubernetes/pki/apiserver-kubelet-client.key")
	if err != nil {
		panic(err)
	}
	// Load CA certificate
	caCert, err := ioutil.ReadFile("/etc/kubernetes/pki/ca.crt")
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

	// Send HTTPS POST request
	url := "https://kubemasterfedora:10250/checkpoint/" + *sourcePodNamespace + "/" + *podName + "/" + *containerName
	postRequest, err := http.NewRequest("POST", url, nil)
	if err != nil {
		panic(err)
	}
	resp, err := httpClient.Do(postRequest)
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

	var items models.CheckpointResponse

	if err := json.Unmarshal(body, &items); err != nil { // Parse []byte to the go struct pointer
		fmt.Println("Can not unmarshal JSON")
	}

	checkpointPath := items.Path[0]
	fmt.Println("the checkpoint path is : ", checkpointPath)

	fmt.Println("checking done ... ✅")
	// trying to build

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
		print("1================================================================================")
		panic(err)
	}
	capabilitiesForRoot, err := conf.Capabilities("root", nil, nil)
	if err != nil {
		print("2================================================================================")
		panic(err)
	}
	// Create storage reference
	imageRef, err := is.Transport.ParseStoreReference(buildStore, "localhost/restore-counter")
	if err != nil {
		panic(errors.New("failed to parse image name"))
	}

	// Build an image scratch
	builderOptions := buildah.BuilderOptions{
		FromImage:    "scratch",
		Capabilities: capabilitiesForRoot,
	}
	importBuilder, err := buildah.NewBuilder(context.TODO(), buildStore, builderOptions)
	if err != nil {
		print("creation of builder object failed: ", err)
		panic(err)

	}
	// Clean up buildah working container
	defer func() {
		if err := importBuilder.Delete(); err != nil {
			logrus.Errorf("Image builder delete failed: %v", err)
		}
	}()

	// Copy checkpoint from temporary tar file in the image
	addAndCopyOptions := buildah.AddAndCopyOptions{}
	if err := importBuilder.Add("", false, addAndCopyOptions, checkpointPath); err != nil {
		fmt.Println(checkpointPath)
		fmt.Println("add failed:", err)
		panic(err)
	}

	importBuilder.SetAnnotation("io.kubernetes.cri-o.annotations.checkpoint.name", "counter")
	commitOptions := buildah.CommitOptions{
		Squash:        true,
		SystemContext: &types.SystemContext{},
	}

	// Create checkpoint image
	id, _, _, err := importBuilder.Commit(context.TODO(), imageRef, commitOptions)
	if err != nil {
		print("commit failed: ", err)
		panic(err)

	}
	fmt.Println("build  done ... ✅")
	fmt.Println("image id :", id)

	//end of the build
	//trying to push

	fmt.Println("trying to push the image to the registry")
	cli, err := d.NewClientWithOpts(d.FromEnv, d.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Println(err.Error())
		panic(err)
	}

	var authConfig = dockertypes.AuthConfig{
		Username:      "abdelghanimeliani",
		Password:      "abgmelesi03101902",
		ServerAddress: "https://quay.io",
	}
	authConfigBytes, _ := json.Marshal(authConfig)
	authConfigEncoded := base64.URLEncoding.EncodeToString(authConfigBytes)

	opts := dockertypes.ImagePushOptions{RegistryAuth: authConfigEncoded}
	rd, err := cli.ImagePush(ctx, "abdelghanimeliani/restore-counter", opts)
	if err != nil {
		println("failed to push : ", err)
	}
	defer rd.Close()

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MigritorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.Migritor{}).
		Complete(r)
}
