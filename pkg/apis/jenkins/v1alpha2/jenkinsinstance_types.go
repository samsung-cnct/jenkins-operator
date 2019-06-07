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

package v1alpha2

import (
	"encoding/json"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CascConfigSpec struct {
	// casc config as multi-line yaml string
	ConfigString string `json:"configstring,omitempty"`

	// or casc config(s) mounted from a named config map
	ConfigMap string `json:"configmap,omitempty"`
}

type ServiceSpec struct {
	// Jenkins service name
	Name string `json:"name,omitempty"`

	// Jenkins instance service type
	ServiceType corev1.ServiceType `json:"servicetype,omitempty"`

	// If type is node port, use this node port value
	NodePort int32 `json:"nodeport,omitempty"`

	// Jenkins service annotations
	Annotations map[string]string `json:"annotations,omitempty"`
}

type StorageSpec struct {
	// Name of pre-existing (or not) PVC for jobs
	JobsPvc string `json:"jobspvc,omitempty"`

	// If PVC is to be created, what is its spec
	JobsPvcSpec *corev1.PersistentVolumeClaimSpec `json:"jobspvcspec,omitempty"`
}

type PluginSpec struct {
	// plugin id
	Id string `json:"id"`

	// plugin version string, follows the format at https://github.com/jenkinsci/docker#plugin-version-format
	Version json.Number `json:"version"`
}

// JenkinsInstanceSpec defines the desired state of JenkinsInstance
type JenkinsInstanceSpec struct {
	// Important: Run "make" to regenerate code after modifying this file

	// What container image to use for a new jenkins instance
	// +kubebuilder:validation:Pattern=.+:.+
	Image string `json:"image,omitempty"`

	// image pull policy
	ImagePullPolicy corev1.PullPolicy `json:"imagepullpolicy,omitempty"`

	// Dictionary of environment variable values
	Env map[string]string `json:"env,omitempty"`

	// Array of plugins to be installed
	Plugins []PluginSpec `json:"plugins,omitempty"`

	// configuration-as-code spec
	CascConfig *CascConfigSpec `json:"cascconfig,omitempty"`

	// configuration-as-code secret name
	CascSecret string `json:"cascsecret,omitempty"`

	// groovy configuration secret name
	GroovySecret string `json:"groovysecret,omitempty"`

	// Jenkins deployment annotations
	Annotations map[string]string `json:"annotations,omitempty"`

	AdminSecret string `json:"adminsecret,omitempty"`

	// Jenkins service options
	Service *ServiceSpec `json:"service,omitempty"`

	// Service account name for jenkins to run under
	ServiceAccount string `json:"serviceaccount,omitempty"`

	// Jenkins storage options
	Storage *StorageSpec `json:"storage,omitempty"`

	// Affinity settings
	Affinity corev1.Affinity `json:"affinity,omitempty"`

	// dns policy
	DNSPolicy corev1.DNSPolicy `json:"dnspolicy,omitempty"`

	// node selector
	NodeSelector map[string]string `json:"nodeselector,omitempty"`

	// specific node name
	NodeName string `json:"nodename,omitempty"`

	// image pull secrets
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagepullsecrets,omitempty"`

	// tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// container resource requests
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// JenkinsInstanceStatus defines the observed state of JenkinsInstance
type JenkinsInstanceStatus struct {
	// setup secret
	SetupSecret string `json:"adminsecret,omitempty"`

	// state if jenkins server instance
	Phase string `json:"phase"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JenkinsInstance
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
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
