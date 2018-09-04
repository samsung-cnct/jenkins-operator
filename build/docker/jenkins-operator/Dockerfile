# Build the manager binary
FROM golang:1.10.3 as builder

# Copy in the go src
WORKDIR /go/src/github.com/maratoid/jenkins-operator
COPY .  ./

# Build
ENV KUBEBUILDER_VERSION=1.0.3
ENV KUBEBUILDER_ARCH=amd64
ENV PATH="${PATH}:/usr/local/kubebuilder/bin"
RUN curl -L -O https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${KUBEBUILDER_VERSION}/kubebuilder_${KUBEBUILDER_VERSION}_linux_${KUBEBUILDER_ARCH}.tar.gz \
    && tar -zxvf kubebuilder_${KUBEBUILDER_VERSION}_linux_${KUBEBUILDER_ARCH}.tar.gz \
    && mv kubebuilder_${KUBEBUILDER_VERSION}_linux_${KUBEBUILDER_ARCH} /usr/local/kubebuilder
RUN make -f build/Makefile install-dep linux

# Copy the controller-manager into a thin image
FROM ubuntu:latest
WORKDIR /root/
COPY --from=builder /go/src/github.com/maratoid/jenkins-operator/jenkins-operator .
ENTRYPOINT ["./jenkins-operator", "--alsologtostderr", "--install-crds=true"]
