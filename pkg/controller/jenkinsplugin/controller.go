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

package jenkinsplugin

import (
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/types"
	"k8s.io/client-go/tools/record"

	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	jenkinsv1alpha1client "github.com/maratoid/jenkins-operator/pkg/client/clientset/versioned/typed/jenkins/v1alpha1"
	jenkinsv1alpha1informer "github.com/maratoid/jenkins-operator/pkg/client/informers/externalversions/jenkins/v1alpha1"
	jenkinsv1alpha1lister "github.com/maratoid/jenkins-operator/pkg/client/listers/jenkins/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/maratoid/jenkins-operator/pkg/inject/args"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/eventhandlers"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/predicates"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"fmt"
	"github.com/golang/glog"
	"bytes"
	"github.com/maratoid/jenkins-operator/pkg/bindata"
	"text/template"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"time"
	"net/url"
)

// EDIT THIS FILE
// This files was created by "kubebuilder create resource" for you to edit.
// Controller implementation logic for JenkinsPlugin resources goes here.

const (
	// SuccessSynced is used as part of the Event 'reason' when a JenkinsPlugin is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a JenkinsPlugin fails
	// to sync due to a resource of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a resource already existing
	MessageResourceExists = "Resource %q already exists and is not managed by JenkinsPlugin"
	// MessageResourceSynced is the message used for an Event fired when a JenkinsPlugin
	// is synced successfully
	MessageResourceSynced = "JenkinsPlugin synced successfully"
)

