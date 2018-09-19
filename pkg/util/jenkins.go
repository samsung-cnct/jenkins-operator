package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/golang/glog"
	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	"github.com/sethgrid/pester"
	corev1 "k8s.io/api/core/v1"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

type credentialProperties struct {
	Class      string
	Properties []string
}

var credentialPropertyMap = map[string]credentialProperties{
	"secretText": {
		Class:      "org.jenkinsci.plugins.plaincredentials.impl.StringCredentialsImpl",
		Properties: []string{"secret"},
	},
	"usernamePassword": {
		Class:      "com.cloudbees.plugins.credentials.impl.UsernamePasswordCredentialsImpl",
		Properties: []string{"username", "password"},
	},
	"serviceaccount": {
		Class:      "org.jenkinsci.plugins.kubernetes.credentials.FileSystemServiceAccountCredential",
		Properties: []string{},
	},
	"vaultgithub": {
		Class:      "com.datapipe.jenkins.vault.credentials.VaultGithubTokenCredential",
		Properties: []string{"accessToken"},
	},
	"vaultapprole": {
		Class:      "com.datapipe.jenkins.vault.credentials.VaultAppRoleCredential",
		Properties: []string{"roleId", "secretId"},
	},
	"vaulttoken": {
		Class:      "com.datapipe.jenkins.vault.credentials.VaultTokenCredential",
		Properties: []string{"token"},
	},
}

type jenkinsApiTokenData struct {
	TokenName  string `json:"tokenName"`
	TokenUuid  string `json:"tokenUuid"`
	TokenValue string `json:"tokenValue"`
}

type jenkinsApiToken struct {
	Status string              `json:"status"`
	Data   jenkinsApiTokenData `json:"data"`
}

type jenkinsCrumbToken struct {
	Class             string `json:"_class"`
	Crumb             string `json:"crumb"`
	CrumbRequestField string `json:"crumbRequestField"`
}

// GetJenkinsLocationHost returns jenkins location url
func GetJenkinsLocationHost(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) string {
	hostUrl, _ := url.Parse(jenkinsInstance.Spec.Location)
	return hostUrl.Host
}

