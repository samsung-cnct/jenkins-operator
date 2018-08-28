package util

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/golang/glog"
	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	"github.com/sethgrid/pester"
	"io/ioutil"
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

func GetJenkinsLocationHost(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) string {
	hostUrl, _ := url.Parse(jenkinsInstance.Spec.Location)
	return hostUrl.Host
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

	apiToken := ""
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

func GetCSRFToken(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, setupSecret *corev1.Secret) (string, string, error) {
	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		return "", "", err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	apiUrl.Path = "crumbIssuer/api/xml"
	apiUrl.RawQuery = "xpath=concat(//crumbRequestField,\":\",//crumb)"

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

	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	csrfToken := strings.Split(string(bodyBytes), ":")

	return csrfToken[0], csrfToken[1], nil
}

// CreateJenkinsJob creates a jenkins job from an xml string
func CreateJenkinsXMLJob(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, setupSecret *corev1.Secret, jobName string, jobXML string) error {
	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	apiUrl.Path = "createItem"
	apiUrl.RawQuery = fmt.Sprintf("name=%s", jobName)

	// if job already exists, change api url to update
	exists, err := JenkinsJobExists(jenkinsInstance, setupSecret, jobName)
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
	csrfTokenKey, csrfTokenVal, err := GetCSRFToken(jenkinsInstance, setupSecret)
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
func CreateJenkinsDSLJob(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, setupSecret *corev1.Secret, jobDSL string) error {
	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	apiUrl.Path = "job/create-jenkins-jobs/build"

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
	csrfTokenKey, csrfTokenVal, err := GetCSRFToken(jenkinsInstance, setupSecret)
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
func JenkinsJobExists(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, setupSecret *corev1.Secret, jobName string) (bool, error) {
	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		return false, err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	apiUrl.Path = fmt.Sprint("job/", jobName, "/api/json")

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
	csrfTokenKey, csrfTokenVal, err := GetCSRFToken(jenkinsInstance, setupSecret)
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
func DeleteJenkinsJob(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, setupSecret *corev1.Secret, jobName string) error {
	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	apiUrl.Path = fmt.Sprint("job/", jobName, "/doDelete")

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
	csrfTokenKey, csrfTokenVal, err := GetCSRFToken(jenkinsInstance, setupSecret)
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

// JenkinsJobExists checks whether a job with a particular name already exists in Jenkins
func JenkinsCredentialExists(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, setupSecret *corev1.Secret, credentialId string) (bool, error) {
	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		return false, err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	apiUrl.Path = fmt.Sprint("credentials/store/system/domain/_/credential/", credentialId, "/api/json")

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
	csrfTokenKey, csrfTokenVal, err := GetCSRFToken(jenkinsInstance, setupSecret)
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

	return resp.StatusCode == 302, nil
}

// CreateJenkinsCredential adds a credential to jenkins
func CreateJenkinsCredential(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, setupSecret *corev1.Secret, credentialSecret *corev1.Secret, jobName string, credentialId string, credentialType string, credentialMap map[string]string) error {
	err := DeleteJenkinsCredential(jenkinsInstance, setupSecret, credentialId)
	if err != nil {
		return err
	}

	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	apiUrl.Path = "/credentials/store/system/domain/_/createCredentials"

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
	csrfTokenKey, csrfTokenVal, err := GetCSRFToken(jenkinsInstance, setupSecret)
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
func DeleteJenkinsCredential(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, setupSecret *corev1.Secret, id string) error {
	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	apiUrl.Path = fmt.Sprint("/credentials/store/system/domain/_/credential/", id, "/doDelete")

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
	csrfTokenKey, csrfTokenVal, err := GetCSRFToken(jenkinsInstance, setupSecret)
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
func SafeRestartJenkins(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, setupSecret *corev1.Secret) error {
	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		return err
	}
	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	apiUrl.Path = "/safeRestart"

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
	csrfTokenKey, csrfTokenVal, err := GetCSRFToken(jenkinsInstance, setupSecret)
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
