GOCMD=go
DOCKERTAG?=jenkins-operator:local
GOBUILD=$(GOCMD) build
GOGET=$(GOCMD) get
GOTEST=$(GOCMD) test
PROJECTDIR = $(shell pwd)
DEPCMD=dep

# Variables for binary version information
GIT_COMMIT=$(shell git rev-parse HEAD)
GIT_TAG=$(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_DIRTY=$(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")
BUILD_DATE=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# Associating version information variables with data
VERSION_PACKAGE=github.com/maratoid/jenkins-operator/pkg/version
LD_DATA_GIT_COMMIT=-X ${VERSION_PACKAGE}.gitCommit=${GIT_COMMIT}
LD_DATA_GIT_TAG=-X ${VERSION_PACKAGE}.gitVersion=${GIT_TAG}
LD_DATA_GIT_DIRTY=-X ${VERSION_PACKAGE}.gitTreeState=${GIT_DIRTY}
LD_DATA_BUILD_DATE=-X ${VERSION_PACKAGE}.buildDate=${BUILD_DATE}
LD_DATA=${LD_DATA_GIT_COMMIT} ${LD_DATA_GIT_TAG} ${LD_DATA_GIT_DIRTY} ${LD_DATA_BUILD_DATE}

# Creating static binary
STATIC_FLAGS=-w -extldflags "-static"

# Generating ldflag string
LDFLAGS = '${STATIC_FLAGS} ${LD_DATA}'

# Location of
OPERATOR_BINARY_NAME=jenkins-operator
OPERATOR_SOURCE_FILE=cmd/manager/main.go

# build binary command
BUILD_OPERATOR_BINARY_CMD=${GOBUILD} -a -tags netgo -ldflags ${LDFLAGS} -o ${OPERATOR_BINARY_NAME} ${OPERATOR_SOURCE_FILE}

all: bin-data test manager-linux-amd64
linux: bin-data test manager-linux-amd64
darwin: bin-data test manager-darwin-amd64
container: bin-data docker-build

# generate bindata
bin-data:
	go-bindata -o $(PROJECTDIR)/pkg/bindata/data.go -ignore data\\.go -pkg bindata -prefix $(PROJECTDIR)/pkg/bindata $(PROJECTDIR)/pkg/bindata/...

# Run tests
test: generate fmt vet manifests
	export JENKINS_OPERATOR_TESTRUN=true; \
	ginkgo -r ./pkg/... ./cmd/... -coverprofile cover.out

# Build manager binary
manager-linux-amd64: generate fmt vet
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 ${BUILD_OPERATOR_BINARY_CMD}
manager-darwin-amd64: generate fmt vet
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 ${BUILD_OPERATOR_BINARY_CMD}

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet
	go run ./cmd/manager/main.go

# Install CRDs into a cluster
install: manifests
	kubectl apply -f config/crds

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests
	kubectl apply -f config/crds
	kustomize build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests:
	go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
	go-bindata -o $(PROJECTDIR)/config/crds/data.go -ignore data\\.go -pkg crds -prefix $(PROJECTDIR)/config/crds $(PROJECTDIR)/config/crds/...

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
	go install github.com/onsi/ginkgo/ginkgo

# New Dep will just blow away Gopkg, vendor and do a dep init
new-dep:
	rm -rf vendor Gopkg.toml Gopkg.lock
	$(DEPCMD) init

# For builds where we're just going to go along with whatever Gopkg is saying
dep-ensure:
	$(DEPCMD) ensure

# Build the docker image
docker-build: test
	docker build $(PROJECTDIR) -f $(PROJECTDIR)/build/docker/Dockerfile -t $(DOCKERTAG)
	@echo "updating kustomize image patch file for manager resource"
	sed -i 's@image: .*@image: '"${DOCKERTAG}"'@' $(PROJECTDIR)/config/default/manager_image_patch.yaml

# Push the docker image
docker-push:
	docker push ${IMG}
