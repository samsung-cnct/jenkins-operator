package util

import (
	"errors"
	"fmt"
	"github.com/maratoid/jenkins-operator/pkg/test"
	corev1 "k8s.io/api/core/v1"
	"net"
	"net/url"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

// MergeSecretData merges secret Data maps
func MergeSecretData(ms ...map[string][]byte) map[string][]byte {
	res := map[string][]byte{}
	for _, m := range ms {
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}

// InArray searches for arbitrary object types in an array
func InArray(val interface{}, array interface{}) (exists bool, index int) {
	exists = false
	index = -1

	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) == true {
				index = i
				exists = true
				return
			}
		}
	}

	return
}

// GetNodePort retrieves node port number of a specified kubernetes service
func GetNodePort(servicePorts []corev1.ServicePort, portName string) int32 {
	for _, port := range servicePorts {
		if port.Name == portName {
			return port.NodePort
		}
	}

	return 0
}

// AddFinalizer adds a finalizer string to an array if not present
func AddFinalizer(finalizer string, finalizers []string) []string {
	if exists, _ := InArray(finalizer, finalizers); exists {
		return finalizers
	}

	return append(finalizers, finalizer)
}

// DeleteFinalizer removes a finalizer string from an array if present
func DeleteFinalizer(finalizer string, finalizers []string) []string {
	// only delete if at the top of the list
	if exists, index := InArray(finalizer, finalizers); exists && index == 0 {
		return append(finalizers[:index], finalizers[index+1:]...)
	}

	return finalizers
}

// AmRunningInCluster returns true if this binary is running in kubernetes cluster
func AmRunningInCluster() bool {
	_, kubeServiceHostPresent := os.LookupEnv("KUBERNETES_SERVICE_HOST")
	_, kubeServicePortPresent := os.LookupEnv("KUBERNETES_SERVICE_PORT")

	return kubeServiceHostPresent && kubeServicePortPresent
}

// AmRunningInTest returns true if this binary is running under ginkgo
func AmRunningInTest() bool {
	_, runningUnderLocalTest := os.LookupEnv("JENKINS_OPERATOR_TESTRUN")
	return runningUnderLocalTest
}

// gets the correct endpoint for a given service based on whether code is running in cluster or not
func GetServiceEndpoint(service *corev1.Service, path string, internalPort int32) (string, error) {
	var endpoint string
	if AmRunningInCluster() {
		endpoint = fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d/%s",
			service.Name, service.Namespace, internalPort, path)
	} else if AmRunningInTest() {
		endpoint = fmt.Sprint(test.GetURL(), "/", path)
	} else {
		restConfig, err := config.GetConfig()
		if err != nil {
			return "", err
		}
		hostUrl, _ := url.Parse(restConfig.Host)
		host, _, _ := net.SplitHostPort(hostUrl.Host)

		var nodePort int32 = 0
		for _, port := range service.Spec.Ports {
			if port.Port == internalPort {
				nodePort = port.NodePort
				break
			}
		}

		if nodePort == 0 {
			return "", errors.New("could not find corresponding node port, service type must be 'NodePort' " +
				"when running controller out of cluster")
		}

		endpoint = fmt.Sprintf("http://%s:%d/%s", host, nodePort, path)
	}

	return endpoint, nil
}