func (bc *JenkinsPluginController) Reconcile(key types.ReconcileKey) error {
	jenkinsPlugin, err := bc.jenkinspluginclient.
		JenkinsPlugins(key.Namespace).
		Get(key.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("JenkinsPlugin '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	jenkinsPluginName := jenkinsPlugin.Spec.Name
	if jenkinsPluginName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: Jenkins plugin name must be specified", key.String()))
		return nil
	}

	jenkinsInstanceName := jenkinsPlugin.Spec.JenkinsInstance
	if jenkinsInstanceName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: JenkinsInstance must be specified", key.String()))
		return nil
	}

	// Get the jenkins instance this plugin is intended for
	jenkinsInstance, err := bc.jenkinsinstanceLister.JenkinsInstances(jenkinsPlugin.Namespace).Get(jenkinsInstanceName)
	// If the resource doesn't exist, we'll re-queue
	if errors.IsNotFound(err) {
		glog.Errorf("JenkinsInstance %s referred to by JenkinsPlugin % does not exist.", jenkinsInstanceName, jenkinsPluginName)
		return err
	}

	// make sure the jenkins instance is ready
	// Otherwise re-queue
	if jenkinsInstance.Status.Phase != "Ready" {
		glog.Errorf("JenkinsInstance %s referred to by JenkinsPlugin % is not ready.", jenkinsInstanceName, jenkinsPluginName)
		return errors.NewBadRequest("JenkinsInstance not ready")
	}

	// Get the secret with the name specified in JenkinsInstance.status
	secret, err := bc.KubernetesInformers.Core().V1().Secrets().Lister().Secrets(jenkinsPlugin.Namespace).Get(jenkinsInstance.Status.SetupSecret)
	// If the resource doesn't exist, requeue
	if errors.IsNotFound(err) {
		glog.Errorf(
			"JenkinsInstance %s referred to by JenkinsPlugin % is not ready: secret %s does not exist",
			jenkinsInstanceName, jenkinsPluginName, jenkinsInstance.Spec.AdminSecret)
		return err
	}

	// Create a kubernetes job that will use jenkins CLI to install a plugin into the found jenkins instance.
	// Get the deployment with the name specified in JenkinsInstance.spec
	job, err := bc.KubernetesInformers.Batch().V1().Jobs().Lister().Jobs(jenkinsPlugin.Namespace).Get(jenkinsPluginName)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		job, err = bc.KubernetesClientSet.BatchV1().Jobs(jenkinsPlugin.Namespace).Create(newPluginJob(jenkinsInstance, jenkinsPlugin, secret))
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating job: %s", err)
		return err
	}

	// If the Job is not controlled by this JenkinsInstance resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(job, jenkinsPlugin) {
		msg := fmt.Sprintf(MessageResourceExists, job.Name)
		bc.jenkinspluginrecorder.Event(jenkinsPlugin, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// wait for job
	timeout := 0
	for timeout < 2000 {
		if job.Status.Succeeded > 0 {
			break
		}

		time.Sleep(5 * time.Second)
		timeout++
	}

	// TODO Update the Job iff its observed Spec does
	// TODO: consider other changes besides replicas number
	//if jenkinsInstance.Spec.Replicas != nil && *jenkinsInstance.Spec.Replicas != *deployment.Spec.Replicas {
	//	glog.V(4).Infof("jenkinsInstance %s replicas: %d, deployment replicas: %d", baseName, *jenkinsInstance.Spec.Replicas, *deployment.Spec.Replicas)
	//	deployment, err = bc.KubernetesClientSet.AppsV1().Deployments(jenkinsInstance.Namespace).Update(newDeployment(jenkinsInstance))
	//}

	// TODO: Update the Service iff its observed Spec does

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// TODO:
	// Finally, we update the status block of the JenkinsPlugin resource to reflect the
	// current state of the world
	//err = bc.updateJenkinsPluginStatus(jenkinsInstance, deployment, service)
	//if err != nil {
	//	return err
	//}

	bc.jenkinspluginrecorder.Event(jenkinsPlugin, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

// +kubebuilder:controller:group=jenkins,version=v1alpha1,kind=JenkinsPlugin,resource=jenkinsplugins
// +kubebuilder:informers:group=batch,version=v1,kind=Job
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:informers:group=core,version=v1,kind=Secret
type JenkinsPluginController struct {
	args.InjectArgs

	// INSERT ADDITIONAL FIELDS HERE
	jenkinsinstanceLister jenkinsv1alpha1lister.JenkinsInstanceLister
	jenkinspluginLister jenkinsv1alpha1lister.JenkinsPluginLister
	jenkinspluginclient jenkinsv1alpha1client.JenkinsV1alpha1Interface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	jenkinspluginrecorder record.EventRecorder
}

// ProvideController provides a controller that will be run at startup.  Kubebuilder will use codegeneration
// to automatically register this controller in the inject package
func ProvideController(arguments args.InjectArgs) (*controller.GenericController, error) {
	// INSERT INITIALIZATIONS FOR ADDITIONAL FIELDS HERE
	bc := &JenkinsPluginController{
		InjectArgs: arguments,
		jenkinsinstanceLister: arguments.ControllerManager.GetInformerProvider(&jenkinsv1alpha1.JenkinsInstance{}).(jenkinsv1alpha1informer.JenkinsInstanceInformer).Lister(),
		jenkinspluginLister: arguments.ControllerManager.GetInformerProvider(&jenkinsv1alpha1.JenkinsPlugin{}).(jenkinsv1alpha1informer.JenkinsPluginInformer).Lister(),
		jenkinspluginclient:   arguments.Clientset.JenkinsV1alpha1(),
		jenkinspluginrecorder: arguments.CreateRecorder("JenkinsPluginController"),
	}

	// Create a new controller that will call JenkinsPluginController.Reconcile on changes to JenkinsPlugins
	gc := &controller.GenericController{
		Name:             "JenkinsPluginController",
		Reconcile:        bc.Reconcile,
		InformerRegistry: arguments.ControllerManager,
	}
	if err := gc.Watch(&jenkinsv1alpha1.JenkinsPlugin{}); err != nil {
		return gc, err
	}

	// Set up an event handler for when Job resources change. This
	// handler will lookup the owner of the given Job, and if it is
	// owned by a JenkinsPlugin resource will enqueue that JenkinsPlugin resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Job resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	if err := gc.WatchControllerOf(&batchv1.Job{}, eventhandlers.Path{bc.LookupJenkinsPlugin},
		predicates.ResourceVersionChanged); err != nil {
		return gc, err
	}

	// Set up an event handler for when JenkinsInstance resources change. This
	// handler will lookup the the given JenkinsInstance
	// owned by a JenkinsPlugin resource will enqueue that JenkinsInstance resource for
	if err := gc.WatchControllerOf(&jenkinsv1alpha1.JenkinsInstance{}, eventhandlers.Path{bc.LookupJenkinsInstance},
		predicates.ResourceVersionChanged); err != nil {
		return gc, err
	}

	// IMPORTANT:
	// To watch additional resource types - such as those created by your controller - add gc.Watch* function calls here
	// Watch function calls will transform each object event into a JenkinsPlugin Key to be reconciled by the controller.
	//
	// **********
	// For any new Watched types, you MUST add the appropriate // +kubebuilder:informer and // +kubebuilder:rbac
	// annotations to the JenkinsPluginController and run "kubebuilder generate.
	// This will generate the code to start the informers and create the RBAC rules needed for running in a cluster.
	// See:
	// https://godoc.org/github.com/kubernetes-sigs/kubebuilder/pkg/gen/controller#example-package
	// **********

	return gc, nil
}

// LookupJenkinsPlugin looks up a JenkinsPlugin from the lister
func (c JenkinsPluginController) LookupJenkinsPlugin(r types.ReconcileKey) (interface{}, error) {
	return c.Informers.Jenkins().V1alpha1().JenkinsPlugins().Lister().JenkinsPlugins(r.Namespace).Get(r.Name)
}

// LookupJenkinsInstance looks up a JenkinsInstance from the lister
func (c JenkinsPluginController) LookupJenkinsInstance(r types.ReconcileKey) (interface{}, error) {
	return c.Informers.Jenkins().V1alpha1().JenkinsInstances().Lister().JenkinsInstances(r.Namespace).Get(r.Name)
}

func newPluginJob(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, jenkinsPlugin *jenkinsv1alpha1.JenkinsPlugin, setupSecret *corev1.Secret) *batchv1.Job {
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsPlugin.Name,
		"component": string(jenkinsPlugin.UID),
	}

	pluginConfig, err := bindata.Asset("plugin-scripts/install_plugin.sh")
	if err != nil {
		glog.Errorf("Error locating binary asset: %s", err)
		return nil
	}

	type PluginInfo struct {
		Api 			string
		PluginId		string
		PluginVersion	string
	}

	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		glog.Errorf("Failed to parse url %s", jenkinsInstance.Status.Api)
		return nil
	}

	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	pluginInfo := PluginInfo{
		Api: apiUrl.String(),
		PluginId: jenkinsPlugin.Spec.PluginId,
		PluginVersion: jenkinsPlugin.Spec.PluginVersion,
	}

	// parse the groovy config template
	configTemplate, err := template.New("jenkins-plugin").Parse(string(pluginConfig[:]))
	if err != nil {
		glog.Errorf("Failed to parse plugin config template: %s", err)
		return nil
	}

	var pluginConfigParsed bytes.Buffer
	if err := configTemplate.Execute(&pluginConfigParsed, pluginInfo); err != nil {
		glog.Errorf("Failed to execute plugin config template: %s", err)
		return nil
	}

	var env []corev1.EnvVar
	env = append(env, corev1.EnvVar{
		Name:      "INSTALL_PLUGIN",
		Value:     pluginConfigParsed.String(),
	})

	var backoffLimit int32 = 3
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jenkinsPlugin.Spec.Name,
			Namespace: jenkinsPlugin.Namespace,
			Labels: labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(jenkinsPlugin, schema.GroupVersionKind{
					Group:   jenkinsv1alpha1.SchemeGroupVersion.Group,
					Version: jenkinsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "JenkinsPlugin",
				}),
			},
		},

		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            jenkinsPlugin.Spec.Name,
							Image:           "java:latest",
							ImagePullPolicy: "IfNotPresent",
							Command: []string{
								"bash",
								"-c",
								"eval \"$INSTALL_PLUGIN\"",
							},
							Env: env,
						},
					},
					RestartPolicy:      corev1.RestartPolicyOnFailure,
				},
			},
			BackoffLimit: &backoffLimit,
		},
	}
}