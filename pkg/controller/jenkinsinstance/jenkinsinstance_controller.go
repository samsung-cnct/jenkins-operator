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
	"context"
	"fmt"
	"github.com/golang/glog"
	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	"github.com/maratoid/jenkins-operator/pkg/configdata"
	"github.com/maratoid/jenkins-operator/pkg/util"
	"github.com/spf13/viper"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/record"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
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

const (
	JenkinsInstancePhaseReady = "Ready"
)

const (
	JenkinsMasterPort = 8080
	JenkinsAgentPort  = 50000
	JenkinsReplicas   = 1
	JenkinsPullPolicy = "Always"
)

// Add creates a new JenkinsInstance Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
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

	watchPredicate := util.NewPredicate(viper.GetString("namespace"))

	// Watch for changes to JenkinsInstance
	err = c.Watch(&source.Kind{Type: &jenkinsv1alpha1.JenkinsInstance{}}, &handler.EnqueueRequestForObject{}, watchPredicate)
	if err != nil {
		return err
	}

	// Watch a Deployment created by JenkinsInstance
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jenkinsv1alpha1.JenkinsInstance{},
	}, watchPredicate)
	if err != nil {
		return err
	}

	// Watch a PVC created by JenkinsInstance
	err = c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jenkinsv1alpha1.JenkinsInstance{},
	}, watchPredicate)
	if err != nil {
		return err
	}

	// Watch a Secret created by JenkinsInstance
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jenkinsv1alpha1.JenkinsInstance{},
	}, watchPredicate)
	if err != nil {
		return err
	}

	// Watch a Service created by JenkinsInstance
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jenkinsv1alpha1.JenkinsInstance{},
	}, watchPredicate)
	if err != nil {
		return err
	}

	// Watch an Ingress created by JenkinsInstance
	err = c.Watch(&source.Kind{Type: &v1beta1.Ingress{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jenkinsv1alpha1.JenkinsInstance{},
	}, watchPredicate)
	if err != nil {
		return err
	}

	// Watch a Networkpolicy created by JenkinsInstance
	err = c.Watch(&source.Kind{Type: &netv1.NetworkPolicy{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &jenkinsv1alpha1.JenkinsInstance{},
	}, watchPredicate)
	if err != nil {
		return err
	}

	// Watch ConfigMap resources not owned by the JenkinsInstance change
	// This is needed for re-loading jenkins configuration
	// When configuration config map changes, Watch will list all JenkinsInstances
	// and re-enqueue the keys for the ones that refer to the configuration config map via their spec.
	err = c.Watch(
		&source.Kind{Type: &corev1.ConfigMap{}},
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
					if inst.Spec.CascConfig.ConfigMap == a.Meta.GetName() {
						keys = append(keys, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Name:      inst.GetName(),
								Namespace: inst.GetNamespace(),
							},
						})
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

var _ reconcile.Reconciler = &ReconcileJenkinsInstance{}

// ReconcileJenkinsInstance reconciles a JenkinsInstance object
type ReconcileJenkinsInstance struct {
	client.Client
	record.EventRecorder
	scheme *runtime.Scheme
}

type JenkinsTokenRequest struct {
	ctx        context.Context
	cancelFunc context.CancelFunc
}

func (bc *ReconcileJenkinsInstance) getJenkinsInstance(name types.NamespacedName) (*jenkinsv1alpha1.JenkinsInstance, error) {
	jenkinsInstance := &jenkinsv1alpha1.JenkinsInstance{}
	err := bc.Client.Get(context.TODO(), name, jenkinsInstance)
	return jenkinsInstance, err
}

func (bc *ReconcileJenkinsInstance) getSetupConfigMap(instanceName types.NamespacedName) (*corev1.ConfigMap, error) {
	setupConfigMap := &corev1.ConfigMap{}
	err := bc.Client.Get(context.TODO(), instanceName, setupConfigMap)
	return setupConfigMap, err
}

func (bc *ReconcileJenkinsInstance) getAdminSecret(instanceName types.NamespacedName) (*corev1.Secret, error) {
	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, fmt.Errorf("could not get Jenkins instance %s: %v", instanceName.String(), err)
	}

	adminSecret := &corev1.Secret{}
	err = bc.Client.Get(
		context.TODO(),
		types.NamespacedName{Namespace: instanceName.Namespace, Name: jenkinsInstance.Spec.AdminSecret},
		adminSecret)
	return adminSecret, err
}

func (bc *ReconcileJenkinsInstance) getService(instanceName types.NamespacedName) (*corev1.Service, error) {
	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, fmt.Errorf("could not get Jenkins instance %s: %v", instanceName.String(), err)
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

func (bc *ReconcileJenkinsInstance) getDeployment(instanceName types.NamespacedName) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	err := bc.Client.Get(
		context.TODO(),
		instanceName,
		deployment)

	return deployment, err
}

// Reconcile reads that state of the cluster for a JenkinsInstance object and makes changes based on the state read
// and what is in the JenkinsInstance.Spec
// Automatically generate RBAC rules to allow the Controller to read and write objects
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=list
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=jenkins.jenkinsoperator.maratoid.github.com,resources=jenkinsinstances,verbs=get;list;watch;create;update;patch;delete
func (bc *ReconcileJenkinsInstance) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	jenkinsInstance, err := bc.getJenkinsInstance(request.NamespacedName)
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Errorf("JenkinsInstance key '%s' in work queue no longer exists", request.String())
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	adminSecretName := jenkinsInstance.Spec.AdminSecret
	if adminSecretName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		glog.Errorf("%s: AdminSecret name must be specified", request.String())
		return reconcile.Result{}, nil
	}
	adminSecretKey := types.NamespacedName{Namespace: request.Namespace, Name: adminSecretName}

	adminSecret := &corev1.Secret{}
	err = bc.Client.Get(context.TODO(), adminSecretKey, adminSecret)
	if errors.IsNotFound(err) {
		glog.Errorf("AdminSecret %s does not exist yet: %v", adminSecretKey.String(), err)
		return reconcile.Result{}, err
	}

	// Get the setup utility secret managed by this controller
	_, err = bc.newSetupConfigMap(request.NamespacedName)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating setup config map: %s", err)
		return reconcile.Result{}, err
	}

	_, err = bc.newDeployment(request.NamespacedName)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating deployment: %s", err)
		return reconcile.Result{}, err
	}

	_, err = bc.newService(request.NamespacedName)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating service: %s", err)
		return reconcile.Result{}, err
	}

	// Finally, we update the status block of the JenkinsInstance resource to reflect the
	// current state of the world
	err = bc.updateJenkinsInstanceStatus(request.NamespacedName)
	if err != nil {
		glog.Errorf("Error updating JenkinsInstance %s: %v", request.String(), err)
		return reconcile.Result{}, err
	}

	bc.Event(jenkinsInstance, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return reconcile.Result{}, nil
}

