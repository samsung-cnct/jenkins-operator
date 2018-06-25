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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// JenkinsServerLister helps list JenkinsServers.
type JenkinsServerLister interface {
	// List lists all JenkinsServers in the indexer.
	List(selector labels.Selector) (ret []*v1alpha1.JenkinsServer, err error)
	// JenkinsServers returns an object that can list and get JenkinsServers.
	JenkinsServers(namespace string) JenkinsServerNamespaceLister
	JenkinsServerListerExpansion
}

// jenkinsServerLister implements the JenkinsServerLister interface.
type jenkinsServerLister struct {
	indexer cache.Indexer
}

// NewJenkinsServerLister returns a new JenkinsServerLister.
func NewJenkinsServerLister(indexer cache.Indexer) JenkinsServerLister {
	return &jenkinsServerLister{indexer: indexer}
}

// List lists all JenkinsServers in the indexer.
func (s *jenkinsServerLister) List(selector labels.Selector) (ret []*v1alpha1.JenkinsServer, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.JenkinsServer))
	})
	return ret, err
}

// JenkinsServers returns an object that can list and get JenkinsServers.
func (s *jenkinsServerLister) JenkinsServers(namespace string) JenkinsServerNamespaceLister {
	return jenkinsServerNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// JenkinsServerNamespaceLister helps list and get JenkinsServers.
type JenkinsServerNamespaceLister interface {
	// List lists all JenkinsServers in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1alpha1.JenkinsServer, err error)
	// Get retrieves the JenkinsServer from the indexer for a given namespace and name.
	Get(name string) (*v1alpha1.JenkinsServer, error)
	JenkinsServerNamespaceListerExpansion
}

// jenkinsServerNamespaceLister implements the JenkinsServerNamespaceLister
// interface.
type jenkinsServerNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all JenkinsServers in the indexer for a given namespace.
func (s jenkinsServerNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.JenkinsServer, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.JenkinsServer))
	})
	return ret, err
}

// Get retrieves the JenkinsServer from the indexer for a given namespace and name.
func (s jenkinsServerNamespaceLister) Get(name string) (*v1alpha1.JenkinsServer, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("jenkinsserver"), name)
	}
	return obj.(*v1alpha1.JenkinsServer), nil
}
