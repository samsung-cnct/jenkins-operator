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
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/types"
	"k8s.io/client-go/tools/record"

	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	jenkinsv1alpha1client "github.com/maratoid/jenkins-operator/pkg/client/clientset/versioned/typed/jenkins/v1alpha1"
	jenkinsv1alpha1informer "github.com/maratoid/jenkins-operator/pkg/client/informers/externalversions/jenkins/v1alpha1"
	jenkinsv1alpha1lister "github.com/maratoid/jenkins-operator/pkg/client/listers/jenkins/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/maratoid/jenkins-operator/pkg/inject/args"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"fmt"
	"github.com/golang/glog"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/eventhandlers"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/predicates"
	"time"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"bytes"
	"net/url"
	"text/template"
	"github.com/maratoid/jenkins-operator/pkg/bindata"
)

// EDIT THIS FILE
// This files was created by "kubebuilder create resource" for you to edit.
// Controller implementation logic for JenkinsJob resources goes here.

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
	MessageResourceExists = "Resource %q already exists and is not managed by JenkinsJob"
	// MessageResourceSynced is the message used for an Event fired when a JenkinsJob
	// is synced successfully
	MessageResourceSynced = "JenkinsJob synced successfully"
	// MessageResourceSynced is the message used for an Event fired when a JenkinsJob
	// is synced successfully
	ErrMessageResourceFailed = "JenkinsJob failed to run"
)

