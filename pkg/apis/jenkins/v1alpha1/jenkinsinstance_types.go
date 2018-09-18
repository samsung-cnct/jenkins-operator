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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

type PluginSpec struct {
	// plugin Id
	Id string `json:"id,omitempty"`
	// plugin version string
	Version string `json:"version,omitempty"`
}

type ServiceSpec struct {
	// Jenkins service name
	Name string `json:"name,omitempty"`

	// Jenkins instance service type
	ServiceType corev1.ServiceType `json:"servicetype,omitempty"`

	// Jenkins service annotations
	Annotations map[string]string `json:"annotations,omitempty"`
}

type IngressSpec struct {

	// Jenkins ingress annotations
	Annotations map[string]string `json:"annotations,omitempty"`

	// Jenkins ingress tls secret
	TlsSecret string `json:"tlssecret,omitempty"`

	// Jenkins service name, if pre-existing
	Service string `json:"service,omitempty"`

	// Ingress backend path
	Path string `json:"path,omitempty"`
}

type StorageSpec struct {
	// Name of pre-existing (or not) PVC for jobs
	JobsPvc string `json:"jobspvc,omitempty"`

	// If PVC is to be created, what is its spec
	JobsPvcSpec *corev1.PersistentVolumeClaimSpec `json:"jobspvcspec,omitempty"`
}

type PluginConfigSpec struct {
	// config stored as multiline string
	Config string `json:"config,omitempty"`
	// config string loaded from a kubernetes secret
	ConfigSecret string `json:"configsecret,omitempty"`
}

// JenkinsInstanceSpec defines the desired state of JenkinsInstance
type JenkinsInstanceSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// What container image to use for a new jenkins instance
	// +kubebuilder:validation:Pattern=.+:.+
	Image string `json:"image,omitempty"`

	// Dictionary of environment variable values
	Env map[string]string `json:"env,omitempty"`

	// Array of plugin configurations
	Plugins []PluginSpec `json:"plugins,omitempty"`

	// Jenkins deployment annotations
	Annotations map[string]string `json:"annotations,omitempty"`

	// How many executors
	Executors int32 `json:"executors,omitempty"`

	AdminSecret string `json:"adminsecret,omitempty"`

	// Groovy configuration scripts
	PluginConfig *PluginConfigSpec `json:"pluginconfig,omitempty"`

	// Jenkins location
	Location string `json:"location,omitempty"`

	// Jenkins admin email
	AdminEmail string `json:"adminemail,omitempty"`

	// Jenkins service options
	Service *ServiceSpec `json:"service,omitempty"`

	// Jenkins ingress options
	Ingress *IngressSpec `json:"ingress,omitempty"`

	// Service account name for jenkins to run under
	ServiceAccount string `json:"serviceaccount,omitempty"`

	// Create a network policy
	NetworkPolicy bool `json:"networkpolicy,omitempty"`

	// Jenkins storage options
	Storage *StorageSpec `json:"storage,omitempty"`
}

// JenkinsInstanceStatus defines the observed state of JenkinsInstance
type JenkinsInstanceStatus struct {
	// Important: Run "make" to regenerate code after modifying this file

	// full url to newly created jenkins remote API endpoint
	Api string `json:"api,omitempty"`

	// api token
	SetupSecret string `json:"adminsecret,omitempty"`

	// state if jenkins server instance
	Phase string `json:"phase"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JenkinsInstance
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=jenkinsinstances
type JenkinsInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JenkinsInstanceSpec   `json:"spec,omitempty"`
	Status JenkinsInstanceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JenkinsInstanceList contains a list of JenkinsInstance
type JenkinsInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JenkinsInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JenkinsInstance{}, &JenkinsInstanceList{})
}
