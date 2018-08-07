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

package jenkinsinstance

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"github.com/golang/glog"
	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	"github.com/maratoid/jenkins-operator/pkg/bindata"
	"github.com/maratoid/jenkins-operator/pkg/util"
	"gopkg.in/matryer/try.v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
	"text/template"
	"time"
)

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

// Add creates a new JenkinsInstance Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
// USER ACTION REQUIRED: update cmd/manager/main.go to call this jenkins.Add(mgr) to install this Controller
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileJenkinsInstance{
		Client:        mgr.GetClient(),
		EventRecorder: mgr.GetRecorder("JenkinsInstanceController"),
		scheme:        mgr.GetScheme(),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("jenkinsinstance-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to JenkinsInstance
	err = c.Watch(&source.Kind{Type: &jenkinsv1alpha1.JenkinsInstance{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch a Deployment created by JenkinsInstance - change this for objects you create
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jenkinsv1alpha1.JenkinsInstance{},
	})
	if err != nil {
		return err
	}

	// Watch a Secret created by JenkinsInstance - change this for objects you create
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jenkinsv1alpha1.JenkinsInstance{},
	})
	if err != nil {
		return err
	}

	// Watch a Service created by JenkinsInstance - change this for objects you create
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jenkinsv1alpha1.JenkinsInstance{},
	})
	if err != nil {
		return err
	}

	// Watch Secret resources not owned by the JenkinsInstance change
	// This is needed for re-loading login information from the pre-provided secret
	// When admin login secret changes, Watch will list all JenkinsInstances
	// and re-enqueue the keys for the ones that refer to that admin login secret via their spec.
	err = c.Watch(
		&source.Kind{Type: &corev1.Secret{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {

				jenkinsInstances := &jenkinsv1alpha1.JenkinsInstanceList{}
				err = mgr.GetClient().List(
					context.TODO(),
					&client.ListOptions{LabelSelector: labels.Everything()},
					jenkinsInstances)
				if err != nil {
					glog.Errorf("Could not list JenkinsInstances")
					return []reconcile.Request{}
				}

				var keys []reconcile.Request
				for _, inst := range jenkinsInstances.Items {
					if inst.Spec.AdminSecret == a.Meta.GetName() {
						keys = append(keys, reconcile.Request{
							NamespacedName: types.NewNamespacedNameFromString(
								fmt.Sprintf("%s%c%s", inst.GetNamespace(), types.Separator, inst.Name)),
						})
					}
				}

				// return found keys
				return keys
			}),
		})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileJenkinsInstance{}

