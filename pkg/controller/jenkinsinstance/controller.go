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
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/maratoid/jenkins-operator/pkg/inject/args"
	"github.com/golang/glog"
	"github.com/maratoid/jenkins-operator/pkg/bindata"
	"github.com/sethvargo/go-password/password"
	"fmt"
	"text/template"
	"bytes"
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

	baseName := jenkinsInstance.Spec.Name
	if baseName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: deployment name must be specified", key.String()))
		return nil
	}

	// Get the secret with the name specified in JenkinsInstance.spec
	secret, err := bc.KubernetesInformers.Core().V1().Secrets().Lister().Secrets(jenkinsInstance.Namespace).Get(baseName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		secret, err = bc.KubernetesClientSet.CoreV1().Secrets(jenkinsInstance.Namespace).Create(newAdminSecret(jenkinsInstance))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating secret: %s", err)
		return err
	}

	// If the Secret is not controlled by this JenkinsInstance resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(secret, jenkinsInstance) {
		msg := fmt.Sprintf(MessageResourceExists, secret.Name)
		bc.jenkinsinstancerecorder.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// Get the deployment with the name specified in JenkinsInstance.spec
	deployment, err := bc.KubernetesInformers.Apps().V1().Deployments().Lister().Deployments(jenkinsInstance.Namespace).Get(baseName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = bc.KubernetesClientSet.AppsV1().Deployments(jenkinsInstance.Namespace).Create(newDeployment(jenkinsInstance))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating deployment: %s", err)
		return err
	}

	// If the Deployment is not controlled by this JenkinsInstance resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(deployment, jenkinsInstance) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		bc.jenkinsinstancerecorder.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// Get the service with the name specified in JenkinsInstance.spec
	service, err := bc.KubernetesInformers.Core().V1().Services().Lister().Services(jenkinsInstance.Namespace).Get(baseName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		service, err = bc.KubernetesClientSet.CoreV1().Services(jenkinsInstance.Namespace).Create(newService(jenkinsInstance))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating service: %s", err)
		return err
	}

	// If the Deployment is not controlled by this JenkinsInstance resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(service, jenkinsInstance) {
		msg := fmt.Sprintf(MessageResourceExists, service.Name)
		bc.jenkinsinstancerecorder.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// Update the Deployment iff its observed Spec does
	// TODO: consider other changes besides replicas number
	if jenkinsInstance.Spec.Replicas != nil && *jenkinsInstance.Spec.Replicas != *deployment.Spec.Replicas {
		glog.V(4).Infof("jenkinsInstance %s replicas: %d, deployment replicas: %d", baseName, *jenkinsInstance.Spec.Replicas, *deployment.Spec.Replicas)
    	deployment, err = bc.KubernetesClientSet.AppsV1().Deployments(jenkinsInstance.Namespace).Update(newDeployment(jenkinsInstance))
	}

	// TODO: Update the Service iff its observed Spec does

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the JenkinsInstance resource to reflect the
	// current state of the world
	err = bc.updateJenkinsInstanceStatus(jenkinsInstance, deployment, service)
	if err != nil {
		return err
	}

	bc.jenkinsinstancerecorder.Event(jenkinsInstance, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// +kubebuilder:controller:group=jenkins,version=v1alpha1,kind=JenkinsInstance,resource=jenkinsinstances
// +kubebuilder:informers:group=apps,version=v1,kind=Deployment
// +kubebuilder:informers:group=core,version=v1,kind=Service
// +kubebuilder:informers:group=core,version=v1,kind=Secret
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch
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
		InjectArgs: arguments,
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

	// Set up an event handler for when Service resources change. This
	// handler will lookup the owner of the given Service, and if it is
	// owned by a JenkinsInstance resource will enqueue that JenkinsInstance resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Service resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	if err := gc.WatchControllerOf(&corev1.Service{}, eventhandlers.Path{bc.LookupJenkinsInstance},
		predicates.ResourceVersionChanged); err != nil {
		return gc, err
	}

	// Set up an event handler for when Secret resources change. This
	// handler will lookup the owner of the given Secret, and if it is
	// owned by a JenkinsInstance resource will enqueue that JenkinsInstance resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Secret resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	if err := gc.WatchControllerOf(&corev1.Secret{}, eventhandlers.Path{bc.LookupJenkinsInstance},
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

func (bc *JenkinsInstanceController) updateJenkinsInstanceStatus(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, deployment *appsv1.Deployment, service *corev1.Service) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	jenkinsInstanceCopy := jenkinsInstance.DeepCopy()

	// TODO: update other status fields
	jenkinsInstanceCopy.Status.Phase = "Ready"
	jenkinsInstanceCopy.Status.Api = "http://" + service.Name + "." + service.Namespace + ".svc." + "cluster.local:" + string(jenkinsInstance.Spec.MasterPort)
	jenkinsInstanceCopy.Status.AdminSecret = jenkinsInstanceCopy.Spec.Name
	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the JenkinsInstanceC resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	_, err := bc.Clientset.JenkinsV1alpha1().JenkinsInstances(jenkinsInstance.Namespace).Update(jenkinsInstanceCopy)
	return err
}

