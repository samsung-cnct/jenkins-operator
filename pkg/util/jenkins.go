package util

import (
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	"github.com/sethgrid/pester"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"net/url"
)

type jenkinsCrumbToken struct {
	Class             string `json:"_class"`
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

// getCSRFToken returns two components of a crumb protection header
func getCSRFToken(jenkinsApiUrl string, userName string, password string) (string, string, error) {
	apiUrl, err := url.Parse(jenkinsApiUrl)
	if err != nil {
		return "", "", err
	}
	apiUrl.User = url.UserPassword(userName, password)
	apiUrl.Path = "crumbIssuer/api/json"

	req, err := http.NewRequest("GET", apiUrl.String(), nil)
	if err != nil {
		glog.Errorf("Error creating GET request to %s", apiUrl)
		return "", "", err
	}

	client := pester.New()
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = true

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error performing GET request to %s: %v", apiUrl, err)
		return "", "", err
	}
	defer resp.Body.Close()

	// {"_class":"hudson.security.csrf.DefaultCrumbIssuer","crumb":"CRUMB-TOKEN","crumbRequestField":"CRUMB-FIELD"}
	crumbToken := jenkinsCrumbToken{}
	err = json.NewDecoder(resp.Body).Decode(&crumbToken)
	if err != nil {
		glog.Errorf("Error decoding GET response: %v", err)
		return "", "", err
	}

	return crumbToken.CrumbRequestField, crumbToken.Crumb, nil
}

// SafeRestartJenkins does a safe jenkins restart
func SafeRestartJenkins(service *corev1.Service, adminSecret *corev1.Secret, masterPort int32) error {
	serviceUrl, err := GetServiceEndpoint(service, "/safeRestart", masterPort)
	if err != nil {
		return err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(adminSecret.Data["JENKINS_ADMIN_USER"][:]), string(adminSecret.Data["JENKINS_ADMIN_PASSWORD"][:]))

	// create request
	req, err := http.NewRequest(
		"POST",
		apiUrl.String(),
		nil)
	if err != nil {
		glog.Errorf("Error creating POST request to %s", apiUrl)
		return err
	}

	// add headers
	serviceUrl, err = GetServiceEndpoint(service, "", masterPort)
	if err != nil {
		return err
	}
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(serviceUrl, string(adminSecret.Data["JENKINS_ADMIN_USER"][:]), string(adminSecret.Data["JENKINS_ADMIN_PASSWORD"][:]))
	if err != nil {
		glog.Errorf("Error getting CSRF token")
		return err
	}
	req.Header.Add(csrfTokenKey, csrfTokenVal)

	client := pester.New()
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = true

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error performing POST request to %s: %v", apiUrl, err)
		return err
	}
	defer resp.Body.Close()

	return nil
}

// check if jenkins is ready
func CheckJenkinsReady(service *corev1.Service, masterPort int32) error {
	serviceUrl, err := GetServiceEndpoint(service, "/login", masterPort)
	if err != nil {
		return err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return err
	}

	// create request
	req, err := http.NewRequest(
		"GET",
		apiUrl.String(),
		nil)
	if err != nil {
		glog.Errorf("Error creating GET request to %s", apiUrl)
		return err
	}

	client := pester.New()
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = true

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error performing GET request to %s: %v", apiUrl, err)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	} else {
		return fmt.Errorf("jenkins not ready, repsponse code %d", resp.StatusCode)
	}

}
