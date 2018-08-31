package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

const (
	mockApiTokenResponse   = `<html><body><input readonly="readonly" id="apiToken" type="text" value="THISISADUMMYJENKINSAPITOKEN" /></body></html>`
	mockCrumbTokenResponse = "Jenkins-Crumb:THISISADUMMYJENKINSCRUMBTOKEN"
)

const (
	getApiTokenPath      = "/me/configure"
	getCrumbTokenPath    = "/crumbIssuer/api/xml"
	createXmlItemPath    = "/createItem"
	createDslItemPath    = "/job/create-jenkins-jobs/build"
	jobExistsPath        = "/job/test-job/api/json"
	jobDeletePath        = "/job/test-job/doDelete"
	credentialExistsPath = "/credentials/store/system/domain/_/credential/test-creds/api/json"
	createCredentialPath = "/credentials/store/system/domain/_/createCredentials"
	credentialDeletePath = "/credentials/store/system/domain/_/credential/test-creds/doDelete"
	restartPath          = "/safeRestart"
)

var (
	// mux is the HTTP request multiplexer used with the test server.
	mux *http.ServeMux

	// server is a test HTTP server used to provide mock API responses.
	server *httptest.Server
)

func Setup() {
	mux = http.NewServeMux()
	mux.HandleFunc(getApiTokenPath, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, mockApiTokenResponse)
	})
	mux.HandleFunc(getCrumbTokenPath, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, mockCrumbTokenResponse)
	})
	mux.HandleFunc(createXmlItemPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(createDslItemPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(jobExistsPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(jobDeletePath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(credentialExistsPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc(createCredentialPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(credentialDeletePath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(restartPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server = httptest.NewServer(mux)
}

func GetURL() string {
	return server.URL
}

func Teardown() {
	server.Close()
}
