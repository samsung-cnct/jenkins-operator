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

package secret

import (
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/types"
	"k8s.io/client-go/tools/record"

	corev1 "k8s.io/api/core/v1"
	corev1informer "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"github.com/maratoid/jenkins-operator/pkg/inject/args"
	"k8s.io/apimachinery/pkg/labels"
	"github.com/golang/glog"
)

// EDIT THIS FILE
// This files was created by "kubebuilder create resource" for you to edit.
// Controller implementation logic for Secret resources goes here.

func (bc *SecretController) Reconcile(key types.ReconcileKey) error {
	allInstances, err := bc.Informers.Jenkins().V1alpha1().JenkinsInstances().Lister().JenkinsInstances(key.Namespace).List(labels.Everything())
	if err != nil {
		glog.Errorf("Could not list all JenkinsInstances for secret %s: %v", key.Name, err)
		return err
	}

	for _, instance := range allInstances {
		if instance.Spec.AdminSecret == key.Name {
			bc.ControllerManager.GetController("JenkinsInstanceController").Reconcile(types.ReconcileKey{Name:instance.Name, Namespace:instance.Namespace})
		}
	}

	return nil
}

// +kubebuilder:informers:group=core,version=v1,kind=Secret
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;watch;list
// +kubebuilder:controller:group=core,version=v1,kind=Secret,resource=secrets
type SecretController struct {
	// INSERT ADDITIONAL FIELDS HERE
	args.InjectArgs
	secretLister corev1lister.SecretLister
	secretclient corev1client.CoreV1Interface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	secretrecorder record.EventRecorder
}

// ProvideController provides a controller that will be run at startup.  Kubebuilder will use codegeneration
// to automatically register this controller in the inject package
func ProvideController(arguments args.InjectArgs) (*controller.GenericController, error) {
	// INSERT INITIALIZATIONS FOR ADDITIONAL FIELDS HERE
	bc := &SecretController{
		InjectArgs: arguments,
		secretLister:   arguments.ControllerManager.GetInformerProvider(&corev1.Secret{}).(corev1informer.SecretInformer).Lister(),
		secretclient:   arguments.KubernetesClientSet.CoreV1(),
		secretrecorder: arguments.CreateRecorder("SecretController"),
	}

	// Create a new controller that will call SecretController.Reconcile on changes to Secrets
	gc := &controller.GenericController{
		Name:             "SecretController",
		Reconcile:        bc.Reconcile,
		InformerRegistry: arguments.ControllerManager,
	}
	if err := gc.Watch(&corev1.Secret{}); err != nil {
		return gc, err
	}

	// IMPORTANT:
	// To watch additional resource types - such as those created by your controller - add gc.Watch* function calls here
	// Watch function calls will transform each object event into a Secret Key to be reconciled by the controller.
	//
	// **********
	// For any new Watched types, you MUST add the appropriate // +kubebuilder:informer and // +kubebuilder:rbac
	// annotations to the SecretController and run "kubebuilder generate.
	// This will generate the code to start the informers and create the RBAC rules needed for running in a cluster.
	// See:
	// https://godoc.org/github.com/kubernetes-sigs/kubebuilder/pkg/gen/controller#example-package
	// **********

	return gc, nil
}
