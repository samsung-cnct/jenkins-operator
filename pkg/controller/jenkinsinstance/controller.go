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

package jenkinsinstance

import (
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/eventhandlers"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/predicates"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/types"
	"k8s.io/client-go/tools/record"

	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	jenkinsv1alpha1client "github.com/maratoid/jenkins-operator/pkg/client/clientset/versioned/typed/jenkins/v1alpha1"
	jenkinsv1alpha1informer "github.com/maratoid/jenkins-operator/pkg/client/informers/externalversions/jenkins/v1alpha1"
	jenkinsv1alpha1lister "github.com/maratoid/jenkins-operator/pkg/client/listers/jenkins/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/maratoid/jenkins-operator/pkg/inject/args"
	"github.com/golang/glog"
	"fmt"
)

// EDIT THIS FILE
// This files was created by "kubebuilder create resource" for you to edit.
// Controller implementation logic for JenkinsInstance resources goes here.

const (
	// SuccessSynced is used as part of the Event 'reason' when a Foo is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Foo fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Foo"
	// MessageResourceSynced is the message used for an Event fired when a Foo
	// is synced successfully
	MessageResourceSynced = "Foo synced successfully"
)

func (bc *JenkinsInstanceController) Reconcile(key types.ReconcileKey) error {
	// INSERT YOUR CODE HERE
	glog.Infof("Implement the Reconcile function on jenkinsinstance.JenkinsInstanceController to reconcile %s\n", key.Name)

	jenkinsInstance, err := bc.jenkinsinstanceclient.
		JenkinsInstances(key.Namespace).
		Get(key.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("JenkinsInstance '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	deploymentName := jenkinsInstance.Spec.Name
	if deploymentName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: deployment name must be specified", k))
		return nil
	}

	// Get the deployment with the name specified in JenkinsInstance.spec
	deployment, err := bc.KubernetesInformers.Apps().V1().Deployments().Lister().Deployments(jenkinsInstance.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = bc.KubernetesClientSet.AppsV1().Deployments(jenkinsInstance.Namespace).Create(newDeployment(jenkinsInstance))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this JenkinsInstance resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(deployment, jenkinsInstance) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		bc.jenkinsinstancerecorder.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// TODO: Update the Deployment iff its observed Spec does
	// TODO: not matched the desired Spec
	// if jenkinsInstance.Spec.Replicas != nil && *foo.Spec.Replicas != *deployment.Spec.Replicas {
	//	glog.V(4).Infof("Foo %s replicas: %d, deployment replicas: %d", name, *foo.Spec.Replicas, *deployment.Spec.Replicas)
    // deployment, err = bc.KubernetesClientSet.AppsV1().Deployments(foo.Namespace).Update(newDeployment(foo))
	//}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the JenkinsInstance resource to reflect the
	// current state of the world
	err = bc.updateJenkinsInstanceStatus(jenkinsInstance, deployment)
	if err != nil {
		return err
	}

	bc.jenkinsinstancerecorder.Event(jenkinsInstance, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// +controller:group=jenkins,version=v1alpha1,kind=JenkinsInstance,resource=jenkinsinstances
// +informers:group=apps,version=v1,kind=Deployment
// +rbac:groups=apps,resources=deployments,verbs=get;list;watch
type JenkinsInstanceController struct {
	// INSERT ADDITIONAL FIELDS HERE
	args.InjectArgs

	jenkinsinstanceLister jenkinsv1alpha1lister.JenkinsInstanceLister
	jenkinsinstanceclient jenkinsv1alpha1client.JenkinsV1alpha1Interface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	jenkinsinstancerecorder record.EventRecorder
}

// ProvideController provides a controller that will be run at startup.  Kubebuilder will use codegeneration
// to automatically register this controller in the inject package
func ProvideController(arguments args.InjectArgs) (*controller.GenericController, error) {
	// INSERT INITIALIZATIONS FOR ADDITIONAL FIELDS HERE
	bc := &JenkinsInstanceController{
		jenkinsinstanceLister: arguments.ControllerManager.GetInformerProvider(&jenkinsv1alpha1.JenkinsInstance{}).(jenkinsv1alpha1informer.JenkinsInstanceInformer).Lister(),

		jenkinsinstanceclient:   arguments.Clientset.JenkinsV1alpha1(),
		jenkinsinstancerecorder: arguments.CreateRecorder("JenkinsInstanceController"),
	}

	// Create a new controller that will call JenkinsInstanceController.Reconcile on changes to JenkinsInstances
	gc := &controller.GenericController{
		Name:             "JenkinsInstanceController",
		Reconcile:        bc.Reconcile,
		InformerRegistry: arguments.ControllerManager,
	}
	if err := gc.Watch(&jenkinsv1alpha1.JenkinsInstance{}); err != nil {
		return gc, err
	}

	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a JenkinsInstance resource will enqueue that JenkinsInstance resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	if err := gc.WatchControllerOf(&appsv1.Deployment{}, eventhandlers.Path{bc.LookupJenkinsInstance},
		predicates.ResourceVersionChanged); err != nil {
		return gc, err
	}

	// IMPORTANT:
	// To watch additional resource types - such as those created by your controller - add gc.Watch* function calls here
	// Watch function calls will transform each object event into a JenkinsInstance Key to be reconciled by the controller.
	//
	// **********
	// For any new Watched types, you MUST add the appropriate // +kubebuilder:informer and // +kubebuilder:rbac
	// annotations to the JenkinsInstanceController and run "kubebuilder generate.
	// This will generate the code to start the informers and create the RBAC rules needed for running in a cluster.
	// See:
	// https://godoc.org/github.com/kubernetes-sigs/kubebuilder/pkg/gen/controller#example-package
	// **********

	return gc, nil
}

// LookupJenkinsInstance looks up a JenkinsInstance from the lister
func (c JenkinsInstanceController) LookupJenkinsInstance(r types.ReconcileKey) (interface{}, error) {
	return c.Informers.Jenkins().V1alpha1().JenkinsInstances().Lister().JenkinsInstances(r.Namespace).Get(r.Name)
}


func (bc *JenkinsInstanceController) updateJenkinsInstanceStatus(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, deployment *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	jenkinsInstanceCopy := jenkinsInstance.DeepCopy()

	// TODO: update status fields
	//jenkinsInstanceCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas
	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the JenkinsInstanceC resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	_, err := bc.Clientset.JenkinsV1alpha1().JenkinsInstances(jenkinsInstance.Namespace).Update(jenkinsInstanceCopy)
	return err
}


// newDeployment creates a new Deployment for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
// TODO: correct image and other parameters
func newDeployment(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "JenkinsCI",
		"controller": jenkinsInstance.Name,
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jenkinsInstance.Spec.Name,
			Namespace: jenkinsInstance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(jenkinsInstance, schema.GroupVersionKind{
					Group:   jenkinsv1alpha1.SchemeGroupVersion.Group,
					Version: jenkinsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "Foo",
				}),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: jenkinsInstance.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "JenkinsCI",
							Image: jenkinsInstance.Spec.Image,
						},
					},
				},
			},
		},
	}
}