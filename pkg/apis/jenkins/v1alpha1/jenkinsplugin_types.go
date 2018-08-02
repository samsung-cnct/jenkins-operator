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

// JenkinsPluginSpec defines the desired state of JenkinsPlugin
type JenkinsPluginSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "kubebuilder generate" to regenerate code after modifying this file

	// Name of this JenkinsPlugin Object
	Name string `json:"name,omitempty"`

	// ID of the JenkinsServer instance to install this plugin in
	JenkinsInstance string `json:"jenkinsinstance,omitempty"`

	// plugin Id string
	PluginId string `json:"pluginid,omitempty"`

	// plugin Version string
	PluginVersion string `json:"pluginversion,omitempty"`

	// Groovy configuration scripts
	PluginConfig string `json:"pluginconfig,omitempty"`
}

// JenkinsPluginStatus defines the observed state of JenkinsPlugin
type JenkinsPluginStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JenkinsPlugin
// +k8s:openapi-gen=true
// +kubebuilder:resource:path=jenkinsplugins
type JenkinsPlugin struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JenkinsPluginSpec   `json:"spec,omitempty"`
	Status JenkinsPluginStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// JenkinsPluginList contains a list of JenkinsPlugin
type JenkinsPluginList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JenkinsPlugin `json:"items"`
}

func init() {
	SchemeBuilder.Register(&JenkinsPlugin{}, &JenkinsPluginList{})
}