// /me/descriptorByName/jenkins.security.ApiTokenProperty/revoke + tokenUuid
// RevokeJenkinsApiToken revokes an api token by uuid
func RevokeJenkinsApiToken(service *corev1.Service, adminSecret *corev1.Secret, setupSecret *corev1.Secret, masterPort int32) error {
	serviceUrl, err := GetServiceEndpoint(service, "me/descriptorByName/jenkins.security.ApiTokenProperty/revoke", masterPort)
	if err != nil {
		return err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(adminSecret.Data["user"][:]), string(adminSecret.Data["pass"][:]))

	tokenUuid := string(setupSecret.Data["tokenUuid"][:])
	apiUrl.RawQuery = fmt.Sprintf("tokenUuid=%s", tokenUuid)

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
	csrfUrl, err := GetServiceEndpoint(service, "", masterPort)
	if err != nil {
		return err
	}
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(
		csrfUrl,
		string(adminSecret.Data["user"][:]),
		string(adminSecret.Data["pass"][:]))
	if err != nil {
		glog.Errorf("Error getting CSRF token")
		return err
	}

	req.Header.Add(csrfTokenKey, csrfTokenVal)
	req.Header.Add("Content-Type", "text/xml; charset=utf-8")

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

// GetJenkinsApiToken gets an api token for the admin user from a newly created jenkins deployment
func GetJenkinsApiToken(service *corev1.Service, adminSecret *corev1.Secret, masterPort int32) (string, string, error) {

	serviceUrl, err := GetServiceEndpoint(service, "me/descriptorByName/jenkins.security.ApiTokenProperty/generateNewToken", masterPort)
	if err != nil {
		return "", "", err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return "", "", err
	}
	apiUrl.User = url.UserPassword(string(adminSecret.Data["user"][:]), string(adminSecret.Data["pass"][:]))
	apiUrl.RawQuery = fmt.Sprintf("newTokenName=%s", "jenkins-operator-token")

	// create request
	req, err := http.NewRequest(
		"POST",
		apiUrl.String(),
		nil)
	if err != nil {
		glog.Errorf("Error creating POST request to %s", apiUrl)
		return "", "", err
	}

	// add headers
	csrfUrl, err := GetServiceEndpoint(service, "", masterPort)
	if err != nil {
		return "", "", err
	}
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(
		csrfUrl,
		string(adminSecret.Data["user"][:]),
		string(adminSecret.Data["pass"][:]))
	if err != nil {
		glog.Errorf("Error getting CSRF token")
		return "", "", err
	}

	req.Header.Add(csrfTokenKey, csrfTokenVal)
	req.Header.Add("Content-Type", "text/xml; charset=utf-8")

	client := pester.New()
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = true

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error performing POST request to %s: %v", apiUrl, err)
		return "", "", err
	}
	defer resp.Body.Close()

	// {"status":"ok","data":{"tokenName":"TOKEN-NAME","tokenUuid":"TOKEN-UUID","tokenValue":"TOKEN-VALUE"}}
	apiToken := jenkinsApiToken{}
	err = json.NewDecoder(resp.Body).Decode(&apiToken)
	if err != nil {
		glog.Errorf("Error decoding POST response: %v", err)
		return "", "", err
	}

	return apiToken.Data.TokenValue, apiToken.Data.TokenUuid, nil
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

// CreateJenkinsJob creates a jenkins job from an xml string
func CreateJenkinsXMLJob(service *corev1.Service, setupSecret *corev1.Secret, jobName string, jobXML string, masterPort int32) error {
	serviceUrl, err := GetServiceEndpoint(service, "createItem", masterPort)
	if err != nil {
		return err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	apiUrl.RawQuery = fmt.Sprintf("name=%s", jobName)

	// if job already exists, change api url to update
	exists, err := JenkinsJobExists(service, setupSecret, jobName, masterPort)
	if err != nil {
		glog.Errorf("Error checking whether jenkins job item %s exists", jobName)
		return err
	}

	if exists {
		apiUrl.Path = fmt.Sprint("job/", jobName, "/config.xml")
		apiUrl.RawQuery = ""
	}

	// create request
	req, err := http.NewRequest(
		"POST",
		apiUrl.String(),
		strings.NewReader(jobXML))
	if err != nil {
		glog.Errorf("Error creating POST request to %s", apiUrl)
		return err
	}

	// add headers
	serviceUrl, err = GetServiceEndpoint(service, "", masterPort)
	if err != nil {
		return err
	}
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(serviceUrl, string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	if err != nil {
		glog.Errorf("Error getting CSRF token")
		return err
	}

	req.Header.Add(csrfTokenKey, csrfTokenVal)
	req.Header.Add("Content-Type", "text/xml; charset=utf-8")

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

// CreateJenkinsJob creates a jenkins job from an jobdsl string
func CreateJenkinsDSLJob(service *corev1.Service, setupSecret *corev1.Secret, jobDSL string, masterPort int32) error {
	serviceUrl, err := GetServiceEndpoint(service, "job/create-jenkins-jobs/build", masterPort)
	if err != nil {
		return err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))

	requestBody := new(bytes.Buffer)
	writer := multipart.NewWriter(requestBody)

	// write the dsl file part
	part, err := writer.CreateFormFile("file0", "file0")
	if err != nil {
		glog.Errorf("Error creating post form: %s", err)
		return err
	}
	part.Write([]byte(jobDSL))

	type DslJson struct {
		Parameter []map[string]string `json:"parameter"`
	}

	dslJson := &DslJson{
		Parameter: []map[string]string{
			{
				"name": "job.dsl",
				"file": "file0",
			},
		},
	}

	jsonHeader, err := json.Marshal(dslJson)
	if err != nil {
		glog.Errorf("error creating jobDsl json for %s", jobDSL)
		return err
	}

	// write the json part
	err = writer.WriteField("json", string(jsonHeader))
	if err != nil {
		glog.Errorf("Error creating post form: %s", err)
		return err
	}

	// close writer to establish request boundary
	err = writer.Close()
	if err != nil {
		glog.Errorf("Error creating post form: %s", err)
		return err
	}

	// create request
	req, err := http.NewRequest(
		"POST",
		apiUrl.String(),
		requestBody)
	if err != nil {
		glog.Errorf("Error creating POST request to %s", apiUrl)
		return err
	}

	// add headers
	serviceUrl, err = GetServiceEndpoint(service, "", masterPort)
	if err != nil {
		return err
	}
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(serviceUrl, string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	if err != nil {
		glog.Errorf("Error getting CSRF token")
		return err
	}

	req.Header.Add(csrfTokenKey, csrfTokenVal)
	req.Header.Add("Content-Type", writer.FormDataContentType())

	client := pester.New()
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = true

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error performing POST request to %s: %s, client log: %s", apiUrl, err, client.LogString())
		return err
	}
	defer resp.Body.Close()

	return nil
}

// JenkinsJobExists checks whether a job with a particular name already exists in Jenkins
func JenkinsJobExists(service *corev1.Service, setupSecret *corev1.Secret, jobName string, masterPort int32) (bool, error) {
	serviceUrl, err := GetServiceEndpoint(service, fmt.Sprint("job/", jobName, "/api/json"), masterPort)
	if err != nil {
		return false, err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return false, err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))

	// create request
	req, err := http.NewRequest(
		"GET",
		apiUrl.String(),
		nil)
	if err != nil {
		glog.Errorf("Error creating GET request to %s", apiUrl)
		return false, err
	}

	// add headers
	serviceUrl, err = GetServiceEndpoint(service, "", masterPort)
	if err != nil {
		return false, err
	}
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(serviceUrl, string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	if err != nil {
		glog.Errorf("Error getting CSRF token")
		return false, err
	}

	req.Header.Add(csrfTokenKey, csrfTokenVal)
	req.Header.Add("Accept", `*/*`)
	req.Header.Add("User-Agent", `jenkins-operator`)

	client := pester.New()
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = true

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error performing GET request to %s: %v", apiUrl, err)
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

// DeleteJenkinsJob deletes a jenkins job
func DeleteJenkinsJob(service *corev1.Service, setupSecret *corev1.Secret, jobName string, masterPort int32) error {
	serviceUrl, err := GetServiceEndpoint(service, fmt.Sprint("job/", jobName, "/doDelete"), masterPort)
	if err != nil {
		return err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))

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
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(serviceUrl, string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
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

func getJenkinsCredentialJson(credentialSecret *corev1.Secret, jobName string, credentialId string, credentialType string, credentialMap map[string]string) string {

	type CredentialJson struct {
		EmptyStr    int32             `json:""`
		Credentials map[string]string `json:"credentials"`
	}

	credentialJson := &CredentialJson{
		EmptyStr: 0,
		Credentials: map[string]string{
			"scope":       "GLOBAL",
			"id":          credentialId,
			"description": fmt.Sprint("Credentials from ", jobName),
			"$class":      credentialPropertyMap[credentialType].Class,
		},
	}

	// add on properties
	for _, secretFieldKey := range credentialPropertyMap[credentialType].Properties {
		credentialJson.Credentials[secretFieldKey] = string(credentialSecret.Data[credentialMap[secretFieldKey]][:])
	}

	credentialString, err := json.Marshal(credentialJson)
	if err != nil {
		glog.Errorf("error creating credential %s json for JenkinsJob %s", credentialId, jobName)
	}

	return string(credentialString)
}

// JenkinsApiTokenValid checks whether an api token is valid
func JenkinsApiTokenValid(service *corev1.Service, adminSecret *corev1.Secret, setupSecret *corev1.Secret, masterPort int32) (bool, error) {

	serviceUrl, err := GetServiceEndpoint(service, "me/api/json", masterPort)
	if err != nil {
		return false, err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return false, err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))

	// create request
	req, err := http.NewRequest(
		"GET",
		apiUrl.String(),
		nil)
	if err != nil {
		glog.Errorf("Error creating GET request to %s", apiUrl)
		return false, err
	}

	// add headers
	csrfUrl, err := GetServiceEndpoint(service, "", masterPort)
	if err != nil {
		return false, err
	}
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(csrfUrl, string(adminSecret.Data["user"][:]), string(adminSecret.Data["pass"][:]))
	if err != nil {
		glog.Errorf("Error getting CSRF token")
		return false, err
	}

	req.Header.Add(csrfTokenKey, csrfTokenVal)
	req.Header.Add("Accept", `*/*`)
	req.Header.Add("User-Agent", `jenkins-operator`)

	client := pester.New()
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = true

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error performing GET request to %s: %v", apiUrl, err)
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == 200, nil
}

// CreateJenkinsCredential adds a credential to jenkins
func CreateJenkinsCredential(service *corev1.Service, setupSecret *corev1.Secret, credentialSecret *corev1.Secret, jobName string, credentialId string, credentialType string, credentialMap map[string]string, masterPort int32) error {
	serviceUrl, err := GetServiceEndpoint(service, "/credentials/store/system/domain/_/createCredentials", masterPort)
	if err != nil {
		return err
	}

	err = DeleteJenkinsCredential(service, setupSecret, credentialId, masterPort)
	if err != nil {
		return err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	credentialJson := getJenkinsCredentialJson(credentialSecret, jobName, credentialId, credentialType, credentialMap)

	var requestBody = url.Values{}
	requestBody.Add("json", credentialJson)

	// create request
	req, err := http.NewRequest(
		"POST",
		apiUrl.String(),
		strings.NewReader(requestBody.Encode()))
	if err != nil {
		glog.Errorf("Error creating POST request to %s", apiUrl)
		return err
	}

	// add headers
	serviceUrl, err = GetServiceEndpoint(service, "", masterPort)
	if err != nil {
		return err
	}
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(serviceUrl, string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	if err != nil {
		glog.Errorf("Error getting CSRF token")
		return err
	}
	req.Header.Add(csrfTokenKey, csrfTokenVal)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := pester.New()
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = true

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error performing POST request to %s: %s client log: %s", apiUrl, err, client.LogString())
		return err
	}
	defer resp.Body.Close()

	return nil
}

// DeleteJenkinsCredential deletes a credential from jenkins
func DeleteJenkinsCredential(service *corev1.Service, setupSecret *corev1.Secret, id string, masterPort int32) error {
	serviceUrl, err := GetServiceEndpoint(service, fmt.Sprint("/credentials/store/system/domain/_/credential/", id, "/doDelete"), masterPort)
	if err != nil {
		return err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))

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
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(serviceUrl, string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
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

// SafeRestartJenkins does a safe jenkins restart
func SafeRestartJenkins(service *corev1.Service, setupSecret *corev1.Secret, masterPort int32) error {
	serviceUrl, err := GetServiceEndpoint(service, "/safeRestart", masterPort)
	if err != nil {
		return err
	}

	apiUrl, err := url.Parse(serviceUrl)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))

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
	csrfTokenKey, csrfTokenVal, err := getCSRFToken(serviceUrl, string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
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
