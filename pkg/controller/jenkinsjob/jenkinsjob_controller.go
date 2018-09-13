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

package jenkinsjob

import (
	"context"
	"fmt"
	"github.com/golang/glog"
	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	"github.com/maratoid/jenkins-operator/pkg/controller/jenkinsinstance"
	"github.com/maratoid/jenkins-operator/pkg/util"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
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
	MessageResourceExists = "Resource %q already exists and is not managed by JenkinsJob"
	// MessageResourceSynced is the message used for an Event fired when a JenkinsJob
	// is synced successfully
	MessageResourceSynced = "JenkinsJob synced successfully"
	// MessageResourceSynced is the message used for an Event fired when a JenkinsJob
	// is synced successfully
	ErrMessageResourceFailed = "JenkinsJob failed to run"

	// Finalizer name
	Finalizer = "jenkinsjobs.jenkins.jenkinsoperator.maratoid.github.com"
)

const (
	JenkinsJobPhaseReady = "Ready"
)

// Add creates a new JenkinsJob Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this jenkins.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileJenkinsJob{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetRecorder("JenkinsJobController"),
		scheme:        mgr.GetScheme(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("jenkinsjob-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	watchPredicate := util.NewPredicate(viper.GetString("namespace"))

	// Watch for changes to JenkinsJob
	err = c.Watch(&source.Kind{Type: &jenkinsv1alpha1.JenkinsJob{}}, &handler.EnqueueRequestForObject{}, watchPredicate)
	if err != nil {
		return err
	}

	// Watch Secret resources not owned by the JenkinsJob
	// This is needed for re-loading login information from the pre-provided secret
	// When credential secret changes, Watch will list all JenkinsJob instances
	// and re-enqueue the keys for the ones that refer to that credential secret via their spec.
	err = c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {

				jenkinsJobs := &jenkinsv1alpha1.JenkinsJobList{}
				err = mgr.GetClient().List(
					context.TODO(),
					&client.ListOptions{LabelSelector: labels.Everything()},
					jenkinsJobs)
				if err != nil {
					glog.Errorf("Could not list JenkinsJobs")
					return []reconcile.Request{}
				}

				var keys []reconcile.Request
				for _, inst := range jenkinsJobs.Items {
					if inst.Spec.Credentials != nil {
						for _, credential := range inst.Spec.Credentials {
							if credential.Secret == a.Meta.GetName() {
								keys = append(keys, reconcile.Request{
									NamespacedName: types.NewNamespacedNameFromString(
										fmt.Sprintf("%s%c%s", inst.GetNamespace(), types.Separator, inst.GetName())),
								})
							}
						}
					}
				}

				// return found keys
				return keys
			}),
		}, watchPredicate)
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileJenkinsJob{}

// ReconcileJenkinsJob reconciles a JenkinsJob object
type ReconcileJenkinsJob struct {
	client.Client
	record.EventRecorder
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a JenkinsJob object and makes changes based on the state read
// and what is in the JenkinsJob.Spec
// +kubebuilder:rbac:groups=jenkins.jenkinsoperator.maratoid.github.com,resources=jenkinsjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
func (bc *ReconcileJenkinsJob) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the JenkinsJob instance
	instance := &jenkinsv1alpha1.JenkinsJob{}
	err := bc.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			glog.Errorf("JenkinsJob '%s' in work queue no longer exists", request.String())
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// add a finalizer if missing
	if instance.DeletionTimestamp == nil {
		updated, err := bc.addFinalizer(instance)
		if err != nil || updated {
			if err != nil {
				glog.Errorf("Error adding job %s finalizer: %s", instance.GetName(), err)
			}

			return reconcile.Result{}, err
		}
	} else {
		err = bc.finalizeJob(instance)
		if err != nil {
			glog.Errorf(
				"Could not finalize JenkinsJob %s: %s", instance.GetName(), err)
		}

		return reconcile.Result{}, err
	}

	// Get the jenkins instance this job is intended for
	jenkinsInstance, err := bc.getJenkinsInstance(instance)
	if err != nil {
		glog.Errorf("Could not get JenkinsInstance referred to by JenkinsJob %s: %s", instance.GetName(), err)
		return reconcile.Result{}, err
	}

	// Get the secret with the name specified in JenkinsInstance.status
	jenkinsSetupSecret, err := bc.getSetupSecret(jenkinsInstance)
	if err != nil {
		glog.Errorf("Could not get secret referred to by JenkinsInstance %s: %s", jenkinsInstance.GetName(), err)
		return reconcile.Result{}, err
	}

	// Create a jenkins job
	err = bc.newJob(jenkinsInstance, instance, jenkinsSetupSecret)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating jenkins job configuration: %s", err)
		return reconcile.Result{}, err
	}

	// TODO Update the Job iff its observed Spec does

	err = bc.updateJenkinsJobStatus(instance)
	if err != nil {
		glog.Errorf("Error updating job %s status: %s", instance.GetName(), err)
		return reconcile.Result{}, err
	}

	bc.EventRecorder.Event(instance, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return reconcile.Result{}, nil
}

