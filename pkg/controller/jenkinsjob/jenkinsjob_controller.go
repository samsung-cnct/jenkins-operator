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

func (bc *ReconcileJenkinsJob) getJenkinsJob(name types.NamespacedName) (*jenkinsv1alpha1.JenkinsJob, error) {
	jenkinsJob := &jenkinsv1alpha1.JenkinsJob{}
	err := bc.Client.Get(context.TODO(), name, jenkinsJob)
	return jenkinsJob, err
}

func (bc *ReconcileJenkinsJob) getJenkinsInstance(name types.NamespacedName) (*jenkinsv1alpha1.JenkinsInstance, error) {
	jenkinsJob, err := bc.getJenkinsJob(name)
	if err != nil {
		return nil, fmt.Errorf("could not get JenkinsJob %s: %v", name.String(), err)
	}

	jenkinsInstance := &jenkinsv1alpha1.JenkinsInstance{}
	err = bc.Client.Get(
		context.TODO(),
		types.NamespacedName{Namespace: name.Namespace, Name: jenkinsJob.Spec.JenkinsInstance},
		jenkinsInstance)
	return jenkinsInstance, err
}

func (bc *ReconcileJenkinsJob) getSetupSecret(instanceName types.NamespacedName) (*corev1.Secret, error) {
	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, err
	}

	setupSecret := &corev1.Secret{}
	err = bc.Client.Get(
		context.TODO(),
		types.NamespacedName{Namespace: instanceName.Namespace, Name: jenkinsInstance.Status.SetupSecret},
		setupSecret)
	return setupSecret, err
}

func (bc *ReconcileJenkinsJob) getService(instanceName types.NamespacedName) (*corev1.Service, error) {
	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, err
	}

	serviceName := jenkinsInstance.GetName()
	if jenkinsInstance.Spec.Service != nil && jenkinsInstance.Spec.Service.Name != "" {
		serviceName = jenkinsInstance.Spec.Service.Name
	}

	service := &corev1.Service{}
	err = bc.Client.Get(
		context.TODO(),
		types.NamespacedName{Namespace: instanceName.Namespace, Name: serviceName},
		service)

	return service, err
}

// Reconcile reads that state of the cluster for a JenkinsJob object and makes changes based on the state read
// and what is in the JenkinsJob.Spec
// +kubebuilder:rbac:groups=jenkins.jenkinsoperator.maratoid.github.com,resources=jenkinsjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
func (bc *ReconcileJenkinsJob) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the JenkinsJob instance
	jenkinsJob, err := bc.getJenkinsJob(request.NamespacedName)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			glog.Errorf("JenkinsJob '%s' in work queue no longer exists", request.NamespacedName.String())
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// add a finalizer if missing
	if jenkinsJob.DeletionTimestamp == nil {
		updated, err := bc.addFinalizer(request.NamespacedName)
		if err != nil || updated {
			if err != nil {
				glog.Errorf("error adding job %s finalizer: %s", jenkinsJob.GetName(), err)
			}

			return reconcile.Result{}, err
		}
	} else {
		err = bc.finalizeJob(request.NamespacedName)
		if err != nil {
			glog.Errorf(
				"could not finalize JenkinsJob %s: %s", jenkinsJob.GetName(), err)
		}

		return reconcile.Result{}, err
	}

	// Create a jenkins job
	err = bc.newJob(request.NamespacedName)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("error creating jenkins job configuration: %s", err)
		return reconcile.Result{}, err
	}

	// TODO Update the Job iff its observed Spec does

	err = bc.updateJenkinsJobStatus(request.NamespacedName)
	if err != nil {
		glog.Errorf("error updating job %s status: %s", jenkinsJob.GetName(), err)
		return reconcile.Result{}, err
	}

	bc.EventRecorder.Event(jenkinsJob, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return reconcile.Result{}, nil
}

// newJob creates a new Jenkins job configuration in the Jenkins instance pointed to by the JenkinsJob object
func (bc *ReconcileJenkinsJob) newJob(instanceName types.NamespacedName) error {
	jenkinsJob, err := bc.getJenkinsJob(instanceName)
	if err != nil {
		return err
	}

	jenkinsJobXml := jenkinsJob.Spec.JobXml
	jenkinsJobDsl := jenkinsJob.Spec.JobDsl

	if jenkinsJobDsl != "" && jenkinsJobXml != "" {
		return fmt.Errorf("JenkinsJob %s can not have both JobXml and JobDSL defined", jenkinsJob.GetName())
	}

	if jenkinsJobDsl == "" && jenkinsJobXml == "" {
		return fmt.Errorf("JenkinsJob %s must have either JobXml or JobDSL defined", jenkinsJob.GetName())
	}

	// Get the jenkins instance this job is intended for
	jenkinsInstance, err := bc.checkJenkinsInstance(instanceName)
	if err != nil {
		return err
	}

	// Get the secret with the name specified in JenkinsInstance.status
	setupSecret, err := bc.getSetupSecret(instanceName)
	if err != nil {
		return err
	}

	// get the service info
	service, err := bc.getService(instanceName)

	for _, credentialSpec := range jenkinsJob.Spec.Credentials {
		credentialSecret := &corev1.Secret{}
		err := bc.Client.Get(
			context.TODO(),
			types.NamespacedName{Name: credentialSpec.Secret, Namespace: jenkinsInstance.GetNamespace()},
			credentialSecret)

		// If the resource doesn't exist, requeue
		if errors.IsNotFound(err) {
			return fmt.Errorf(
				"credential secret %s for JenkinsJob %s does not exist",
				credentialSpec.Secret,
				jenkinsJob.GetName())
		}

		err = util.CreateJenkinsCredential(
			service,
			setupSecret,
			credentialSecret,
			jenkinsJob.GetName(),
			credentialSpec.Credential,
			credentialSpec.CredentialType,
			credentialSpec.SecretData,
			jenkinsinstance.JenkinsMasterPort)
		if err != nil {
			return fmt.Errorf(
				"failed to create Jenkins credentials %s for JenkinsJob %s: %s",
				credentialSpec.Credential, jenkinsJob.GetName(), err)
		}
	}

	if jenkinsJobDsl != "" {
		err = util.CreateJenkinsDSLJob(
			service, setupSecret, jenkinsJob.Spec.JobDsl, jenkinsinstance.JenkinsMasterPort)
	} else {
		err = util.CreateJenkinsXMLJob(
			service, setupSecret, jenkinsJob.GetName(), jenkinsJob.Spec.JobXml, jenkinsinstance.JenkinsMasterPort)
	}

	return err
}

