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
	"github.com/maratoid/jenkins-operator/pkg/util"

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
	"github.com/sethgrid/pester"
	"fmt"
	"text/template"
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"net/http"
	goerr "errors"
	"k8s.io/apimachinery/pkg/labels"
	"strings"
)

// EDIT THIS FILE
// This files was created by "kubebuilder create resource" for you to edit.
// Controller implementation logic for JenkinsInstance resources goes here.

const (
	// SuccessSynced is used as part of the Event 'reason' when a JenkinsInstance is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a JenkinsInstance fails
	// to sync due to a resource of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a resource already existing
	MessageResourceExists = "Resource %q already exists and is not managed by JenkinsInstance"
	// MessageResourceSynced is the message used for an Event fired when a JenkinsInstance
	// is synced successfully
	MessageResourceSynced = "JenkinsInstance synced successfully"
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

	adminSecretName := jenkinsInstance.Spec.AdminSecret
	if adminSecretName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: AdminSecret name must be specified", key.String()))
		return nil
	}

	adminSecret, err := bc.KubernetesInformers.Core().V1().Secrets().Lister().Secrets(jenkinsInstance.Namespace).Get(adminSecretName)
	if errors.IsNotFound(err) {
		glog.Errorf("AdminSecret %s does not exist yet: %v", adminSecretName, err)
		return err
	}

	// Get the secret with the name specified in JenkinsInstance.spec
	setupSecret, err := bc.KubernetesInformers.Core().V1().Secrets().Lister().Secrets(jenkinsInstance.Namespace).Get(baseName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		setupSecret, err = bc.KubernetesClientSet.CoreV1().Secrets(jenkinsInstance.Namespace).Create(newSetupSecret(jenkinsInstance, adminSecret))
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
	if !metav1.IsControlledBy(setupSecret, jenkinsInstance) {
		msg := fmt.Sprintf(MessageResourceExists, setupSecret.Name)
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

	// If the Service is not controlled by this JenkinsInstance resource, we should log
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
	err = bc.updateJenkinsInstanceStatus(jenkinsInstance, deployment, service, adminSecret, setupSecret)
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

	// Set up an event handler for when Secret resources not owned by the JenkinsInstance change
	// This is needed for re-loading login information from the pre-provided secret
	// When admin login secret changes, WatchTransformationKeysOf will list all JenkinsInstances
	// and re-enqueue the keys for the ones that refer to that admin login secret via their spec.
	gc.WatchTransformationKeysOf(&corev1.Secret{},func(i interface{}) []types.ReconcileKey {
		p, ok := i.(*corev1.Secret)
		if !ok {
			return []types.ReconcileKey{}
		}

		// list all jenkins instances and look for ones referring to this secret via CR spec
		instances, err := bc.Informers.Jenkins().V1alpha1().JenkinsInstances().Lister().JenkinsInstances(p.Namespace).List(labels.Everything())
		if err != nil {
			glog.Errorf("Could not list JenkinsInstances")
			return []types.ReconcileKey{}
		}

		var keys []types.ReconcileKey
		for _, inst := range instances {
			if inst.Spec.AdminSecret == p.Name {
				keys = append(keys, types.ReconcileKey {
					Namespace: p.Namespace,
					Name: inst.Name,
				})
			}
		}

		// return found keys
		return keys
	},
	)

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
func (bc JenkinsInstanceController) LookupJenkinsInstance(r types.ReconcileKey) (interface{}, error) {
	return bc.Informers.Jenkins().V1alpha1().JenkinsInstances().Lister().JenkinsInstances(r.Namespace).Get(r.Name)
}

func (bc *JenkinsInstanceController) updateJenkinsInstanceStatus(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, deployment *appsv1.Deployment, service *corev1.Service, adminSecret *corev1.Secret, setupSecret *corev1.Secret) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	jenkinsInstanceCopy := jenkinsInstance.DeepCopy()

	// use admin secret data to get the user configuration page and parse out an api token
	apiToken, err := getJenkinsApiToken(jenkinsInstance, service, adminSecret)
	if err != nil {
		glog.Errorf("Error: %v", err)
		return err
	}

	setupSecretCopy := setupSecret.DeepCopy()
	setupSecretCopy.Data["apiToken"] = []byte(apiToken)
	_, err = bc.KubernetesClientSet.CoreV1().Secrets(jenkinsInstance.Namespace).Update(setupSecretCopy)
	if err != nil {
		glog.Errorf("Error updating secret %s: %v", setupSecret.Name, err)
		return err
	}


	// TODO: update other status fields
	api, err := util.GetServiceEndpoint(service, "", 8080)
	if err != nil {
		return err
	}

	jenkinsInstanceCopy.Status.Api = api
	jenkinsInstanceCopy.Status.SetupSecret = setupSecretCopy.Name
	jenkinsInstanceCopy.Status.Phase = "Ready"

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the JenkinsInstanceC resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	_, err = bc.Clientset.JenkinsV1alpha1().JenkinsInstances(jenkinsInstance.Namespace).Update(jenkinsInstanceCopy)
	return err
}

// newSetupSecret creates an admin password secret for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func newSetupSecret(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, adminSecret *corev1.Secret) *corev1.Secret {
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.Name,
		"component": string(jenkinsInstance.UID),
	}

	adminUserConfig, err := bindata.Asset("init-groovy/0-jenkins-config.groovy")
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
		Executors	int32
	}

	// decode Admin secret strings
	adminUser := string(adminSecret.Data["user"][:])
	adminPassword := string(adminSecret.Data["pass"][:])
	if err != nil {
		glog.Errorf("ErrorDecoding admin secret %s pass: %v", adminSecret.Name, err)
		return nil
	}

	jenkinsInfo := JenkinsInfo{
		User: string(adminUser[:]),
		Password: string(adminPassword[:]),
		Url: jenkinsInstance.Spec.Location,
		AdminEmail: jenkinsInstance.Spec.AdminEmail,
		AgentPort: jenkinsInstance.Spec.AgentPort,
		Executors: jenkinsInstance.Spec.Executors,
	}

	// parse the plugin array
	plugins := jenkinsInstance.Spec.Plugins
	var pluginList []string
	for _, plugin := range plugins {
		pluginInfo := fmt.Sprintf("%s:%s", plugin.Id, plugin.Version)
		pluginList = append(pluginList, pluginInfo)
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

	// add things to the string data
	stringData := map[string]string{
		"0-jenkins-config.groovy": jenkinsConfigParsed.String(),
		"1-user-config.groovy": jenkinsInstance.Spec.Config,
		"plugins.txt": strings.Join(pluginList, "\n"),
		"user": adminUser,
	}

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
		StringData: stringData,
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
		"component": string(jenkinsInstance.UID),
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

	// get binary data for variables and groovy config
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

	// Create environment variables
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

	// build a command string to install plugins and launch jenkins
	commandString := ""
	commandString += "/usr/local/bin/install-plugins.sh $(cat /var/jenkins_home/init.groovy.d/plugins.txt | tr '\\n' ' ') && "
	commandString += "/sbin/tini -- /usr/local/bin/jenkins.sh"
	commandString += ""

	// Get the correct volume source to use
	pvcName := jenkinsInstance.Spec.JobsPvc
	var volumeSource corev1.VolumeSource
	if pvcName == "" {
		volumeSource = corev1.VolumeSource {
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}
	} else {
		volumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
				ReadOnly: false,
			},
		}
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
							Command: []string {
								"bash",
								"-c",
								commandString,
							},
							ImagePullPolicy: jenkinsInstance.Spec.PullPolicy,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "init-groovy-d",
									ReadOnly:  false,
									MountPath: "/var/jenkins_home/init.groovy.d",
								},
								{
									Name:      "job-storage",
									ReadOnly:  false,
									MountPath: "/var/jenkins_home/jobs",
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
								},
							},
						},
						{
							Name: "job-storage",
							VolumeSource: volumeSource,
						},
					},
				},
			},
		},
	}
}