// newJob creates a new JenkinsCI job configuration in the JenkinsCI instance pointed to by the JenkinsJob object
func (bc *ReconcileJenkinsJob) newJob(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, jenkinsJob *jenkinsv1alpha1.JenkinsJob, setupSecret *corev1.Secret) error {
	jenkinsJobXml := jenkinsJob.Spec.JobXml
	jenkinsJobDsl := jenkinsJob.Spec.JobDsl

	if jenkinsJobDsl != "" && jenkinsJobXml != "" {
		return fmt.Errorf("JenkinsJob %s can not have both JobXml and JobDSL defined", jenkinsJob.GetName())
	}

	if jenkinsJobDsl == "" && jenkinsJobXml == "" {
		return fmt.Errorf("JenkinsJob %s must have either JobXml or JobDSL defined", jenkinsJob.GetName())
	}

	for _, credentialSpec := range jenkinsJob.Spec.Credentials {
		credentialSecret := &corev1.Secret{}
		err := bc.Client.Get(
			context.TODO(),
			types.NewNamespacedNameFromString(
				fmt.Sprintf(
					"%s%c%s",
					jenkinsInstance.GetNamespace(),
					types.Separator,
					credentialSpec.Secret)),
			credentialSecret)

		// If the resource doesn't exist, requeue
		if errors.IsNotFound(err) {
			return fmt.Errorf("Credential secret %s for JenkinsJob %s does not exist", credentialSpec.Secret, jenkinsJob.GetName())
		}

		err = util.CreateJenkinsCredential(
			jenkinsInstance,
			setupSecret,
			credentialSecret,
			jenkinsJob.GetName(),
			credentialSpec.Credential,
			credentialSpec.CredentialType,
			credentialSpec.SecretData)
		if err != nil {
			return fmt.Errorf("Failed to create Jenkins credentials %s for JenkinsJob %s: %s", credentialSpec.Credential, jenkinsJob.GetName(), err)
		}
	}

	var err error
	if jenkinsJobDsl != "" {
		err = util.CreateJenkinsDSLJob(jenkinsInstance, setupSecret, jenkinsJob.Spec.JobDsl)
	} else {
		err = util.CreateJenkinsXMLJob(jenkinsInstance, setupSecret, jenkinsJob.GetName(), jenkinsJob.Spec.JobXml)
	}

	return err
}

// updateJenkinsJobStatus updates status fields of the JenkinsJob object and emits events
func (bc *ReconcileJenkinsJob) updateJenkinsJobStatus(jenkinsJob *jenkinsv1alpha1.JenkinsJob) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	jenkinsJobCopy := jenkinsJob.DeepCopy()

	// set sync phase
	jenkinsJobCopy.Status.Phase = JenkinsJobPhaseReady

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the JenkinsInstance resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	return bc.Client.Update(context.TODO(), jenkinsJobCopy)
}

// deleteFinalizer cleans up JenkinsJob finalizer string if present in the JenkinsJob object
func (bc *ReconcileJenkinsJob) deleteFinalizer(jenkinsJob *jenkinsv1alpha1.JenkinsJob) (bool, error) {
	if exists, _ := util.InArray(Finalizer, jenkinsJob.Finalizers); exists {
		jenkinsJobCopy := jenkinsJob.DeepCopy()
		jenkinsJobCopy.Finalizers = util.DeleteFinalizer(Finalizer, jenkinsJobCopy.Finalizers)
		return true, bc.Client.Update(context.TODO(), jenkinsJobCopy)
	}

	return false, nil
}

