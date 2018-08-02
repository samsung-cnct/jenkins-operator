/*
Copyright 2018 Samsung CNCT.

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
	"context"
	"text/template"

	"bytes"
	"fmt"
	"github.com/golang/glog"
	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	"github.com/maratoid/jenkins-operator/pkg/bindata"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"time"
)

const (
	// SuccessSynced is used as part of the Event 'reason' when a JenkinsJob is synced
	SuccessSynced = "Synced"
	// SuccessSynced is used as part of the Event 'reason' when a JenkinsJob is synced
	ErrSynced = "Failed"
	// ErrResourceExists is used as part of the Event 'reason' when a JenkinsJob fails
	// to sync due to a resource of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a resource already existing
	MessageResourceExists = "Resource %q already exists and is not managed by JenkinsPlugin"
	// MessageResourceSynced is the message used for an Event fired when a JenkinsJob
	// is synced successfully
	MessageResourceSynced = "JenkinsPlugin synced successfully"
	// MessageResourceSynced is the message used for an Event fired when a JenkinsJob
	// is synced successfully
	ErrMessageResourceFailed = "JenkinsPlugin failed to run"
)

// Add creates a new JenkinsPlugin Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this jenkins.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileJenkinsPlugin{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetRecorder("JenkinsPluginController"),
		scheme:        mgr.GetScheme(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("jenkinsplugin-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to JenkinsPlugin
	err = c.Watch(&source.Kind{Type: &jenkinsv1alpha1.JenkinsPlugin{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch a Job created by JenkinsPlugin
	err = c.Watch(&source.Kind{Type: &batchv1.Job{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jenkinsv1alpha1.JenkinsPlugin{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileJenkinsPlugin{}

// ReconcileJenkinsPlugin reconciles a JenkinsPlugin object
type ReconcileJenkinsPlugin struct {
	client.Client
	record.EventRecorder
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a JenkinsPlugin object and makes changes based on the state read
// and what is in the JenkinsPlugin.Spec
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=jenkins.jenkinsoperator.maratoid.github.com,resources=jenkinsplugins,verbs=get;list;watch;create;update;patch;delete
func (bc *ReconcileJenkinsPlugin) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the JenkinsPlugin instance
	instance := &jenkinsv1alpha1.JenkinsPlugin{}
	err := bc.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			glog.Errorf("JenkinsPlugin '%s' in work queue no longer exists", request.String())
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	jenkinsInstanceName := instance.Spec.JenkinsInstance
	if jenkinsInstanceName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		glog.Errorf("%s: JenkinsInstance must be specified", request.String())
		return reconcile.Result{}, nil
	}

	jenkinsPluginName := instance.Spec.Name
	if jenkinsPluginName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		glog.Errorf("%s: Jenkins plugin name must be specified", request.String())
		return reconcile.Result{}, err
	}

	// Get the jenkins instance this plugin is intended for
	jenkinsInstance := &jenkinsv1alpha1.JenkinsInstance{}
	err = bc.Client.Get(context.TODO(), request.NamespacedName, jenkinsInstance)
	if errors.IsNotFound(err) {
		glog.Errorf("JenkinsInstance %s referred to by JenkinsPlugin % does not exist.", jenkinsInstanceName, instance.Name)
		return reconcile.Result{}, err
	}

	// make sure the jenkins instance is ready
	// Otherwise re-queue
	if jenkinsInstance.Status.Phase != "Ready" {
		glog.Errorf("JenkinsInstance %s referred to by JenkinsPlugin % is not ready.", jenkinsInstanceName, instance.Name)
		return reconcile.Result{}, fmt.Errorf("JenkinsInstance %s not ready", jenkinsInstanceName)
	}

	// Get the secret with the name specified in JenkinsInstance.status
	jenkinsSetupSecret := &corev1.Secret{}
	err = bc.Client.Get(
		context.TODO(),
		types.NewNamespacedNameFromString(fmt.Sprintf("%s%c%s", request.Namespace, types.Separator, jenkinsInstance.Status.SetupSecret)),
		jenkinsSetupSecret)
	// If the resource doesn't exist, requeue
	if errors.IsNotFound(err) {
		glog.Errorf(
			"JenkinsInstance %s referred to by JenkinsPlugin % is not ready: secret %s does not exist",
			jenkinsInstanceName, instance.Name, jenkinsInstance.Spec.AdminSecret)
		return reconcile.Result{}, err
	}

	// Create a kubernetes job that will use jenkins remote api to create the job from spec.
	// Get the deployment with the name specified in JenkinsInstance.spec
	job := &batchv1.Job{}
	err = bc.Client.Get(
		context.TODO(),
		types.NewNamespacedNameFromString(fmt.Sprintf("%s%c%s", request.Namespace, types.Separator, instance.Name)),
		job)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		job, err = bc.newJob(jenkinsInstance, instance, jenkinsSetupSecret)
		if err != nil {
			glog.Errorf("Error creating job object: %s", err)
			return reconcile.Result{}, err
		}
		err = bc.Client.Create(context.TODO(), job)
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating job: %s", err)
		return reconcile.Result{}, err
	}

	// If the Job is not controlled by this JenkinsInstance resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(job, instance) {
		msg := fmt.Sprintf(MessageResourceExists, job.Name)
		bc.EventRecorder.Event(instance, corev1.EventTypeWarning, ErrResourceExists, msg)
		return reconcile.Result{}, fmt.Errorf(msg)
	}

	// TODO Update the Job iff its observed Spec does
	// TODO: update status

	// TODO: this is a bad place for a wait
	// wait for job
	timeout := 0
	syncType := corev1.EventTypeWarning
	syncResult := ErrSynced
	syncResultMsg := ErrMessageResourceFailed
	for timeout < 2000 {
		err = bc.Client.Get(context.TODO(), request.NamespacedName, job)
		if err != nil {
			glog.Errorf("Error getting job: %s", err)
			break
		}

		if job.Status.Succeeded > 0 {
			syncType = corev1.EventTypeNormal
			syncResult = SuccessSynced
			syncResultMsg = MessageResourceSynced
			break
		}

		time.Sleep(5 * time.Second)
		timeout++
	}

	bc.EventRecorder.Event(instance, syncType, syncResult, syncResultMsg)

	return reconcile.Result{}, nil
}

func (bc *ReconcileJenkinsPlugin) newJob(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, jenkinsPlugin *jenkinsv1alpha1.JenkinsPlugin, setupSecret *corev1.Secret) (*batchv1.Job, error) {
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsPlugin.Name,
		"component":  string(jenkinsPlugin.UID),
	}

	pluginConfig, err := bindata.Asset("plugin-scripts/install_plugin.sh")
	if err != nil {
		return nil, err
	}

	type PluginInfo struct {
		Api           string
		PluginId      string
		PluginVersion string
		PluginConfig  string
	}

	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		glog.Errorf("Failed to parse url %s", jenkinsInstance.Status.Api)
		return nil, err
	}

	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))
	pluginInfo := PluginInfo{
		Api:           apiUrl.String(),
		PluginId:      jenkinsPlugin.Spec.PluginId,
		PluginVersion: jenkinsPlugin.Spec.PluginVersion,
		PluginConfig:  jenkinsPlugin.Spec.PluginConfig,
	}

	// parse the groovy config template
	configTemplate, err := template.New("jenkins-plugin").Parse(string(pluginConfig[:]))
	if err != nil {
		glog.Errorf("Failed to parse plugin config template: %s", err)
		return nil, err
	}

	var pluginConfigParsed bytes.Buffer
	if err := configTemplate.Execute(&pluginConfigParsed, pluginInfo); err != nil {
		glog.Errorf("Failed to execute plugin config template: %s", err)
		return nil, err
	}

	var env []corev1.EnvVar
	env = append(env, corev1.EnvVar{
		Name:  "INSTALL_PLUGIN",
		Value: pluginConfigParsed.String(),
	})

	var backoffLimit int32 = 3
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jenkinsPlugin.Spec.Name,
			Namespace: jenkinsPlugin.Namespace,
			Labels:    labels,
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
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
			BackoffLimit: &backoffLimit,
		},
	}

	err = controllerutil.SetControllerReference(jenkinsPlugin, job, bc.scheme)
	if err != nil {
		return nil, err
	}

	return job, nil
}