func getJenkinsApiToken(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, service *corev1.Service, adminSecret *corev1.Secret) (string, error) {

	serviceUrl, err := util.GetServiceEndpoint(service, "me/configure", 8080)
	if err != nil {
		return "", err
	}

	adminUser := string(adminSecret.Data["user"][:])
	adminPassword := string(adminSecret.Data["pass"][:])

	// get the user config page
	req, err := http.NewRequest("GET", serviceUrl, nil)
	if err != nil {
		glog.Errorf("Error creating GET request to %s", serviceUrl)
		return "", err
	}

	req.SetBasicAuth(string(adminUser[:]), string(adminPassword[:]))
	if err != nil {
		glog.Errorf("Error setting basic auth on GET request to %s", serviceUrl)
		return "", err
	}

	client := pester.New()
	client.MaxRetries = 5
	client.Backoff = pester.ExponentialBackoff
	client.KeepLog = true

	resp, err := client.Do(req)
	if err != nil {
		glog.Errorf("Error performing GET request to %s: %v", serviceUrl, err)
		return "", err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		glog.Errorf("Error parsing response: %v", err)
		return "", err
	}

	var apiToken string = ""
	doc.Find("input#apiToken").Each(func(i int, element *goquery.Selection) {
		value, exists := element.Attr("value")
		if exists {
			apiToken = value
		}
	})

	if apiToken == "" {
		err = goerr.New("element 'apiToken' missing value")
	} else {
		err = nil
	}

	return apiToken, err
}