// ReconcileJenkinsInstance reconciles a JenkinsInstance object
type ReconcileJenkinsInstance struct {
	client.Client
	record.EventRecorder
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a JenkinsInstance object and makes changes based on the state read
// and what is in the JenkinsInstance.Spec
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=jenkins.jenkinsoperator.maratoid.github.com,resources=jenkinsinstances,verbs=get;list;watch;create;update;patch;delete
func (bc *ReconcileJenkinsInstance) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	jenkinsInstance := &jenkinsv1alpha1.JenkinsInstance{}
	err := bc.Client.Get(context.TODO(), request.NamespacedName, jenkinsInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Errorf("JenkinsInstance key '%s' in work queue no longer exists", request.String())
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	baseName := jenkinsInstance.GetName()
	if baseName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		glog.Errorf("%s: deployment name must be specified", request.String())
		return reconcile.Result{}, nil
	}

	adminSecretName := jenkinsInstance.Spec.AdminSecret
	if adminSecretName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		glog.Errorf("%s: AdminSecret name must be specified", request.String())
		return reconcile.Result{}, nil
	}
	adminSecretKey := types.NewNamespacedNameFromString(
		fmt.Sprintf("%s%c%s", request.Namespace, types.Separator, adminSecretName))

	adminSecret := &corev1.Secret{}
	err = bc.Client.Get(context.TODO(), adminSecretKey, adminSecret)
	if errors.IsNotFound(err) {
		glog.Errorf("AdminSecret %s does not exist yet: %v", adminSecretKey.String(), err)
		return reconcile.Result{}, err
	}

	// Get the setup utility secret managed by this controller
	setupSecret := &corev1.Secret{}
	err = bc.Client.Get(context.TODO(), request.NamespacedName, setupSecret)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		setupSecret, err = bc.newSetupSecret(jenkinsInstance, adminSecret)
		if err != nil {
			glog.Errorf("Error creating setup secret object: %s", err)
			return reconcile.Result{}, err
		}
		err = bc.Client.Create(context.TODO(), setupSecret)
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating setup secret: %s", err)
		return reconcile.Result{}, err
	}

	// If the Secret is not controlled by this JenkinsInstance resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(setupSecret, jenkinsInstance) {
		msg := fmt.Sprintf(MessageResourceExists, setupSecret.GetName())
		bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
		return reconcile.Result{}, fmt.Errorf(msg)
	}

	// Get the deployment with the name specified in JenkinsInstance.spec
	deployment := &appsv1.Deployment{}
	err = bc.Client.Get(context.TODO(), request.NamespacedName, deployment)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		deployment, err = bc.newDeployment(jenkinsInstance)
		if err != nil {
			glog.Errorf("Error creating deployment object: %s", err)
			return reconcile.Result{}, err
		}
		err = bc.Client.Create(context.TODO(), deployment)
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating deployment: %s", err)
		return reconcile.Result{}, err
	}

	// If the Deployment is not controlled by this JenkinsInstance resource, we should log
	// a warning to the event recorder and return
	if !metav1.IsControlledBy(deployment, jenkinsInstance) {
		msg := fmt.Sprintf(MessageResourceExists, deployment.GetName())
		bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
		return reconcile.Result{}, fmt.Errorf(msg)
	}

	// Get the service with the name specified in JenkinsInstance.spec
	service := &corev1.Service{}
	err = bc.Client.Get(context.TODO(), request.NamespacedName, service)
	// If the resource doesn't exist, we'll create it
	if errors.IsNotFound(err) {
		service, err = bc.newService(jenkinsInstance)
		if err != nil {
			glog.Errorf("Error creating service object: %s", err)
			return reconcile.Result{}, err
		}
		err = bc.Client.Create(context.TODO(), service)
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating service: %s", err)
		return reconcile.Result{}, err
	}

	// If the Service is not controlled by this JenkinsInstance resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(service, jenkinsInstance) {
		msg := fmt.Sprintf(MessageResourceExists, service.GetName())
		bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
		return reconcile.Result{}, fmt.Errorf(msg)
	}

	// Update the Deployment iff its observed Spec does
	// TODO: consider other changes besides replicas number
	if jenkinsInstance.Spec.Replicas != nil && *jenkinsInstance.Spec.Replicas != *deployment.Spec.Replicas {
		glog.V(4).Infof(
			"jenkinsInstance %s replicas: %d, deployment replicas: %d",
			baseName, *jenkinsInstance.Spec.Replicas,
			*deployment.Spec.Replicas)
		deployment, err = bc.newDeployment(jenkinsInstance)
		if err != nil {
			glog.Errorf("Error creating deployment object: %s", err)
			return reconcile.Result{}, err
		}
		err = bc.Client.Update(context.TODO(), deployment)
	}

	// TODO: Update the Service iff its observed Spec does

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating deployment: %s", err)
		return reconcile.Result{}, err
	}

	// Finally, we update the status block of the JenkinsInstance resource to reflect the
	// current state of the world
	err = bc.updateJenkinsInstanceStatus(jenkinsInstance, deployment, service, adminSecret, setupSecret)
	if err != nil {
		return reconcile.Result{}, err
	}

	bc.Event(jenkinsInstance, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return reconcile.Result{}, nil
}

// update status fields of the jenkins instance object and emit events
func (bc *ReconcileJenkinsInstance) updateJenkinsInstanceStatus(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, deployment *appsv1.Deployment, service *corev1.Service, adminSecret *corev1.Secret, setupSecret *corev1.Secret) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	jenkinsInstanceCopy := jenkinsInstance.DeepCopy()

	var apiToken string
	err := try.Do(func(attempt int) (bool, error) {

		// use admin secret data to get the user configuration page and parse out an api token
		var err error
		apiToken, err = util.GetJenkinsApiToken(jenkinsInstance, service, adminSecret)
		if err != nil {
			glog.Errorf("Error: %v, retrying", err)
			time.Sleep(10 * time.Second) // wait 10 seconds
		}

		return attempt < 5, err
	})

	if err != nil {
		glog.Errorf("Error: %v", err)
		return err
	}

	setupSecretCopy := setupSecret.DeepCopy()
	setupSecretCopy.Data["apiToken"] = []byte(apiToken)
	err = bc.Client.Update(context.TODO(), setupSecretCopy)
	if err != nil {
		glog.Errorf("Error updating secret %s: %v", setupSecret.GetName(), err)
		return err
	}

	// TODO: update other status fields
	api, err := util.GetServiceEndpoint(service, "", 8080)
	if err != nil {
		return err
	}

	jenkinsInstanceCopy.Status.Api = api
	jenkinsInstanceCopy.Status.SetupSecret = setupSecretCopy.GetName()
	jenkinsInstanceCopy.Status.Phase = "Ready"

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the JenkinsInstanceC resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	err = bc.Client.Update(context.TODO(), jenkinsInstanceCopy)
	return err
}