func (bc *JenkinsJobController) Reconcile(key types.ReconcileKey) error {
	jenkinsJob, err := bc.jenkinsjobclient.
		JenkinsJobs(key.Namespace).
		Get(key.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("jenkinsJob '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	jenkinsInstanceName := jenkinsJob.Spec.JenkinsInstance
	if jenkinsInstanceName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: JenkinsInstance must be specified", key.String()))
		return nil
	}

	jenkinsJobXml := jenkinsJob.Spec.JobXml
	jenkinsJobDsl := jenkinsJob.Spec.JobDsl
	if (jenkinsJobXml != "") && (jenkinsJobDsl != "") {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: Cannot specify both Job XML and Job DSL", key.String()))
		return nil
	}

	if (jenkinsJobXml == "") && (jenkinsJobDsl == "") {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: Must specify JobXml or JobDsl", key.String()))
		return nil
	}

	// Get the jenkins instance this plugin is intended for
	jenkinsInstance, err := bc.jenkinsinstanceLister.JenkinsInstances(jenkinsJob.Namespace).Get(jenkinsInstanceName)
	// If the resource doesn't exist, we'll re-queue
	if errors.IsNotFound(err) {
		glog.Errorf("JenkinsInstance %s referred to by JenkinsJob % does not exist.", jenkinsInstanceName, jenkinsJob.Name)
		return err
	}

	// make sure the jenkins instance is ready
	// Otherwise re-queue
	if jenkinsInstance.Status.Phase != "Ready" {
		glog.Errorf("JenkinsInstance %s referred to by JenkinsJob % is not ready.", jenkinsInstanceName, jenkinsJob.Name)
		return errors.NewBadRequest("JenkinsInstance not ready")
	}

	// Get the secret with the name specified in JenkinsInstance.status
	secret, err := bc.KubernetesInformers.Core().V1().Secrets().Lister().Secrets(jenkinsJob.Namespace).Get(jenkinsInstance.Status.SetupSecret)
	// If the resource doesn't exist, requeue
	if errors.IsNotFound(err) {
		glog.Errorf(
			"JenkinsInstance %s referred to by JenkinsJob % is not ready: secret %s does not exist",
			jenkinsInstanceName, jenkinsJob.Name, jenkinsInstance.Spec.AdminSecret)
		return err
	}

	// Create a kubernetes job that will use jenkins remote api to create the job from spec.
	// Get the deployment with the name specified in JenkinsInstance.spec
	job, err := bc.KubernetesInformers.Batch().V1().Jobs().Lister().Jobs(jenkinsJob.Namespace).Get(jenkinsJob.Name)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		job, err = bc.KubernetesClientSet.BatchV1().Jobs(jenkinsJob.Namespace).Create(newJobJob(jenkinsInstance, jenkinsJob, secret))
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
	if !metav1.IsControlledBy(job, jenkinsJob) {
		msg := fmt.Sprintf(MessageResourceExists, job.Name)
		bc.jenkinsjobrecorder.Event(jenkinsJob, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
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
		job, err = bc.KubernetesInformers.Batch().V1().Jobs().Lister().Jobs(jenkinsJob.Namespace).Get(jenkinsJob.Name)
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

	bc.jenkinsjobrecorder.Event(jenkinsJob, syncType, syncResult, syncResultMsg)

	return nil
}

// +kubebuilder:controller:group=jenkins,version=v1alpha1,kind=JenkinsJob,resource=jenkinsjobs
// +kubebuilder:informers:group=batch,version=v1,kind=Job
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch
// +kubebuilder:informers:group=core,version=v1,kind=Secret
type JenkinsJobController struct {
	args.InjectArgs
	jenkinsinstanceLister jenkinsv1alpha1lister.JenkinsInstanceLister
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
		InjectArgs: arguments,
		jenkinsjobLister: arguments.ControllerManager.GetInformerProvider(&jenkinsv1alpha1.JenkinsJob{}).(jenkinsv1alpha1informer.JenkinsJobInformer).Lister(),
		jenkinsinstanceLister: arguments.ControllerManager.GetInformerProvider(&jenkinsv1alpha1.JenkinsInstance{}).(jenkinsv1alpha1informer.JenkinsInstanceInformer).Lister(),
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

	// Set up an event handler for when Job resources change. This
	// handler will lookup the owner of the given Job, and if it is
	// owned by a JenkinsPlugin resource will enqueue that JenkinsPlugin resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Job resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	if err := gc.WatchControllerOf(&batchv1.Job{}, eventhandlers.Path{bc.LookupJenkinsJob},
		predicates.ResourceVersionChanged); err != nil {
		return gc, err
	}

	// Set up an event handler for when JenkinsInstance resources change.
	if err := gc.WatchControllerOf(&jenkinsv1alpha1.JenkinsInstance{}, eventhandlers.Path{bc.LookupJenkinsInstance},
		predicates.ResourceVersionChanged); err != nil {
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

// LookupJenkinsPlugin looks up a JenkinsPlugin from the lister
func (bc JenkinsJobController) LookupJenkinsJob(r types.ReconcileKey) (interface{}, error) {
	return bc.Informers.Jenkins().V1alpha1().JenkinsJobs().Lister().JenkinsJobs(r.Namespace).Get(r.Name)
}

// LookupJenkinsInstance looks up a JenkinsInstance from the lister
func (bc JenkinsJobController) LookupJenkinsInstance(r types.ReconcileKey) (interface{}, error) {
	jenkinsJob, err := bc.LookupJenkinsJob(r)
	if err != nil {
		return nil, err
	}

	return bc.Informers.Jenkins().V1alpha1().JenkinsInstances().Lister().JenkinsInstances(r.Namespace).Get(jenkinsJob.(*jenkinsv1alpha1.JenkinsJob).Spec.JenkinsInstance)
}

func newJobJob(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, jenkinsJob *jenkinsv1alpha1.JenkinsJob, setupSecret *corev1.Secret) *batchv1.Job {
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsJob.Name,
		"component": string(jenkinsJob.UID),
	}

	jenkinsJobXml := jenkinsJob.Spec.JobXml
	jenkinsJobDsl := jenkinsJob.Spec.JobDsl

	apiUrl, err := url.Parse(jenkinsInstance.Status.Api)
	if err != nil {
		glog.Errorf("Failed to parse url %s", jenkinsInstance.Status.Api)
		return nil
	}

	apiUrl.User = url.UserPassword(string(setupSecret.Data["user"][:]), string(setupSecret.Data["apiToken"][:]))

	type JobInfo struct {
		Api 			string
		JobName			string
		JobXml			string
		JobDsl          string
	}

	jobInfo := JobInfo{
		Api: apiUrl.String(),
		JobName: jenkinsJob.Name,
		JobXml: jenkinsJobXml,
		JobDsl: jenkinsJobDsl,
	}

	// load the correct template (xml or dsl)
	var jobConfig []byte
	if jenkinsJobXml != "" {
		jobConfig, err = bindata.Asset("job-scripts/install-xml-job.sh")
		if err != nil {
			glog.Errorf("Error locating binary asset: %s", err)
			return nil
		}
	} else if jenkinsJobDsl != "" {
		jobConfig, err = bindata.Asset("job-scripts/install-dsl-job.sh")
		if err != nil {
			glog.Errorf("Error locating binary asset: %s", err)
			return nil
		}
	}

	// parse the config template
	configTemplate, err := template.New("jenkins-job").Parse(string(jobConfig[:]))
	if err != nil {
		glog.Errorf("Failed to parse plugin config template: %s", err)
		return nil
	}

	var jobConfigParsed bytes.Buffer
	if err := configTemplate.Execute(&jobConfigParsed, jobInfo); err != nil {
		glog.Errorf("Failed to execute plugin config template: %s", err)
		return nil
	}

	var env []corev1.EnvVar
	env = append(env, corev1.EnvVar{
		Name:      "INSTALL_JOB",
		Value:     jobConfigParsed.String(),
	})

	var backoffLimit int32 = 3
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jenkinsJob.Name,
			Namespace: jenkinsJob.Namespace,
			Labels: labels,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(jenkinsJob, schema.GroupVersionKind{
					Group:   jenkinsv1alpha1.SchemeGroupVersion.Group,
					Version: jenkinsv1alpha1.SchemeGroupVersion.Version,
					Kind:    "JenkinsJob",
				}),
			},
		},

		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            jenkinsJob.Name,
							Image:           "java:latest",
							ImagePullPolicy: "IfNotPresent",
							Command: []string{
								"bash",
								"-c",
								"eval \"$INSTALL_JOB\"",
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