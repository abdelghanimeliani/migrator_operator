# Build the manager binary
FROM golang:1.19 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download
RUN  go get -u github.com/proglottis/gpgme
RUN apt update
RUN yes | apt install libbtrfs-dev libgpgme-dev libdevmapper-dev
# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY models/ models/

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
From ubuntu
RUN apt update
RUN apt install -y libbtrfs-dev libgpgme-dev libdevmapper-dev  runc
RUN apt install -y fuse-overlayfs --exclude container-selinux; rm -rf /var/cache /var/log/dnf* /var/log/yum.*

# FROM fedora:latest

# # Don't include container-selinux and remove
# # directories used by dnf that are just taking
# # up space.
# RUN yum -y install buildah fuse-overlayfs --exclude container-selinux; rm -rf /var/cache /var/log/dnf* /var/log/yum.*
# RUN dnf install -y btrfs-progs-devel gpgme-devel device-mapper-devel

# # Adjust storage.conf to enable Fuse storage.
RUN touch /etc/containers/storage.conf


RUN sed -i -e 's|^#mount_program|mount_program|g' -e '/additionalimage.*/a "/var/lib/shared",' /etc/containers/storage.conf

RUN mkdir -p /var/lib/shared/overlay-images /var/lib/shared/overlay-layers; touch /var/lib/shared/overlay-images/images.lock; touch /var/lib/shared/overlay-layers/layers.lock
# Set up environment variables to note that this is
# not starting with user namespace and default to
# isolate the filesystem with chroot.
ENV _BUILDAH_STARTED_IN_USERNS="" BUILDAH_ISOLATION=chroot
COPY --from=builder /workspace/manager .



ENTRYPOINT ["/manager"]