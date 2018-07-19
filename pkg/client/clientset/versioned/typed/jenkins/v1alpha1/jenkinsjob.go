/*
Copyright 2018 Samsung SDS Cloud Native Computing Team.

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

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	scheme "github.com/maratoid/jenkins-operator/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// JenkinsJobsGetter has a method to return a JenkinsJobInterface.
// A group's client should implement this interface.
type JenkinsJobsGetter interface {
	JenkinsJobs(namespace string) JenkinsJobInterface
}

// JenkinsJobInterface has methods to work with JenkinsJob resources.
type JenkinsJobInterface interface {
	Create(*v1alpha1.JenkinsJob) (*v1alpha1.JenkinsJob, error)
	Update(*v1alpha1.JenkinsJob) (*v1alpha1.JenkinsJob, error)
	UpdateStatus(*v1alpha1.JenkinsJob) (*v1alpha1.JenkinsJob, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.JenkinsJob, error)
	List(opts v1.ListOptions) (*v1alpha1.JenkinsJobList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.JenkinsJob, err error)
	JenkinsJobExpansion
}

// jenkinsJobs implements JenkinsJobInterface
type jenkinsJobs struct {
	client rest.Interface
	ns     string
}

// newJenkinsJobs returns a JenkinsJobs
func newJenkinsJobs(c *JenkinsV1alpha1Client, namespace string) *jenkinsJobs {
	return &jenkinsJobs{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the jenkinsJob, and returns the corresponding jenkinsJob object, and an error if there is any.
func (c *jenkinsJobs) Get(name string, options v1.GetOptions) (result *v1alpha1.JenkinsJob, err error) {
	result = &v1alpha1.JenkinsJob{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("jenkinsjobs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of JenkinsJobs that match those selectors.
func (c *jenkinsJobs) List(opts v1.ListOptions) (result *v1alpha1.JenkinsJobList, err error) {
	result = &v1alpha1.JenkinsJobList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("jenkinsjobs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested jenkinsJobs.
func (c *jenkinsJobs) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("jenkinsjobs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a jenkinsJob and creates it.  Returns the server's representation of the jenkinsJob, and an error, if there is any.
func (c *jenkinsJobs) Create(jenkinsJob *v1alpha1.JenkinsJob) (result *v1alpha1.JenkinsJob, err error) {
	result = &v1alpha1.JenkinsJob{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("jenkinsjobs").
		Body(jenkinsJob).
		Do().
		Into(result)
	return
}

// Update takes the representation of a jenkinsJob and updates it. Returns the server's representation of the jenkinsJob, and an error, if there is any.
func (c *jenkinsJobs) Update(jenkinsJob *v1alpha1.JenkinsJob) (result *v1alpha1.JenkinsJob, err error) {
	result = &v1alpha1.JenkinsJob{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("jenkinsjobs").
		Name(jenkinsJob.Name).
		Body(jenkinsJob).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *jenkinsJobs) UpdateStatus(jenkinsJob *v1alpha1.JenkinsJob) (result *v1alpha1.JenkinsJob, err error) {
	result = &v1alpha1.JenkinsJob{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("jenkinsjobs").
		Name(jenkinsJob.Name).
		SubResource("status").
		Body(jenkinsJob).
		Do().
		Into(result)
	return
}

// Delete takes name of the jenkinsJob and deletes it. Returns an error if one occurs.
func (c *jenkinsJobs) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("jenkinsjobs").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *jenkinsJobs) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("jenkinsjobs").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched jenkinsJob.
func (c *jenkinsJobs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.JenkinsJob, err error) {
	result = &v1alpha1.JenkinsJob{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("jenkinsjobs").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}