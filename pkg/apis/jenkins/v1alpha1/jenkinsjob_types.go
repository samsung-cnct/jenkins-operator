/*
Copyright 2018 Samsung CNCT.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

const (
	JenkinsJobSecretUserPass = "usernamePassword"
	JenkinsJobSecretText     = "secretText"
	JenkinsJobSecretFile     = "secretFile"
	JenkinsJobSecretCert     = "certificate"
)

const (
	JenkinsJobSecretUsernameKey    = "username"
	JenkinsJobSecretPasswordKey    = "password"
	JenkinsJobSecretFileKey        = "data"
	JenkinsJobSecretTextKey        = "text"
	JenkinsJobSecretCertificateKey = "certificate"
)

// JenkinsSecret spec defines secret data to be used by JenkinsJob
type JenkinsCredentialSpec struct {
	// Id of credential to be created
	// +kubebuilder:validation:Pattern=^[a-z][a-z-]*[a-z]
	Credential string `json:"credential,omitempty"`
	// credential type
	// +kubebuilder:validation:Pattern=usernamePassword|secretText|serviceaccount|vaultgithub|vaultapprole|vaulttoken
	CredentialType string `json:"credentialtype,omitempty"`

	// name of secret that contains credential data
	Secret string `json:"secret,omitempty"`

	// map of credential fields to kubernetes secret data fields (from the Secret value above)
	// in the <credential field>:<kubernetes secret field>
	// format
	// ---
	// For secretFile:
	// filename - name of the file
	// data - file data
	// ie:
	// filename: name-of-file
	// data: data-of-file
	// ---
	// for usernamePassword:
	// username - user name
	// password - password
	// ie:
	// username: name-of-user
	// password: userpass
	// ---
	// for secretText:
	// text - secret text data
	// ie:
	// text: secrettext
	// ---
	// for certificate:
	// password - certificate password
	// certificate - PKCS#12 certificate data
	// ie:
	// password: certpw
	// certificate: pkcs-cert
	SecretData map[string]string `json:"secretdata,omitempty"`
}

// JenkinsJobSpec defines the desired state of JenkinsJob
type JenkinsJobSpec struct {
	// ID of the JenkinsServer instance to install this plugin in
	JenkinsInstance string `json:"jenkinsinstance,omitempty"`

	// Content of a jenkins job in form of Jenkins XML
	JobXml string `json:"jobxml,omitempty"`

	// Content of a jenkins job in form of Jenkins JobDSL
	JobDsl string `json:"jobdsl,omitempty"`

	// Jenkins secret data
	Credentials []JenkinsCredentialSpec `json:"credentials,omitempty"`
}

// JenkinsJobStatus defines the observed state of JenkinsJob
type JenkinsJobStatus struct {
	// state if jenkins server instance
	Phase string `json:"phase"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JenkinsJob
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=jenkinsjobs
type JenkinsJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JenkinsJobSpec   `json:"spec,omitempty"`
	Status JenkinsJobStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JenkinsJobList contains a list of JenkinsJob
type JenkinsJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JenkinsJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JenkinsJob{}, &JenkinsJobList{})
}