// updateJenkinsJobStatus updates status fields of the JenkinsJob object and emits events
func (bc *ReconcileJenkinsJob) updateJenkinsJobStatus(instanceName types.NamespacedName) error {
	jenkinsJob, err := bc.getJenkinsJob(instanceName)
	if err != nil {
		return err
	}
	jenkinsJobCopy := jenkinsJob.DeepCopy()

	// set sync phase
	jenkinsJobCopy.Status.Phase = JenkinsJobPhaseReady
	return bc.Client.Update(context.TODO(), jenkinsJobCopy)
}

// deleteFinalizer cleans up JenkinsJob finalizer string if present in the JenkinsJob object
func (bc *ReconcileJenkinsJob) deleteFinalizer(instanceName types.NamespacedName) (bool, error) {
	jenkinsJob, err := bc.getJenkinsJob(instanceName)
	if err != nil {
		return false, err
	}

	if exists, _ := util.InArray(Finalizer, jenkinsJob.Finalizers); exists {
		jenkinsJobCopy := jenkinsJob.DeepCopy()
		jenkinsJobCopy.Finalizers = util.DeleteFinalizer(Finalizer, jenkinsJobCopy.Finalizers)
		return true, bc.Client.Update(context.TODO(), jenkinsJobCopy)
	}

	return false, nil
}

// addFinalizer adds a finalizer string to JenkinsJob object
func (bc *ReconcileJenkinsJob) addFinalizer(instanceName types.NamespacedName) (bool, error) {
	jenkinsJob, err := bc.getJenkinsJob(instanceName)
	if err != nil {
		return false, err
	}

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
func (bc *ReconcileJenkinsJob) checkJenkinsInstance(instanceName types.NamespacedName) (*jenkinsv1alpha1.JenkinsInstance, error) {
	jenkinsJob, err := bc.getJenkinsJob(instanceName)
	if err != nil {
		return nil, err
	}

	jenkinsInstanceName := jenkinsJob.Spec.JenkinsInstance
	if jenkinsInstanceName == "" {
		return nil, fmt.Errorf("JenkinsInstance must be specified for JenkinsJob %s", jenkinsJob.GetName())
	}

	// Get the jenkins instance this job is intended for
	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
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

// finalizeJob cleans up the Jenkins job configuration in the Jenkins server managed by the
// JenkinsInstance object, pointed to by the JenkinsJob object, when JenkinsJob object is deleted
func (bc *ReconcileJenkinsJob) finalizeJob(instanceName types.NamespacedName) error {
	jenkinsJob, err := bc.getJenkinsJob(instanceName)
	if err != nil {
		return err
	}

	// if there are no finalizers, return
	if exists, _ := util.InArray(Finalizer, jenkinsJob.Finalizers); !exists {
		return nil
	}

	service, err := bc.getService(types.NamespacedName{
		Namespace: jenkinsJob.Namespace, Name: jenkinsJob.Spec.JenkinsInstance,
	})
	if err != nil {
		_, err = bc.deleteFinalizer(instanceName)
		return err
	}

	// Get the secret with the name specified in JenkinsInstance.status
	setupSecret, err := bc.getSetupSecret(types.NamespacedName{
		Namespace: jenkinsJob.Namespace, Name: jenkinsJob.Spec.JenkinsInstance,
	})
	if err != nil {
		_, err = bc.deleteFinalizer(instanceName)
		return err
	}

	// delete the actual job config from jenkins
	err = util.DeleteJenkinsJob(service, setupSecret, jenkinsJob.GetName(), jenkinsinstance.JenkinsMasterPort)
	if err != nil {
		glog.Errorf("Failed to delete Jenkins Job config for JenkinsJob %s: %s", jenkinsJob.GetName(), err)
	}

	for _, credentialSpec := range jenkinsJob.Spec.Credentials {
		err = util.DeleteJenkinsCredential(
			service,
			setupSecret,
			credentialSpec.Credential,
			jenkinsinstance.JenkinsMasterPort)
		if err != nil {
			glog.Errorf("Failed to delete Jenkins credentials %s for JenkinsJob %s: %s", credentialSpec.Credential, jenkinsJob.GetName(), err)
		}
	}

	_, err = bc.deleteFinalizer(instanceName)
	return err
}