// updateJenkinsInstanceStatus updates status fields of the jenkins instance object and emits events
func (bc *ReconcileJenkinsInstance) updateJenkinsInstanceStatus(instanceName types.NamespacedName) error {

	service, err := bc.getService(instanceName)
	if err != nil {
		return err
	}

	err = util.CheckJenkinsReady(service, JenkinsMasterPort)
	if err != nil {
		return err
	}

	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return err
	}

	doUpdate := jenkinsInstance.Status.Phase != JenkinsInstancePhaseReady

	if doUpdate {
		jenkinsInstance.Status.Phase = JenkinsInstancePhaseReady
		err = bc.Status().Update(context.Background(), jenkinsInstance)
		if err != nil {
			return err
		}
	}

	return nil
}

// newSetupConfigMap creates an admin password ConfigMap for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newSetupConfigMap(instanceName types.NamespacedName) (*corev1.ConfigMap, error) {
	exists := false
	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, err
	}

	setupConfigMap, err := bc.getSetupConfigMap(instanceName)
	// If the resource doesn't exist, we'll create it
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		// If the ConfigMap is not controlled by this JenkinsInstance resource, we should log
		// a warning to the event recorder and return
		if !metav1.IsControlledBy(setupConfigMap, jenkinsInstance) {
			msg := fmt.Sprintf(MessageResourceExists, setupConfigMap.GetName())
			bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
			return setupConfigMap, fmt.Errorf(msg)
		}

		exists = true
	}

	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	// load required plugins
	requiredPlugins, err := configdata.Asset("environment/required-plugins")
	if err != nil {
		return nil, err
	}

	var pluginList []string

	// add required plugins first
	scanner := bufio.NewScanner(strings.NewReader(string(requiredPlugins[:])))
	for scanner.Scan() {
		pluginList = append(pluginList, scanner.Text())
	}

	if jenkinsInstance.Spec.Plugins != nil {
		for _, plugin := range jenkinsInstance.Spec.Plugins {
			isAlreadyRequired := false
			for _, addedPlugin := range pluginList {
				components := strings.Split(addedPlugin, ":")
				if strings.EqualFold(components[0], plugin.Id) {
					isAlreadyRequired = true
					break
				}
			}

			if !isAlreadyRequired {
				pluginInfo := fmt.Sprintf("%s:%s", plugin.Id, plugin.Version)
				pluginList = append(pluginList, pluginInfo)
			}
		}
	}

	// add things to the string data
	stringData := map[string]string{
		"plugins.txt": strings.Join(pluginList, "\n"),
	}
	if jenkinsInstance.Spec.CascConfig.ConfigString != "" {
		stringData["jenkins.yaml"] = jenkinsInstance.Spec.CascConfig.ConfigString
	}

	if exists {
		setupConfigMapCopy := setupConfigMap.DeepCopy()
		setupConfigMapCopy.Data = util.MergeData(stringData, setupConfigMap.Data)
		setupConfigMapCopy.Labels = labels

		if reflect.DeepEqual(setupConfigMapCopy.Data, setupConfigMap.Data) {
			return setupConfigMap, nil
		}

		glog.Info("updating secret")
		err = bc.Client.Update(context.TODO(), setupConfigMapCopy)
		if err != nil {
			return setupConfigMapCopy, err
		}

		// safe restart jenkins
		service, err := bc.getService(instanceName)
		if err != nil {
			return setupConfigMapCopy, err
		}

		adminSecret, err := bc.getAdminSecret(instanceName)
		if err != nil {
			return nil, err
		}

		err = util.SafeRestartJenkins(service, adminSecret, JenkinsMasterPort)
		if err != nil {
			return setupConfigMapCopy, err
		}

		if err != nil {
			glog.Errorf("failed to restart jenkins instance %s after setup configmap %s was updated",
				jenkinsInstance.GetName(), setupConfigMap.GetName())
		}
		return setupConfigMapCopy, err
	} else {
		setupConfigMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jenkinsInstance.GetName(),
				Namespace: jenkinsInstance.GetNamespace(),
				Labels:    labels,
			},
			Data: stringData,
		}

		err = controllerutil.SetControllerReference(jenkinsInstance, setupConfigMap, bc.scheme)
		if err != nil {
			return nil, err
		}

		err = bc.Client.Create(context.TODO(), setupConfigMap)
		return setupConfigMap, err
	}
}

