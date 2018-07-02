# Instructions to install API using the installer
# Build and test the controller-manager
FROM golang:1.10.1-alpine as builder
RUN apk update && apk add git curl

ENV TEST_ASSET_DIR /usr/local/bin
ENV TEST_ASSET_KUBECTL $TEST_ASSET_DIR/kubectl
ENV TEST_ASSET_KUBE_APISERVER $TEST_ASSET_DIR/kube-apiserver
ENV TEST_ASSET_ETCD $TEST_ASSET_DIR/etcd

# Download test framework binaries
ENV TEST_ASSET_URL https://storage.googleapis.com/k8s-c10s-test-binaries
RUN curl ${TEST_ASSET_URL}/etcd-Linux-x86_64 --output $TEST_ASSET_ETCD
RUN curl ${TEST_ASSET_URL}/kube-apiserver-Linux-x86_64 --output $TEST_ASSET_KUBE_APISERVER
RUN curl https://storage.googleapis.com/kubernetes-release/release/v1.9.2/bin/linux/amd64/kubectl --output $TEST_ASSET_KUBECTL
RUN chmod +x $TEST_ASSET_ETCD
RUN chmod +x $TEST_ASSET_KUBE_APISERVER
RUN chmod +x $TEST_ASSET_KUBECTL

# Copy in the go src
ADD . /go/src/github.com/maratoid/jenkins-operator
WORKDIR /go/src/github.com/maratoid/jenkins-operator

# Build and test the API code
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o controller-manager ./cmd/controller-manager/main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o ./jenkins-operator cmd/controller-manager/main.go
RUN go test ./pkg/... ./cmd/...


FROM alpine
RUN mkdir -p /root
COPY --from=builder /go/src/github.com/maratoid/jenkins-operator/jenkins-operator /root/controller-manager

ENTRYPOINT ["/root/controller-manager"]
CMD ["--install-crds=true"]
