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
package fake

import (
	v1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkinsoperator.maratoid.github.com/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeJenkinsServers implements JenkinsServerInterface
type FakeJenkinsServers struct {
	Fake *FakeJenkinsoperatorV1alpha1
	ns   string
}

var jenkinsserversResource = schema.GroupVersionResource{Group: "jenkinsoperator.maratoid.github.com", Version: "v1alpha1", Resource: "jenkinsservers"}

var jenkinsserversKind = schema.GroupVersionKind{Group: "jenkinsoperator.maratoid.github.com", Version: "v1alpha1", Kind: "JenkinsServer"}

// Get takes name of the jenkinsServer, and returns the corresponding jenkinsServer object, and an error if there is any.
func (c *FakeJenkinsServers) Get(name string, options v1.GetOptions) (result *v1alpha1.JenkinsServer, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(jenkinsserversResource, c.ns, name), &v1alpha1.JenkinsServer{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.JenkinsServer), err
}

// List takes label and field selectors, and returns the list of JenkinsServers that match those selectors.
func (c *FakeJenkinsServers) List(opts v1.ListOptions) (result *v1alpha1.JenkinsServerList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(jenkinsserversResource, jenkinsserversKind, c.ns, opts), &v1alpha1.JenkinsServerList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.JenkinsServerList{}
	for _, item := range obj.(*v1alpha1.JenkinsServerList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested jenkinsServers.
func (c *FakeJenkinsServers) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(jenkinsserversResource, c.ns, opts))

}

// Create takes the representation of a jenkinsServer and creates it.  Returns the server's representation of the jenkinsServer, and an error, if there is any.
func (c *FakeJenkinsServers) Create(jenkinsServer *v1alpha1.JenkinsServer) (result *v1alpha1.JenkinsServer, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(jenkinsserversResource, c.ns, jenkinsServer), &v1alpha1.JenkinsServer{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.JenkinsServer), err
}

// Update takes the representation of a jenkinsServer and updates it. Returns the server's representation of the jenkinsServer, and an error, if there is any.
func (c *FakeJenkinsServers) Update(jenkinsServer *v1alpha1.JenkinsServer) (result *v1alpha1.JenkinsServer, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(jenkinsserversResource, c.ns, jenkinsServer), &v1alpha1.JenkinsServer{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.JenkinsServer), err
}

// Delete takes name of the jenkinsServer and deletes it. Returns an error if one occurs.
func (c *FakeJenkinsServers) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(jenkinsserversResource, c.ns, name), &v1alpha1.JenkinsServer{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeJenkinsServers) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(jenkinsserversResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.JenkinsServerList{})
	return err
}

// Patch applies the patch and returns the patched jenkinsServer.
func (c *FakeJenkinsServers) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.JenkinsServer, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(jenkinsserversResource, c.ns, name, data, subresources...), &v1alpha1.JenkinsServer{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.JenkinsServer), err
}
