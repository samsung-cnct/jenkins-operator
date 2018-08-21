package util

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/golang/glog"
	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	"github.com/sethgrid/pester"
	corev1 "k8s.io/api/core/v1"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

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

func AddFinalizer(finalizer string, finalizers []string) []string {
	if exists, _ := InArray(finalizer, finalizers); exists {
		return finalizers
	}

	return append(finalizers, finalizer)
}

func DeleteFinalizer(finalizer string, finalizers []string) []string {
	if exists, index := InArray(finalizer, finalizers); exists {
		return append(finalizers[:index], finalizers[index+1:]...)
	}

	return finalizers
}

func AmRunningInCluster() bool {
	_, kubeServiceHostPresent := os.LookupEnv("KUBERNETES_SERVICE_HOST")
	_, kubeServicePortPresent := os.LookupEnv("KUBERNETES_SERVICE_PORT")

	return kubeServiceHostPresent && kubeServicePortPresent
}

func GetJenkinsLocationHost(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) string {
	hostUrl, _ := url.Parse(jenkinsInstance.Spec.Location)
	return hostUrl.Host
}

// gets the correct endpoint for a given service based on whether code is running in cluster or not
func GetServiceEndpoint(service *corev1.Service, path string, internalPort int32) (string, error) {
	var endpoint string
	if AmRunningInCluster() {
		endpoint = fmt.Sprintf(
			"http://%s.%s.svc.cluster.local:%d",
			service.Name, service.Namespace, internalPort)
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
			return "", errors.New("could not find corresponding node port")
		}

		endpoint = fmt.Sprintf("http://%s:%d/%s", host, nodePort, path)
	}

	return endpoint, nil
}

// GetJenkinsApiToken gets an api token for the admin user from a newly created jenkins deployment
func GetJenkinsApiToken(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, service *corev1.Service, adminSecret *corev1.Secret, masterPort int32) (string, error) {

	serviceUrl, err := GetServiceEndpoint(service, "me/configure", masterPort)
	if err != nil {
		return "", err
	}

	adminUser := string(adminSecret.Data["user"][:])
	adminPassword := string(adminSecret.Data["pass"][:])

	// get the user config page
	req, err := http.NewRequest("GET", serviceUrl, nil)
	if err != nil {
		glog.Errorf("Error creating GET request to %s", serviceUrl)
		return "", err
	}

	req.SetBasicAuth(string(adminUser[:]), string(adminPassword[:]))
	if err != nil {
		glog.Errorf("Error setting basic auth on GET request to %s", serviceUrl)
		return "", err
	}

	client := pester.New()
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = true

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error performing GET request to %s: %v", serviceUrl, err)
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		glog.Errorf("Error parsing response: %v", err)
		return "", err
	}

	var apiToken string = ""
	doc.Find("input#apiToken").Each(func(i int, element *goquery.Selection) {
		value, exists := element.Attr("value")
		if exists {
			apiToken = value
		}
	})

	if apiToken == "" {
		err = fmt.Errorf("element 'apiToken' missing value")
	} else {
		err = nil
	}

	return apiToken, err
}
