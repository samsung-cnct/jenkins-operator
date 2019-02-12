package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

const (
	mockCrumbTokenResponse = `{"_class":"hudson.security.csrf.DefaultCrumbIssuer","crumb":"THISISADUMMYJENKINSCRUMBTOKEN","crumbRequestField":"Jenkins-Crumb"}`
)

const (
	getCrumbTokenPath = "/crumbIssuer/api/json"
	restartPath       = "/safeRestart"
	loginPath         = "/login"
)

var (
	// mux is the HTTP request multiplexer used with the test server.
	mux *http.ServeMux

	// server is a test HTTP server used to provide mock API responses.
	server *httptest.Server
)

func Setup() {
	mux = http.NewServeMux()
	mux.HandleFunc(getCrumbTokenPath, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, mockCrumbTokenResponse)
	})
	mux.HandleFunc(restartPath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc(loginPath, func(w http.ResponseWriter, r *http.Request) {
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