// newService creates a new Service for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newService(instanceName types.NamespacedName) (*corev1.Service, error) {
	exists := false

	// get jenkins instance
	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, err
	}

	// Get the service with the name specified in JenkinsInstance.spec
	service, err := bc.getService(instanceName)
	// If the resource doesn't exist, we'll create it
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		// If the Service is not controlled by this JenkinsInstance resource, we should log
		// a warning to the event recorder and ret
		if !metav1.IsControlledBy(service, jenkinsInstance) {
			msg := fmt.Sprintf(MessageResourceExists, service.GetName())
			bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
			return service, fmt.Errorf(msg)
		}

		exists = true
	}

	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	if exists {
		serviceCopy := service.DeepCopy()
		serviceCopy.Labels = labels
		serviceCopy.Spec.Ports = []corev1.ServicePort{
			{
				Name:     "master",
				Protocol: "TCP",
				Port:     JenkinsMasterPort,
				NodePort: util.GetNodePort(service.Spec.Ports, "master"),
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: JenkinsMasterPort,
				},
			},
			{
				Name:     "agent",
				Protocol: "TCP",
				Port:     JenkinsAgentPort,
				NodePort: util.GetNodePort(service.Spec.Ports, "agent"),
				TargetPort: intstr.IntOrString{
					Type:   intstr.Int,
					IntVal: JenkinsAgentPort,
				},
			},
		}
		serviceCopy.Spec.Selector = map[string]string{
			"component": string(jenkinsInstance.UID),
		}

		if jenkinsInstance.Spec.Service != nil {
			serviceCopy.ObjectMeta.Annotations = jenkinsInstance.Spec.Service.Annotations
			if jenkinsInstance.Spec.Service.ServiceType != "" {
				serviceCopy.Spec.Type = jenkinsInstance.Spec.Service.ServiceType
			}
		}

		if reflect.DeepEqual(serviceCopy.Spec, service.Spec) {
			return service, nil
		}

		glog.Info("updating service")
		err = bc.Client.Update(context.TODO(), serviceCopy)
		return serviceCopy, err
	} else {
		serviceName := jenkinsInstance.GetName()
		if jenkinsInstance.Spec.Service != nil && jenkinsInstance.Spec.Service.Name != "" {
			serviceName = jenkinsInstance.Spec.Service.Name
		}

		service = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: jenkinsInstance.GetNamespace(),
				Labels:    labels,
			},

			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "master",
						Protocol: "TCP",
						Port:     JenkinsMasterPort,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: JenkinsMasterPort,
						},
						NodePort: jenkinsInstance.Spec.Service.NodePort,
					},
					{
						Name:     "agent",
						Protocol: "TCP",
						Port:     JenkinsAgentPort,
						TargetPort: intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: JenkinsAgentPort,
						},
					},
				},
				Selector: map[string]string{
					"component": string(jenkinsInstance.UID),
				},
			},
		}

		if jenkinsInstance.Spec.Service != nil {
			service.ObjectMeta.Annotations = jenkinsInstance.Spec.Service.Annotations
			if jenkinsInstance.Spec.Service.ServiceType != "" {
				service.Spec.Type = jenkinsInstance.Spec.Service.ServiceType
			}
		}

		err = controllerutil.SetControllerReference(jenkinsInstance, service, bc.scheme)
		if err != nil {
			return nil, err
		}

		err = bc.Client.Create(context.TODO(), service)
		return service, nil
	}
}

