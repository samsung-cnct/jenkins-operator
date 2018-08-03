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

	// Jenkins master port
	MasterPort int32 `json:"masterport,omitempty"`

	// Jenkins agent port
	AgentPort int32 `json:"agentport,omitempty"`

	// How many executors
	Executors int32 `json:"executors,omitempty"`

	AdminSecret string `json:"adminsecret,omitempty"`

	// Groovy configuration scripts
	Config string `json:"config,omitempty"`

	// Number of replicas
	Replicas *int32 `json:"replicas,omitempty"`

	// Image pull policy
	PullPolicy corev1.PullPolicy `json:"pullpolicy,omitempty"`

	// Jenkins instance service type
	ServiceType corev1.ServiceType `json:"servicetype,omitempty"`

	// Jenkins location
	Location string `json:"location,omitempty"`

	// Jenkins admin email
	AdminEmail string `json:"adminemail,omitempty"`

	// Name of pre-existing PVC for jobs
	JobsPvc string `json:"jobspvc,omitempty"`
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
