GOCMD=go
KUBEBUILDERCMD=kubebuilder
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
OPERATOR_SOURCE_FILE=cmd/controller-manager/main.go

# build binary command
BUILD_OPERATOR_BINARY_CMD=${GOBUILD} -a -tags netgo -ldflags ${LDFLAGS} -o ${OPERATOR_BINARY_NAME} ${OPERATOR_SOURCE_FILE}

# By default we will most likely want to build a linux build of the application
all: bin-data kube-builder jenkins-operator-linux-amd64
linux: bin-data kube-builder jenkins-operator-linux-amd64
darwin: bin-data kube-builder jenkins-operator-darwin-amd64
container: bin-data kube-builder jenkins-operator-docker

# Clean will clean up dep and remove any binaries
clean: remove-binary

# generate bindata
bin-data:
	go-bindata -o $(PROJECTDIR)/pkg/bindata/data.go -ignore data\\.go -pkg bindata -prefix $(PROJECTDIR)/pkg/bindata $(PROJECTDIR)/pkg/bindata/...

# Generate kube builder code
kube-builder:
	$(KUBEBUILDERCMD) generate

# Install dep will install dep in $GOROOT/bin
install-dep:
	go get -u github.com/golang/dep/cmd/dep
	go get -u github.com/go-bindata/go-bindata/...

# New Dep will just blow away Gopkg, vendor and do a dep init
new-dep:
	rm -rf vendor Gopkg.toml Gopkg.lock
	$(DEPCMD) init

# For builds where we're just going to go along with whatever Gopkg is saying
dep-ensure:
	$(DEPCMD) ensure

# Build a darwin (mac) binary
jenkins-operator-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 ${BUILD_OPERATOR_BINARY_CMD}

# Build a linux amd64 binary
jenkins-operator-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 ${BUILD_OPERATOR_BINARY_CMD}

# Build a docker image
jenkins-operator-docker:
	docker build $(PROJECTDIR) -f $(PROJECTDIR)/build/docker/Dockerfile -t $(DOCKERTAG)

# Delete an existing binary / cleanup
remove-binary:
	rm ${OPERATOR_BINARY_NAME}