// addFinalizer adds a finalizer string to JenkinsJob object
func (bc *ReconcileJenkinsJob) addFinalizer(jenkinsJob *jenkinsv1alpha1.JenkinsJob) (bool, error) {
	if exists, _ := util.InArray(Finalizer, jenkinsJob.Finalizers); !exists {

		glog.Infof("Adding finalizer for %s", jenkinsJob.Name)
		// NEVER modify objects from the store. It's a read-only, local cache.
		// You can use DeepCopy() to make a deep copy of original object and modify this copy
		// Or create a copy manually for better performance
		jenkinsJobCopy := jenkinsJob.DeepCopy()
		jenkinsJobCopy.Finalizers = util.AddFinalizer(Finalizer, jenkinsJobCopy.Finalizers)
		return true, bc.Client.Update(context.TODO(), jenkinsJobCopy)
	}

	return false, nil
}

// getJenkinsInstance retrieves the JenkinsInstance pointed to by the JenkinsJob object
func (bc *ReconcileJenkinsJob) getJenkinsInstance(jenkinsJob *jenkinsv1alpha1.JenkinsJob) (*jenkinsv1alpha1.JenkinsInstance, error) {
	jenkinsInstanceName := jenkinsJob.Spec.JenkinsInstance
	if jenkinsInstanceName == "" {
		return nil, fmt.Errorf("JenkinsInstance must be specified for JenkinsJob %s", jenkinsJob.GetName())
	}

	// Get the jenkins instance this job is intended for
	jenkinsInstance := &jenkinsv1alpha1.JenkinsInstance{}
	err := bc.Client.Get(
		context.TODO(),
		types.NamespacedName{Name: jenkinsInstanceName, Namespace: jenkinsJob.Namespace},
		jenkinsInstance)
	if errors.IsNotFound(err) {
		return nil, err
	}

	// make sure the jenkins instance is ready
	// Otherwise re-queue
	if jenkinsInstance.Status.Phase != jenkinsinstance.JenkinsInstancePhaseReady {
		return nil, fmt.Errorf(
			"JenkinsInstance %s referred to by JenkinsJob %s is not ready",
			jenkinsInstanceName,
			jenkinsJob.GetName())
	}

	return jenkinsInstance, nil
}

// getSetupSecret retrieves the setup secret object of the JenkinsInstance pointed to by JenkinsJob object
func (bc *ReconcileJenkinsJob) getSetupSecret(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) (*corev1.Secret, error) {
	jenkinsSetupSecret := &corev1.Secret{}
	err := bc.Client.Get(
		context.TODO(),
		types.NewNamespacedNameFromString(
			fmt.Sprintf(
				"%s%c%s",
				jenkinsInstance.GetNamespace(),
				types.Separator,
				jenkinsInstance.Status.SetupSecret)),
		jenkinsSetupSecret)

	// If the resource doesn't exist, requeue
	if errors.IsNotFound(err) {
		return nil, fmt.Errorf("Setup secret %s for JenkinsInstance %s does not exist",
			jenkinsInstance.Status.SetupSecret, jenkinsInstance.GetName())
	}

	return jenkinsSetupSecret, err
}

// finalizeJob cleans up the JenkinsCI job configuration in the JenkinsCI server managed by the
// JenkinsInstance object, pointed to by the JenkinsJob object, when JenkinsJob object is deleted
func (bc *ReconcileJenkinsJob) finalizeJob(jenkinsJob *jenkinsv1alpha1.JenkinsJob) error {
	// if there are no finalizers, return
	if exists, _ := util.InArray(Finalizer, jenkinsJob.Finalizers); !exists {
		return nil
	}

	// Get the jenkins instance this plugin is intended for
	jenkinsInstance, err := bc.getJenkinsInstance(jenkinsJob)
	if err != nil {
		_, err = bc.deleteFinalizer(jenkinsJob)
		return err
	}

	// Get the secret with the name specified in JenkinsInstance.status
	setupSecret, err := bc.getSetupSecret(jenkinsInstance)
	if err != nil {
		_, err = bc.deleteFinalizer(jenkinsJob)
		return err
	}

	// delete the actual job config from jenkins
	err = util.DeleteJenkinsJob(jenkinsInstance, setupSecret, jenkinsJob.GetName())
	if err != nil {
		glog.Errorf("Failed to delete Jenkins Job config for JenkinsJob %s: %s", jenkinsJob.GetName(), err)
	}

	for _, credentialSpec := range jenkinsJob.Spec.Credentials {
		err = util.DeleteJenkinsCredential(
			jenkinsInstance,
			setupSecret,
			credentialSpec.Credential)
		if err != nil {
			glog.Errorf("Failed to delete Jenkins credentials %s for JenkinsJob %s: %s", credentialSpec.Credential, jenkinsJob.GetName(), err)
		}
	}

	_, err = bc.deleteFinalizer(jenkinsJob)
	return err
}
