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
package v1alpha1

import (
	v1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkinsoperator.maratoid.github.com/v1alpha1"
	scheme "github.com/maratoid/jenkins-operator/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// JenkinsServersGetter has a method to return a JenkinsServerInterface.
// A group's client should implement this interface.
type JenkinsServersGetter interface {
	JenkinsServers(namespace string) JenkinsServerInterface
}

// JenkinsServerInterface has methods to work with JenkinsServer resources.
type JenkinsServerInterface interface {
	Create(*v1alpha1.JenkinsServer) (*v1alpha1.JenkinsServer, error)
	Update(*v1alpha1.JenkinsServer) (*v1alpha1.JenkinsServer, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.JenkinsServer, error)
	List(opts v1.ListOptions) (*v1alpha1.JenkinsServerList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.JenkinsServer, err error)
	JenkinsServerExpansion
}

// jenkinsServers implements JenkinsServerInterface
type jenkinsServers struct {
	client rest.Interface
	ns     string
}

// newJenkinsServers returns a JenkinsServers
func newJenkinsServers(c *JenkinsoperatorV1alpha1Client, namespace string) *jenkinsServers {
	return &jenkinsServers{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the jenkinsServer, and returns the corresponding jenkinsServer object, and an error if there is any.
func (c *jenkinsServers) Get(name string, options v1.GetOptions) (result *v1alpha1.JenkinsServer, err error) {
	result = &v1alpha1.JenkinsServer{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("jenkinsservers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of JenkinsServers that match those selectors.
func (c *jenkinsServers) List(opts v1.ListOptions) (result *v1alpha1.JenkinsServerList, err error) {
	result = &v1alpha1.JenkinsServerList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("jenkinsservers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested jenkinsServers.
func (c *jenkinsServers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("jenkinsservers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a jenkinsServer and creates it.  Returns the server's representation of the jenkinsServer, and an error, if there is any.
func (c *jenkinsServers) Create(jenkinsServer *v1alpha1.JenkinsServer) (result *v1alpha1.JenkinsServer, err error) {
	result = &v1alpha1.JenkinsServer{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("jenkinsservers").
		Body(jenkinsServer).
		Do().
		Into(result)
	return
}

// Update takes the representation of a jenkinsServer and updates it. Returns the server's representation of the jenkinsServer, and an error, if there is any.
func (c *jenkinsServers) Update(jenkinsServer *v1alpha1.JenkinsServer) (result *v1alpha1.JenkinsServer, err error) {
	result = &v1alpha1.JenkinsServer{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("jenkinsservers").
		Name(jenkinsServer.Name).
		Body(jenkinsServer).
		Do().
		Into(result)
	return
}

// Delete takes name of the jenkinsServer and deletes it. Returns an error if one occurs.
func (c *jenkinsServers) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("jenkinsservers").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *jenkinsServers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("jenkinsservers").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched jenkinsServer.
func (c *jenkinsServers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.JenkinsServer, err error) {
	result = &v1alpha1.JenkinsServer{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("jenkinsservers").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
