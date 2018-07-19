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

package jenkinsjob

import (
	"log"

	"github.com/kubernetes-sigs/kubebuilder/pkg/controller"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/types"
	"k8s.io/client-go/tools/record"

	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	jenkinsv1alpha1client "github.com/maratoid/jenkins-operator/pkg/client/clientset/versioned/typed/jenkins/v1alpha1"
	jenkinsv1alpha1informer "github.com/maratoid/jenkins-operator/pkg/client/informers/externalversions/jenkins/v1alpha1"
	jenkinsv1alpha1lister "github.com/maratoid/jenkins-operator/pkg/client/listers/jenkins/v1alpha1"

	"github.com/maratoid/jenkins-operator/pkg/inject/args"
)

// EDIT THIS FILE
// This files was created by "kubebuilder create resource" for you to edit.
// Controller implementation logic for JenkinsJob resources goes here.

func (bc *JenkinsJobController) Reconcile(k types.ReconcileKey) error {
	// INSERT YOUR CODE HERE
	log.Printf("Implement the Reconcile function on jenkinsjob.JenkinsJobController to reconcile %s\n", k.Name)
	return nil
}

// +kubebuilder:controller:group=jenkins,version=v1alpha1,kind=JenkinsJob,resource=jenkinsjobs
type JenkinsJobController struct {
	// INSERT ADDITIONAL FIELDS HERE
	jenkinsjobLister jenkinsv1alpha1lister.JenkinsJobLister
	jenkinsjobclient jenkinsv1alpha1client.JenkinsV1alpha1Interface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	jenkinsjobrecorder record.EventRecorder
}

// ProvideController provides a controller that will be run at startup.  Kubebuilder will use codegeneration
// to automatically register this controller in the inject package
func ProvideController(arguments args.InjectArgs) (*controller.GenericController, error) {
	// INSERT INITIALIZATIONS FOR ADDITIONAL FIELDS HERE
	bc := &JenkinsJobController{
		jenkinsjobLister: arguments.ControllerManager.GetInformerProvider(&jenkinsv1alpha1.JenkinsJob{}).(jenkinsv1alpha1informer.JenkinsJobInformer).Lister(),

		jenkinsjobclient:   arguments.Clientset.JenkinsV1alpha1(),
		jenkinsjobrecorder: arguments.CreateRecorder("JenkinsJobController"),
	}

	// Create a new controller that will call JenkinsJobController.Reconcile on changes to JenkinsJobs
	gc := &controller.GenericController{
		Name:             "JenkinsJobController",
		Reconcile:        bc.Reconcile,
		InformerRegistry: arguments.ControllerManager,
	}
	if err := gc.Watch(&jenkinsv1alpha1.JenkinsJob{}); err != nil {
		return gc, err
	}

	// IMPORTANT:
	// To watch additional resource types - such as those created by your controller - add gc.Watch* function calls here
	// Watch function calls will transform each object event into a JenkinsJob Key to be reconciled by the controller.
	//
	// **********
	// For any new Watched types, you MUST add the appropriate // +kubebuilder:informer and // +kubebuilder:rbac
	// annotations to the JenkinsJobController and run "kubebuilder generate.
	// This will generate the code to start the informers and create the RBAC rules needed for running in a cluster.
	// See:
	// https://godoc.org/github.com/kubernetes-sigs/kubebuilder/pkg/gen/controller#example-package
	// **********

	return gc, nil
}