// newDeployment creates a new Deployment for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newDeployment(instanceName types.NamespacedName) (*appsv1.Deployment, error) {
	exists := false

	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, err
	}

	// Get the deployment with the name specified in JenkinsInstance.spec
	deployment, err := bc.getDeployment(instanceName)

	// If the resource doesn't exist, we'll create it
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		// If the Deployment is not controlled by this JenkinsInstance resource, we should log
		// a warning to the event recorder and return
		if !metav1.IsControlledBy(deployment, jenkinsInstance) {
			msg := fmt.Sprintf(MessageResourceExists, deployment.GetName())
			bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
			return deployment, fmt.Errorf(msg)
		}

		exists = true
	}

	useLabels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	// get binary data for variables and groovy config
	jenkinsJvmEnv, err := configdata.Asset("environment/jenkins-jvm-environment")
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

	// CASC variables
	env = append(env, corev1.EnvVar{
		Name:  "CASC_JENKINS_CONFIG",
		Value: "/var/jenkins_home/casc_configs",
	})
	env = append(env, corev1.EnvVar{
		Name:  "SECRETS",
		Value: "/var/jenkins_home/casc_secrets",
	})

	// user-specified environment variables
	for envVar, envVarVal := range jenkinsInstance.Spec.Env {
		env = append(env, corev1.EnvVar{
			Name:  envVar,
			Value: envVarVal,
		})
	}

	// build a command string to install plugins and launch jenkins
	commandString := ""
	commandString += "/usr/local/bin/install-plugins.sh $(cat /var/jenkins_home/required_plugins/plugins.txt | tr '\\n' ' ') && "
	commandString += "/sbin/tini -- /usr/local/bin/jenkins.sh"
	commandString += ""

	// if service account name is specified, check that it exists
	if jenkinsInstance.Spec.ServiceAccount != "" {
		serviceAccount := &corev1.ServiceAccount{}
		err := bc.Client.Get(
			context.TODO(),
			types.NamespacedName{
				Namespace: jenkinsInstance.GetNamespace(),
				Name:      jenkinsInstance.Spec.ServiceAccount},
			serviceAccount)
		if err != nil {
			return nil, err
		}
	}

	// Get the correct volume source to use
	// if pvc name is specified, try to either locate it or create it
	pvcName := jenkinsInstance.GetName()
	if jenkinsInstance.Spec.Storage != nil {
		if jenkinsInstance.Spec.Storage.JobsPvc != "" {
			pvcName = jenkinsInstance.Spec.Storage.JobsPvc
		}
		pvc := &corev1.PersistentVolumeClaim{}
		err = bc.Client.Get(
			context.TODO(),
			types.NamespacedName{
				Namespace: jenkinsInstance.GetNamespace(),
				Name:      pvcName},
			pvc)

		// if PVC is not found
		if errors.IsNotFound(err) {
			// error out if pvc spec is not specified
			if jenkinsInstance.Spec.Storage.JobsPvcSpec == nil {
				return nil, fmt.Errorf(
					"PVC %s does not exist and JobsPvcSpec is not specified",
					pvcName)
			}

			// otherwise create the pvc
			pvc = &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      pvcName,
					Namespace: jenkinsInstance.GetNamespace(),
				},
				Spec: *jenkinsInstance.Spec.Storage.JobsPvcSpec,
			}
			err = controllerutil.SetControllerReference(jenkinsInstance, pvc, bc.scheme)
			if err != nil {
				return nil, err
			}
			err = bc.Client.Create(context.TODO(), pvc)
			if err != nil {
				return nil, err
			}
		}
	}

	// if PVC name is not specified, use an EmptyDir
	var jobVolumeSource corev1.VolumeSource
	if jenkinsInstance.Spec.Storage == nil {
		jobVolumeSource = corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}
	} else {
		jobVolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: pvcName,
				ReadOnly:  false,
			},
		}
	}

	var replicas int32 = JenkinsReplicas
	var runAsUser int64 = 0

	// setup volumes
	deploymentVolumes := []corev1.Volume{
		{
			Name: "required-plugins",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: jenkinsInstance.Name,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "plugins.txt",
							Path: "plugins.txt",
						},
					},
				},
			},
		},
		{
			Name:         "job-storage",
			VolumeSource: jobVolumeSource,
		},
	}

	// add casc secrets
	if jenkinsInstance.Spec.CascSecret != "" {
		// check if exists
		cascSecret := &corev1.Secret{}
		err := bc.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: instanceName.Namespace,
			Name:      jenkinsInstance.Spec.CascSecret,
		}, cascSecret)
		if err != nil {
			return nil, err
		}

		deploymentVolumes = append(deploymentVolumes, corev1.Volume{
			Name: "casc-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: jenkinsInstance.Spec.CascSecret,
				},
			},
		})
	}
	// add groovy secrets
	if jenkinsInstance.Spec.GroovySecret != "" {
		groovySecret := &corev1.Secret{}
		err := bc.Client.Get(context.TODO(), types.NamespacedName{
			Namespace: instanceName.Namespace,
			Name:      jenkinsInstance.Spec.GroovySecret,
		}, groovySecret)
		if err != nil {
			return nil, err
		}

		var items []corev1.KeyToPath
		for groovySecretKeyName, _ := range groovySecret.Data {
			items = append(items, corev1.KeyToPath{
				Key:  groovySecretKeyName,
				Path: groovySecretKeyName + ".groovy",
			})
		}

		deploymentVolumes = append(deploymentVolumes, corev1.Volume{
			Name: "groovy-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: jenkinsInstance.Spec.GroovySecret,
					Items:      items,
				},
			},
		})
	}

	// add casc config from either setup config map or named config map
	if jenkinsInstance.Spec.CascConfig != nil {
		if jenkinsInstance.Spec.CascConfig.ConfigMap != "" {
			// check if exists
			cascConfig := &corev1.ConfigMap{}
			err := bc.Client.Get(context.TODO(), types.NamespacedName{
				Namespace: instanceName.Namespace,
				Name:      jenkinsInstance.Spec.CascConfig.ConfigMap,
			}, cascConfig)
			if err != nil {
				return nil, err
			}

			var items []corev1.KeyToPath
			for cascConfigKeyName, _ := range cascConfig.Data {
				items = append(items, corev1.KeyToPath{
					Key:  cascConfigKeyName,
					Path: cascConfigKeyName + ".yaml",
				})
			}

			deploymentVolumes = append(deploymentVolumes, corev1.Volume{
				Name: "casc-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: jenkinsInstance.Spec.CascConfig.ConfigMap,
						},
						Items: items,
					},
				},
			})
		} else if jenkinsInstance.Spec.CascConfig.ConfigString != "" {
			deploymentVolumes = append(deploymentVolumes, corev1.Volume{
				Name: "casc-config",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: jenkinsInstance.Name,
						},
						Items: []corev1.KeyToPath{
							{
								Key:  "jenkins.yaml",
								Path: "jenkins.yaml",
							},
						},
					},
				},
			})
		} else {
			return nil, fmt.Errorf("cascconfig must specify configstring or configmap")
		}
	}

	// setup volume mounts
	deploymentVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "casc-config",
			ReadOnly:  true,
			MountPath: "/var/jenkins_home/casc_configs",
		},
		{
			Name:      "job-storage",
			ReadOnly:  false,
			MountPath: "/var/jenkins_home/jobs",
		},
		{
			Name:      "required-plugins",
			ReadOnly:  true,
			MountPath: "/var/jenkins_home/required_plugins",
		},
	}

	if jenkinsInstance.Spec.CascSecret != "" {
		deploymentVolumeMounts = append(deploymentVolumeMounts, corev1.VolumeMount{
			Name:      "casc-secret",
			ReadOnly:  true,
			MountPath: "/var/jenkins_home/casc_secrets",
		})
	}

	if jenkinsInstance.Spec.GroovySecret != "" {
		deploymentVolumeMounts = append(deploymentVolumeMounts, corev1.VolumeMount{
			Name:      "groovy-secret",
			ReadOnly:  false,
			MountPath: "/var/jenkins_home/init.groovy.d",
		})
	}

	if exists {
		deploymentCopy := deployment.DeepCopy()
		deploymentCopy.Annotations = jenkinsInstance.Spec.Annotations
		deploymentCopy.Spec.Replicas = &replicas
		deploymentCopy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: useLabels,
		}
		deploymentCopy.Spec.Template.Spec.Containers = []corev1.Container{
			{
				Name:  "jenkinsci",
				Image: jenkinsInstance.Spec.Image,
				Ports: []corev1.ContainerPort{
					{
						Name:          "master",
						ContainerPort: JenkinsMasterPort,
						HostPort:      JenkinsMasterPort,
						Protocol:      "TCP",
					},
					{
						Name:          "agent",
						ContainerPort: JenkinsAgentPort,
						HostPort:      JenkinsAgentPort,
						Protocol:      "TCP",
					},
				},
				Env: env,
				Command: []string{
					"bash",
					"-c",
					commandString,
				},
				ImagePullPolicy: JenkinsPullPolicy,
				VolumeMounts:    deploymentVolumeMounts,
			},
		}
		deploymentCopy.Spec.Template.Spec.Volumes = deploymentVolumes
		deploymentCopy.Spec.Template.Spec.ServiceAccountName = jenkinsInstance.Spec.ServiceAccount

		changed := reflect.DeepEqual(deploymentCopy.Annotations, deployment.Annotations) &&
			reflect.DeepEqual(deploymentCopy.Spec.Selector, deployment.Spec.Selector) &&
			reflect.DeepEqual(deploymentCopy.Spec.Template.Spec.Containers, deployment.Spec.Template.Spec.Containers) &&
			reflect.DeepEqual(deploymentCopy.Spec.Template.Spec.Volumes, deployment.Spec.Template.Spec.Volumes) &&
			(deploymentCopy.Spec.Replicas == deployment.Spec.Replicas) &&
			(deploymentCopy.Spec.Template.Spec.ServiceAccountName == deployment.Spec.Template.Spec.ServiceAccountName)

		if !changed {
			return deployment, nil
		}

		glog.Info("updating deployment")
		err = bc.Client.Update(context.TODO(), deploymentCopy)
		return deploymentCopy, err

	} else {
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:        jenkinsInstance.GetName(),
				Namespace:   jenkinsInstance.GetNamespace(),
				Annotations: jenkinsInstance.Spec.Annotations,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: useLabels,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: useLabels,
					},
					Spec: corev1.PodSpec{
						SecurityContext: &corev1.PodSecurityContext{
							RunAsUser: &runAsUser,
						},
						Containers: []corev1.Container{
							{
								Name:  "jenkinsci",
								Image: jenkinsInstance.Spec.Image,
								Ports: []corev1.ContainerPort{
									{
										Name:          "master",
										ContainerPort: JenkinsMasterPort,
										HostPort:      JenkinsMasterPort,
										Protocol:      "TCP",
									},
									{
										Name:          "agent",
										ContainerPort: JenkinsAgentPort,
										HostPort:      JenkinsAgentPort,
										Protocol:      "TCP",
									},
								},
								Env: env,
								Command: []string{
									"bash",
									"-c",
									commandString,
								},
								ImagePullPolicy: JenkinsPullPolicy,
								VolumeMounts:    deploymentVolumeMounts,
							},
						},

						Volumes: deploymentVolumes,
					},
				},
			},
		}

		// assign service account
		deployment.Spec.Template.Spec.ServiceAccountName = jenkinsInstance.Spec.ServiceAccount

		err = controllerutil.SetControllerReference(jenkinsInstance, deployment, bc.scheme)
		if err != nil {
			return nil, err
		}

		err = bc.Client.Create(context.TODO(), deployment)
		return deployment, err
	}
}