// newSetupSecret creates an admin password secret for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newSetupSecret(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance, adminSecret *corev1.Secret) (*corev1.Secret, error) {
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	adminUserConfig, err := bindata.Asset("init-groovy/0-jenkins-config.groovy")
	if err != nil {
		return nil, err
	}

	type JenkinsInfo struct {
		User       string
		Password   string
		Url        string
		AdminEmail string
		AgentPort  int32
		Executors  int32
	}

	// decode Admin secret strings
	adminUser := string(adminSecret.Data["user"][:])
	adminPassword := string(adminSecret.Data["pass"][:])
	if err != nil {
		return nil, err
	}

	jenkinsInfo := JenkinsInfo{
		User:       string(adminUser[:]),
		Password:   string(adminPassword[:]),
		Url:        jenkinsInstance.Spec.Location,
		AdminEmail: jenkinsInstance.Spec.AdminEmail,
		AgentPort:  jenkinsInstance.Spec.AgentPort,
		Executors:  jenkinsInstance.Spec.Executors,
	}

	// parse the plugin array
	requiredPlugin, err := bindata.Asset("environment/required-plugins")
	if err != nil {
		return nil, err
	}
	plugins := jenkinsInstance.Spec.Plugins
	var pluginList []string

	// add required plugins first
	scanner := bufio.NewScanner(strings.NewReader(string(requiredPlugin[:])))
	for scanner.Scan() {
		pluginList = append(pluginList, scanner.Text())
	}

	// add user plugins next
	for _, plugin := range plugins {
		pluginInfo := fmt.Sprintf("%s:%s", plugin.Id, plugin.Version)
		pluginList = append(pluginList, pluginInfo)
	}

	// TODO: remove duplicate plugin ids

	// parse the groovy config template
	configTemplate, err := template.New("jenkins-config").Parse(string(adminUserConfig[:]))
	if err != nil {
		return nil, err
	}

	var jenkinsConfigParsed bytes.Buffer
	if err := configTemplate.Execute(&jenkinsConfigParsed, jenkinsInfo); err != nil {
		return nil, err
	}

	// load seed job dsl from bindata
	seedDsl, err := bindata.Asset("jobdsl/seed-job-dsl")
	if err != nil {
		return nil, err
	}

	// add things to the string data
	stringData := map[string]string{
		"0-jenkins-config.groovy": jenkinsConfigParsed.String(),
		"1-user-config.groovy":    jenkinsInstance.Spec.Config,
		"plugins.txt":             strings.Join(pluginList, "\n"),
		"seed-job-dsl":            string(seedDsl[:]),
		"user":                    adminUser,
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jenkinsInstance.GetName(),
			Namespace: jenkinsInstance.GetNamespace(),
			Labels:    labels,
		},
		StringData: stringData,
		Type:       corev1.SecretTypeOpaque,
	}

	err = controllerutil.SetControllerReference(jenkinsInstance, secret, bc.scheme)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

// newService creates a new Service for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newService(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) (*corev1.Service, error) {
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jenkinsInstance.GetName(),
			Namespace: jenkinsInstance.GetNamespace(),
			Labels:    labels,
		},

		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:     "master",
					Protocol: "TCP",
					Port:     jenkinsInstance.Spec.MasterPort,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: jenkinsInstance.Spec.MasterPort,
					},
				},
				{
					Name:     "agent",
					Protocol: "TCP",
					Port:     jenkinsInstance.Spec.AgentPort,
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

	err := controllerutil.SetControllerReference(jenkinsInstance, service, bc.scheme)
	if err != nil {
		return nil, err
	}

	return service, nil
}

// newDeployment creates a new Deployment for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newDeployment(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) (*appsv1.Deployment, error) {
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	// get binary data for variables and groovy config
	jenkinsJvmEnv, err := bindata.Asset("environment/jenkins-jvm-environment")
	if err != nil {
		glog.Errorf("Error locating binary asset: %s", err)
		return nil, err
	}

	// Create environment variables
	// variables out of jenkins-jvm-environment
	var env []corev1.EnvVar
	scanner := bufio.NewScanner(strings.NewReader(string(jenkinsJvmEnv[:])))
	for scanner.Scan() {

		envComponents := strings.Split(scanner.Text(), ":")
		env = append(env, corev1.EnvVar{
			Name:  envComponents[0],
			Value: envComponents[1],
		})
	}

	// user-specified environment variables
	for envVar, envVarVal := range jenkinsInstance.Spec.Env {
		env = append(env, corev1.EnvVar{
			Name:  envVar,
			Value: envVarVal,
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
		volumeSource = corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}
	} else {
		volumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
				ReadOnly:  false,
			},
		}
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jenkinsInstance.GetName(),
			Namespace: jenkinsInstance.GetNamespace(),
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
									Name:          "master",
									ContainerPort: jenkinsInstance.Spec.MasterPort,
									HostPort:      jenkinsInstance.Spec.MasterPort,
									Protocol:      "TCP",
								},
								{
									Name:          "agent",
									ContainerPort: jenkinsInstance.Spec.AgentPort,
									HostPort:      jenkinsInstance.Spec.AgentPort,
									Protocol:      "TCP",
								},
							},
							Env: env,
							Command: []string{
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
								Secret: &corev1.SecretVolumeSource{
									SecretName: jenkinsInstance.GetName(),
								},
							},
						},
						{
							Name:         "job-storage",
							VolumeSource: volumeSource,
						},
					},
				},
			},
		},
	}

	err = controllerutil.SetControllerReference(jenkinsInstance, deployment, bc.scheme)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}
