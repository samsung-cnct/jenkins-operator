package util

import (
	"os"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	configlib "github.com/kubernetes-sigs/kubebuilder/pkg/config"
	"net/url"
	"net"
	"errors"
)

func AmRunningInCluster() bool {
	_, kubeServiceHostPresent := os.LookupEnv("KUBERNETES_SERVICE_HOST")
	_, kubeServicePortPresent := os.LookupEnv("KUBERNETES_SERVICE_PORT")

	return kubeServiceHostPresent && kubeServicePortPresent
}

func GetServiceEndpoint(service *corev1.Service, path string, internalPort int32) (string, error) {
	var endpoint string
	if AmRunningInCluster() {
		endpoint = fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d",
			service.Name, service.Namespace, internalPort)
	} else {
		restConfig, err := configlib.GetConfig()
		if err != nil {
			return "", err
		}
		hostUrl, _ := url.Parse(restConfig.Host)
		host, _, _ := net.SplitHostPort(hostUrl.Host)

		var nodePort int32 = 0
		for _, port := range service.Spec.Ports {
			if port.Port == internalPort {
				nodePort = port.NodePort
				break;
			}
		}

		if nodePort == 0 {
			return "", errors.New("could not find corresponding node port")
		}

		endpoint = fmt.Sprintf("http://%s:%d/%s",host, nodePort, path)
	}

	return endpoint, nil
}