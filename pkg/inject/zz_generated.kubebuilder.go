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
package inject

import (
	"github.com/kubernetes-sigs/kubebuilder/pkg/inject/run"
	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	rscheme "github.com/maratoid/jenkins-operator/pkg/client/clientset/versioned/scheme"
	"github.com/maratoid/jenkins-operator/pkg/controller/jenkinsinstance"
	"github.com/maratoid/jenkins-operator/pkg/controller/jenkinsplugin"
	"github.com/maratoid/jenkins-operator/pkg/controller/secret"
	"github.com/maratoid/jenkins-operator/pkg/inject/args"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	rscheme.AddToScheme(scheme.Scheme)

	// Inject Informers
	Inject = append(Inject, func(arguments args.InjectArgs) error {
		Injector.ControllerManager = arguments.ControllerManager

		if err := arguments.ControllerManager.AddInformerProvider(&jenkinsv1alpha1.JenkinsInstance{}, arguments.Informers.Jenkins().V1alpha1().JenkinsInstances()); err != nil {
			return err
		}
		if err := arguments.ControllerManager.AddInformerProvider(&jenkinsv1alpha1.JenkinsPlugin{}, arguments.Informers.Jenkins().V1alpha1().JenkinsPlugins()); err != nil {
			return err
		}

		// Add Kubernetes informers
		if err := arguments.ControllerManager.AddInformerProvider(&appsv1.Deployment{}, arguments.KubernetesInformers.Apps().V1().Deployments()); err != nil {
			return err
		}
		if err := arguments.ControllerManager.AddInformerProvider(&corev1.Service{}, arguments.KubernetesInformers.Core().V1().Services()); err != nil {
			return err
		}
		if err := arguments.ControllerManager.AddInformerProvider(&corev1.Secret{}, arguments.KubernetesInformers.Core().V1().Secrets()); err != nil {
			return err
		}
		if err := arguments.ControllerManager.AddInformerProvider(&batchv1.Job{}, arguments.KubernetesInformers.Batch().V1().Jobs()); err != nil {
			return err
		}

		if c, err := jenkinsinstance.ProvideController(arguments); err != nil {
			return err
		} else {
			arguments.ControllerManager.AddController(c)
		}
		if c, err := jenkinsplugin.ProvideController(arguments); err != nil {
			return err
		} else {
			arguments.ControllerManager.AddController(c)
		}
		if c, err := secret.ProvideController(arguments); err != nil {
			return err
		} else {
			arguments.ControllerManager.AddController(c)
		}
		return nil
	})

	// Inject CRDs
	Injector.CRDs = append(Injector.CRDs, &jenkinsv1alpha1.JenkinsInstanceCRD)
	Injector.CRDs = append(Injector.CRDs, &jenkinsv1alpha1.JenkinsPluginCRD)
	// Inject PolicyRules
	Injector.PolicyRules = append(Injector.PolicyRules, rbacv1.PolicyRule{
		APIGroups: []string{"jenkins.jenkinsoperator.maratoid.github.com"},
		Resources: []string{"*"},
		Verbs:     []string{"*"},
	})
	Injector.PolicyRules = append(Injector.PolicyRules, rbacv1.PolicyRule{
		APIGroups: []string{
			"apps",
		},
		Resources: []string{
			"deployments",
		},
		Verbs: []string{
			"get", "list", "watch",
		},
	})
	Injector.PolicyRules = append(Injector.PolicyRules, rbacv1.PolicyRule{
		APIGroups: []string{
			"",
		},
		Resources: []string{
			"services",
		},
		Verbs: []string{
			"get", "list", "watch",
		},
	})
	Injector.PolicyRules = append(Injector.PolicyRules, rbacv1.PolicyRule{
		APIGroups: []string{
			"",
		},
		Resources: []string{
			"secrets",
		},
		Verbs: []string{
			"get", "list", "watch",
		},
	})
	Injector.PolicyRules = append(Injector.PolicyRules, rbacv1.PolicyRule{
		APIGroups: []string{
			"batch",
		},
		Resources: []string{
			"jobs",
		},
		Verbs: []string{
			"get", "list", "watch",
		},
	})
	// Inject GroupVersions
	Injector.GroupVersions = append(Injector.GroupVersions, schema.GroupVersion{
		Group:   "jenkins.jenkinsoperator.maratoid.github.com",
		Version: "v1alpha1",
	})
	Injector.RunFns = append(Injector.RunFns, func(arguments run.RunArguments) error {
		Injector.ControllerManager.RunInformersAndControllers(arguments)
		return nil
	})
}
