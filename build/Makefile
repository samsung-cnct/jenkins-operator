GOCMD=go
DOCKERTAG?=quay.io/samsung_cnct/jenkins-operator:local
GOBUILD=$(GOCMD) build
GOGET=$(GOCMD) get
GOTEST=$(GOCMD) test
PROJECTDIR=$(shell pwd)
DEPCMD=dep

# Variables for binary version information
GIT_COMMIT=$(shell git rev-parse HEAD)
APP_VERSION=$(shell cat .versionfile)
BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Associating version information variables with data
VERSION_PACKAGE=github.com/maratoid/jenkins-operator/pkg/version
LD_DATA_GIT_COMMIT=-X ${VERSION_PACKAGE}.gitCommit=${GIT_COMMIT}
LD_DATA_VERSION=-X ${VERSION_PACKAGE}.version=${APP_VERSION}
LD_DATA_BUILD_DATE=-X ${VERSION_PACKAGE}.buildDate=${BUILD_DATE}
LD_DATA=${LD_DATA_GIT_COMMIT} ${LD_DATA_VERSION} ${LD_DATA_BUILD_DATE}

# Creating static binary
STATIC_FLAGS=-w -extldflags "-static"

# Generating ldflag string
LDFLAGS='${STATIC_FLAGS} ${LD_DATA}'

# Location of
OPERATOR_BINARY_NAME=jenkins-operator
OPERATOR_SOURCE_FILE=cmd/manager/main.go

# build binary command
BUILD_OPERATOR_BINARY_CMD=${GOBUILD} -a -tags netgo -ldflags ${LDFLAGS} -o ${OPERATOR_BINARY_NAME} ${OPERATOR_SOURCE_FILE}

all: test manager-linux-amd64
linux: test manager-linux-amd64
darwin: test manager-darwin-amd64
container: docker-build

# generate bindata
bin-data:
	go-bindata -o $(PROJECTDIR)/pkg/configdata/data.go -ignore data\\.go -ignore \\.DS_Store -mode 511 -nometadata -pkg configdata -prefix $(PROJECTDIR)/pkg/configdata $(PROJECTDIR)/pkg/configdata/...
	go-bindata -o $(PROJECTDIR)/pkg/crddata/data.go -ignore data\\.go -ignore \\.DS_Store -mode 511 -nometadata -pkg crddata -prefix $(PROJECTDIR)/crd $(PROJECTDIR)/crd/...

# Run tests
test: manifests bin-data generate fmt vet
	export JENKINS_OPERATOR_TESTRUN=true; \
	ginkgo -r ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager-linux-amd64: manifests bin-data generate fmt vet
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 ${BUILD_OPERATOR_BINARY_CMD}
manager-darwin-amd64: manifests bin-data generate fmt vet
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 ${BUILD_OPERATOR_BINARY_CMD}

# Run against the configured Kubernetes cluster in ~/.kube/config
run: manifests bin-data generate fmt vet
	go run ./cmd/manager/main.go

# Generate manifests e.g. CRD, RBAC etc.
# generate parts of helm chart
manifests:
	go run $(PROJECTDIR)/vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go crd --output-dir $(PROJECTDIR)/crd
	go run $(PROJECTDIR)/vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go rbac --name rbac --output-dir $(PROJECTDIR)/rbac
	output=$$(kustomize build build/kustomize/rbac/role); echo "$$output" > $(PROJECTDIR)/deployments/helm/jenkins-operator/RBAC/rbac_role.yaml
	output=$$(kustomize build build/kustomize/rbac/rolebinding); echo "$$output" > $(PROJECTDIR)/deployments/helm/jenkins-operator/RBAC/rbac_role_binding.yaml
	output=$$(kustomize build build/kustomize/crd/protected/instances); echo "$$output" > $(PROJECTDIR)/deployments/helm/jenkins-operator/CRD-protected/jenkins_v1alpha1_jenkinsinstance.yaml
	output=$$(kustomize build build/kustomize/crd/protected/jobs); echo "$$output" > $(PROJECTDIR)/deployments/helm/jenkins-operator/CRD-protected/jenkins_v1alpha1_jenkinsjob.yaml
	output=$$(kustomize build build/kustomize/crd/unprotected/instances); echo "$$output" > $(PROJECTDIR)/deployments/helm/jenkins-operator/CRD/jenkins_v1alpha1_jenkinsinstance.yaml
	output=$$(kustomize build build/kustomize/crd/unprotected/jobs); echo "$$output" > $(PROJECTDIR)/deployments/helm/jenkins-operator/CRD/jenkins_v1alpha1_jenkinsjob.yaml

# label hack for helm templates

# Run go fmt against code
fmt:
	go fmt ./pkg/... ./cmd/...

# Run go vet against code
vet:
	go vet ./pkg/... ./cmd/...

# Generate code
generate:
	go generate ./pkg/... ./cmd/...

# Install dep will install dependencies
install-dep:
	go get -u github.com/golang/dep/cmd/dep
	go get -u github.com/go-bindata/go-bindata/...
	go get -u github.com/onsi/ginkgo/ginkgo
	go get -u github.com/onsi/gomega/...

# New Dep will just blow away Gopkg, vendor and do a dep init
new-dep:
	rm -rf vendor Gopkg.toml Gopkg.lock
	$(DEPCMD) init

# For builds where we're just going to go along with whatever Gopkg is saying
dep-ensure:
	$(DEPCMD) ensure

# Build the docker image
docker-build:
	docker build $(PROJECTDIR) -f $(PROJECTDIR)/build/docker/jenkins-operator/Dockerfile -t $(DOCKERTAG)

# Push the docker image
docker-push:
	docker push ${IMG}