// newAdminSecret creates an admin password secret for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func newAdminSecret(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) *corev1.Secret {
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.Name,
	}

	adminpass, err := password.Generate(10, 3, 3, false, false)
	if err != nil {
		glog.Errorf("Error generating admin password: %s", err)
		return nil
	}

	adminUserConfig, err := bindata.Asset("init-groovy/jenkins_security.groovy")
	if err != nil {
		glog.Errorf("Error locating binary asset: %s", err)
		return nil
	}

	type JenkinsInfo struct {
		User 		string
		Password 	string
		Url			string
		AdminEmail	string
		AgentPort	int32
	}

	jenkinsInfo := JenkinsInfo{
		User: jenkinsInstance.Spec.AdminUser,
		Password: string(adminpass[:]),
		Url: jenkinsInstance.Spec.Location,
		AdminEmail: jenkinsInstance.Spec.AdminEmail,
		AgentPort: jenkinsInstance.Spec.AgentPort,
	}

	// parse the groovy config template
	configTemplate, err := template.New("jenkins-config").Parse(string(adminUserConfig[:]))
	if err != nil {
		glog.Errorf("Failed to parse jenkins config template: %s", err)
		return nil
	}

	var jenkinsConfigParsed bytes.Buffer
	if err := configTemplate.Execute(&jenkinsConfigParsed, jenkinsInfo); err != nil {
		glog.Errorf("Failed to execute jenkins config template: %s", err)
		return nil
	}

	authContents := fmt.Sprintf("%s:%s", jenkinsInstance.Spec.AdminUser, string(adminpass[:]))

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: jenkinsInstance.Spec.Name,
			Namespace: jenkinsInstance.Namespace,
			Labels: labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(jenkinsInstance, schema.GroupVersionKind{
					Group:   jenkinsv1alpha1.SchemeGroupVersion.Group,
					Version: jenkinsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "JenkinsInstance",
				}),
			},
		},

		StringData: map[string]string{
			"password": adminpass,
			"jenkins_security.groovy": jenkinsConfigParsed.String(),
			"cliAuth": authContents,
		},

		Type: corev1.SecretTypeOpaque,
	}
}

// newService creates a new Service for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func newService(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) *corev1.Service {
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.Name,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: jenkinsInstance.Spec.Name,
			Namespace: jenkinsInstance.Namespace,
			Labels: labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(jenkinsInstance, schema.GroupVersionKind{
					Group:   jenkinsv1alpha1.SchemeGroupVersion.Group,
					Version: jenkinsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "JenkinsInstance",
				}),
			},
		},

		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name: "master",
					Protocol: "TCP",
					Port: jenkinsInstance.Spec.MasterPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: jenkinsInstance.Spec.MasterPort,
					},
				},
				{
					Name: "agent",
					Protocol: "TCP",
					Port: jenkinsInstance.Spec.AgentPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: jenkinsInstance.Spec.AgentPort,
					},
				},
			},
			Selector: map[string]string{
				"component": string(jenkinsInstance.UID),
			},
			Type: jenkinsInstance.Spec.ServiceType,
		},
	}
}

// newDeployment creates a new Deployment for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func newDeployment(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) *appsv1.Deployment {
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.Name,
		"component": string(jenkinsInstance.UID),
	}

	javaOptsName, err := bindata.Asset("java_opts")
	if err != nil {
		glog.Errorf("Error locating binary asset: %s", err)
		return nil
	}
	javaOptsVal, err := bindata.Asset("java_opts.values")
	if err != nil {
		glog.Errorf("Error locating binary asset: %s", err)
		return nil
	}

	jenkinsOptsName, err := bindata.Asset("jenkins_opts")
	if err != nil {
		glog.Errorf("Error locating binary asset: %s", err)
		return nil
	}
	jenkinsOptsVal, err := bindata.Asset("jenkins_opts.values")
	if err != nil {
		glog.Errorf("Error locating binary asset: %s", err)
		return nil
	}

	var env []corev1.EnvVar
	env = append(env, corev1.EnvVar{
		Name:      string(javaOptsName[:]),
		Value:     string(javaOptsVal[:]),
	})
	env = append(env, corev1.EnvVar{
		Name:      string(jenkinsOptsName[:]),
		Value:     string(jenkinsOptsVal[:]),
	})
	for envVar, envVarVal := range jenkinsInstance.Spec.Env {
		env = append(env, corev1.EnvVar{
			Name:      envVar,
			Value:     envVarVal,
		})
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jenkinsInstance.Spec.Name,
			Namespace: jenkinsInstance.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(jenkinsInstance, schema.GroupVersionKind{
					Group:   jenkinsv1alpha1.SchemeGroupVersion.Group,
					Version: jenkinsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "JenkinsInstance",
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
							Name:  "jenkinsci",
							Image: jenkinsInstance.Spec.Image,
							Ports: []corev1.ContainerPort{
								{
									Name:"master",
									ContainerPort: jenkinsInstance.Spec.MasterPort,
									HostPort: jenkinsInstance.Spec.MasterPort,
									Protocol: "TCP",
								},
								{
									Name:"agent",
									ContainerPort: jenkinsInstance.Spec.AgentPort,
									HostPort: jenkinsInstance.Spec.AgentPort,
									Protocol: "TCP",
								},
							},
							Env: env,
							ImagePullPolicy: jenkinsInstance.Spec.PullPolicy,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name: "init-groovy-d",
									ReadOnly: true,
									MountPath: "/var/jenkins_home/init.groovy.d",
								},
								{
									Name: "cli-auth",
									ReadOnly: true,
									MountPath: "/var/jenkins_home/cli-auth",
								},
							},
						},
					},

					Volumes: []corev1.Volume{
						{
							Name: "init-groovy-d",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource {
									SecretName: jenkinsInstance.Spec.Name,
									Items: []corev1.KeyToPath{
										{
											Key: "jenkins_security.groovy",
											Path: "jenkins_security.groovy",
										},
									},
								},
							},
						},
						{
							Name: "cli-auth",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource {
									SecretName: jenkinsInstance.Spec.Name,
									Items: []corev1.KeyToPath{
										{
											Key: "cliAuth",
											Path: "cliAuth",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}