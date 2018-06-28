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

// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// JenkinsInstanceLister helps list JenkinsInstances.
type JenkinsInstanceLister interface {
	// List lists all JenkinsInstances in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.JenkinsInstance, err error)
	// JenkinsInstances returns an object that can list and get JenkinsInstances.
	JenkinsInstances(namespace string) JenkinsInstanceNamespaceLister
	JenkinsInstanceListerExpansion
}

// jenkinsInstanceLister implements the JenkinsInstanceLister interface.
type jenkinsInstanceLister struct {
	indexer cache.Indexer
}

// NewJenkinsInstanceLister returns a new JenkinsInstanceLister.
func NewJenkinsInstanceLister(indexer cache.Indexer) JenkinsInstanceLister {
	return &jenkinsInstanceLister{indexer: indexer}
}

// List lists all JenkinsInstances in the indexer.
func (s *jenkinsInstanceLister) List(selector labels.Selector) (ret []*v1alpha1.JenkinsInstance, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.JenkinsInstance))
	})
	return ret, err
}

// JenkinsInstances returns an object that can list and get JenkinsInstances.
func (s *jenkinsInstanceLister) JenkinsInstances(namespace string) JenkinsInstanceNamespaceLister {
	return jenkinsInstanceNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// JenkinsInstanceNamespaceLister helps list and get JenkinsInstances.
type JenkinsInstanceNamespaceLister interface {
	// List lists all JenkinsInstances in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.JenkinsInstance, err error)
	// Get retrieves the JenkinsInstance from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.JenkinsInstance, error)
	JenkinsInstanceNamespaceListerExpansion
}

// jenkinsInstanceNamespaceLister implements the JenkinsInstanceNamespaceLister
// interface.
type jenkinsInstanceNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all JenkinsInstances in the indexer for a given namespace.
func (s jenkinsInstanceNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.JenkinsInstance, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.JenkinsInstance))
	})
	return ret, err
}

// Get retrieves the JenkinsInstance from the indexer for a given namespace and name.
func (s jenkinsInstanceNamespaceLister) Get(name string) (*v1alpha1.JenkinsInstance, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("jenkinsinstance"), name)
	}
	return obj.(*v1alpha1.JenkinsInstance), nil
}