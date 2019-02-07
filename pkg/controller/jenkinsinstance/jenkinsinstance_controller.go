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
	"text/template"
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
								fmt.Sprintf("%s%c%s", inst.GetNamespace(), types.Separator, inst.GetName())),
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

func (bc *ReconcileJenkinsInstance) getSetupSecret(instanceName types.NamespacedName) (*corev1.Secret, error) {
	setupSecret := &corev1.Secret{}
	err := bc.Client.Get(context.TODO(), instanceName, setupSecret)
	return setupSecret, err
}

func (bc *ReconcileJenkinsInstance) getAdminSecret(instanceName types.NamespacedName) (*corev1.Secret, error) {
	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, fmt.Errorf("Could not get Jenkins instance %s: %v", instanceName.String(), err)
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
		return nil, fmt.Errorf("Could not get Jenkins instance %s: %v", instanceName.String(), err)
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

func (bc *ReconcileJenkinsInstance) getIngress(instanceName types.NamespacedName) (*v1beta1.Ingress, error) {
	ingress := &v1beta1.Ingress{}
	err := bc.Client.Get(
		context.TODO(),
		instanceName,
		ingress)

	return ingress, err
}

func (bc *ReconcileJenkinsInstance) getNetworkPolicy(instanceName types.NamespacedName) (*netv1.NetworkPolicy, error) {
	policy := &netv1.NetworkPolicy{}
	err := bc.Client.Get(
		context.TODO(),
		instanceName,
		policy)

	return policy, err
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
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=list
// +kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
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
	adminSecretKey := types.NewNamespacedNameFromString(
		fmt.Sprintf("%s%c%s", request.Namespace, types.Separator, adminSecretName))

	adminSecret := &corev1.Secret{}
	err = bc.Client.Get(context.TODO(), adminSecretKey, adminSecret)
	if errors.IsNotFound(err) {
		glog.Errorf("AdminSecret %s does not exist yet: %v", adminSecretKey.String(), err)
		return reconcile.Result{}, err
	}

	// Get the setup utility secret managed by this controller
	_, err = bc.newSetupSecret(request.NamespacedName)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating setup secret: %s", err)
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

	// Get the ingress with the name specified in JenkinsInstance.spec
	_, err = bc.newIngress(request.NamespacedName)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating ingress: %s", err)
		return reconcile.Result{}, err
	}

	// Setup Network policy if specified in JenkinsInstance.spec
	_, err = bc.newNetworkPolicy(request.NamespacedName)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating network policy: %s", err)
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

	setupSecret, err := bc.getSetupSecret(instanceName)
	if err != nil {
		return err
	}

	adminSecret, err := bc.getAdminSecret(instanceName)
	if err != nil {
		return err
	}

	service, err := bc.getService(instanceName)
	if err != nil {
		return err
	}

	apiToken := string(setupSecret.Data["apiToken"][:])
	apiTokenUuid := string(setupSecret.Data["apiTokenUuid"][:])
	valid, err := util.JenkinsApiTokenValid(service, adminSecret, setupSecret, JenkinsMasterPort)
	if err != nil {
		return err
	}

	if !valid {
		err = util.RevokeJenkinsApiToken(service, adminSecret, setupSecret, JenkinsMasterPort)
		if err != nil {
			glog.Warningf("Error revoking jenkins api token: %v", err)
		}
		apiToken, apiTokenUuid, err = util.GetJenkinsApiToken(service, adminSecret, JenkinsMasterPort)
		if err != nil {
			return err
		}
	}

	if (string(setupSecret.Data["apiToken"][:]) != apiToken) ||
		(string(setupSecret.Data["apiTokenUuid"][:]) != apiTokenUuid) {
		setupSecretCopy := setupSecret.DeepCopy()
		setupSecretCopy.Data["apiToken"] = []byte(apiToken)
		setupSecretCopy.Data["apiTokenUuid"] = []byte(apiTokenUuid)
		err = bc.Client.Update(context.TODO(), setupSecretCopy)
		if err != nil {
			return err
		}
	}

	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return err
	}

	doUpdate := (jenkinsInstance.Status.Phase != JenkinsInstancePhaseReady) ||
		(jenkinsInstance.Status.SetupSecret != setupSecret.GetName())

	if doUpdate {
		jenkinsInstanceCopy := jenkinsInstance.DeepCopy()
		jenkinsInstanceCopy.Status.Phase = JenkinsInstancePhaseReady
		jenkinsInstanceCopy.Status.SetupSecret = setupSecret.GetName()

		err = bc.Client.Update(context.TODO(), jenkinsInstanceCopy)
		if err != nil {
			return err
		}
	}

	return nil
}

// newSetupSecret creates an admin password secret for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newSetupSecret(instanceName types.NamespacedName) (*corev1.Secret, error) {
	exists := false
	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, err
	}

	setupSecret, err := bc.getSetupSecret(instanceName)
	// If the resource doesn't exist, we'll create it
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		// If the Secret is not controlled by this JenkinsInstance resource, we should log
		// a warning to the event recorder and return
		if !metav1.IsControlledBy(setupSecret, jenkinsInstance) {
			msg := fmt.Sprintf(MessageResourceExists, setupSecret.GetName())
			bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
			return setupSecret, fmt.Errorf(msg)
		}

		exists = true
	}

	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	adminUserConfig, err := configdata.Asset("casc/jenkins.yaml")
	if err != nil {
		return nil, err
	}

	type JenkinsInfo struct {
		Plugins    map[string]string
		User       string
		Password   string
		Url        string
		AdminEmail string
		AgentPort  int32
		Executors  int32
	}

	adminSecret, err := bc.getAdminSecret(instanceName)
	if err != nil {
		return nil, err
	}

	// decode Admin secret strings
	adminUser := string(adminSecret.Data["user"][:])
	adminPassword := string(adminSecret.Data["pass"][:])
	if err != nil {
		return nil, err
	}

	jenkinsInfo := JenkinsInfo{
		Plugins:    make(map[string]string),
		User:       string(adminUser[:]),
		Password:   string(adminPassword[:]),
		Url:        jenkinsInstance.Spec.Location,
		AdminEmail: jenkinsInstance.Spec.AdminEmail,
		AgentPort:  JenkinsAgentPort,
		Executors:  jenkinsInstance.Spec.Executors,
	}

	// parse the plugin array
	requiredPlugin, err := configdata.Asset("environment/required-plugins")
	if err != nil {
		return nil, err
	}
	plugins := jenkinsInstance.Spec.Plugins

	// add required plugins first
	scanner := bufio.NewScanner(strings.NewReader(string(requiredPlugin[:])))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), ":")
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid format of default plugins: %s", scanner.Text())
		}

		jenkinsInfo.Plugins[parts[0]] = parts[1]
	}

	// add user plugins next
	for _, plugin := range plugins {
		jenkinsInfo.Plugins[plugin.Id] = plugin.Version
	}

	// parse the groovy config template
	configTemplate, err := template.New("jenkins-config").Parse(string(adminUserConfig[:]))
	if err != nil {
		return nil, err
	}

	var jenkinsConfigParsed bytes.Buffer
	if err := configTemplate.Execute(&jenkinsConfigParsed, jenkinsInfo); err != nil {
		return nil, err
	}

	// add things to the string data
	byteData := map[string][]byte{
		"jenkins.yaml": []byte(jenkinsConfigParsed.String()),
		"user":         []byte(adminUser),
	}

	// add plugin configurations
	for _, plugin := range plugins {
		byteData["plugin-"+plugin.Id+".yaml"] = []byte(plugin.Config)
	}

	if exists {
		setupSecretCopy := setupSecret.DeepCopy()
		setupSecretCopy.Data = util.MergeSecretData(byteData, setupSecret.Data)
		setupSecretCopy.Labels = labels

		if reflect.DeepEqual(setupSecretCopy.Data, setupSecret.Data) {
			return setupSecret, nil
		}

		glog.Info("updating secret")
		err = bc.Client.Update(context.TODO(), setupSecretCopy)
		if err != nil {
			return setupSecretCopy, err
		}

		// safe restart jenkins
		service, err := bc.getService(instanceName)
		if err != nil {
			return setupSecretCopy, err
		}

		err = util.SafeRestartJenkins(service, setupSecretCopy, JenkinsMasterPort)
		if err != nil {
			return setupSecretCopy, err
		}

		if err != nil {
			glog.Errorf("failed to restart jenkins instance %s after setup secret %s was updated",
				jenkinsInstance.GetName(), setupSecret.GetName())
		}
		return setupSecretCopy, err
	} else {
		setupSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jenkinsInstance.GetName(),
				Namespace: jenkinsInstance.GetNamespace(),
				Labels:    labels,
			},
			Data: byteData,
			Type: corev1.SecretTypeOpaque,
		}

		err = controllerutil.SetControllerReference(jenkinsInstance, setupSecret, bc.scheme)
		if err != nil {
			return nil, err
		}

		err = bc.Client.Create(context.TODO(), setupSecret)
		return setupSecret, err
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

// newIngress creates a new Ingress for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newIngress(instanceName types.NamespacedName) (*v1beta1.Ingress, error) {
	exists := false

	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, err
	}

	if jenkinsInstance.Spec.Ingress == nil {
		return nil, nil
	}

	ingress, err := bc.getIngress(instanceName)

	// If the ingress doesn't exist, we'll create it
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		// If the Ingress is not controlled by this JenkinsInstance resource, we should log
		// a warning to the event recorder and ret
		if !metav1.IsControlledBy(ingress, jenkinsInstance) {
			msg := fmt.Sprintf(MessageResourceExists, ingress.GetName())
			bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
			return ingress, fmt.Errorf(msg)
		}

		exists = true
	}

	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	serviceName := jenkinsInstance.GetName()
	if jenkinsInstance.Spec.Service != nil && jenkinsInstance.Spec.Service.Name != "" {
		serviceName = jenkinsInstance.Spec.Service.Name
	}
	if jenkinsInstance.Spec.Ingress.Service != "" {
		serviceName = jenkinsInstance.Spec.Ingress.Service
	}

	ingressPath := jenkinsInstance.Spec.Ingress.Path
	if ingressPath == "" {
		ingressPath = "/"
	}

	if exists {
		ingressCopy := ingress.DeepCopy()
		ingressCopy.Labels = labels
		ingressCopy.Spec.TLS = []v1beta1.IngressTLS{
			{
				SecretName: jenkinsInstance.Spec.Ingress.TlsSecret,
				Hosts: []string{
					util.GetJenkinsLocationHost(jenkinsInstance),
				},
			},
		}
		ingressCopy.Spec.Rules = []v1beta1.IngressRule{
			{
				Host: util.GetJenkinsLocationHost(jenkinsInstance),
				IngressRuleValue: v1beta1.IngressRuleValue{
					HTTP: &v1beta1.HTTPIngressRuleValue{
						Paths: []v1beta1.HTTPIngressPath{
							{
								Path: ingressPath,
								Backend: v1beta1.IngressBackend{
									ServiceName: serviceName,
									ServicePort: intstr.IntOrString{
										Type:   intstr.Int,
										IntVal: JenkinsMasterPort,
									},
								},
							},
						},
					},
				},
			},
		}

		if reflect.DeepEqual(ingressCopy.Spec, ingress.Spec) {
			return ingress, nil
		}

		glog.Info("updating ingress")
		err = bc.Client.Update(context.TODO(), ingressCopy)
		return ingress, err

	} else {

		ingress = &v1beta1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:        jenkinsInstance.GetName(),
				Namespace:   jenkinsInstance.GetNamespace(),
				Labels:      labels,
				Annotations: jenkinsInstance.Spec.Ingress.Annotations,
			},
			Spec: v1beta1.IngressSpec{
				TLS: []v1beta1.IngressTLS{
					{
						SecretName: jenkinsInstance.Spec.Ingress.TlsSecret,
						Hosts: []string{
							util.GetJenkinsLocationHost(jenkinsInstance),
						},
					},
				},
				Rules: []v1beta1.IngressRule{
					{
						Host: util.GetJenkinsLocationHost(jenkinsInstance),
						IngressRuleValue: v1beta1.IngressRuleValue{
							HTTP: &v1beta1.HTTPIngressRuleValue{
								Paths: []v1beta1.HTTPIngressPath{
									{
										Path: ingressPath,
										Backend: v1beta1.IngressBackend{
											ServiceName: serviceName,
											ServicePort: intstr.IntOrString{
												Type:   intstr.Int,
												IntVal: JenkinsMasterPort,
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

		err = controllerutil.SetControllerReference(jenkinsInstance, ingress, bc.scheme)
		if err != nil {
			return nil, err
		}

		err = bc.Client.Create(context.TODO(), ingress)
		return ingress, err
	}
}

// newNetworkPolicy creates a NetworkPolicy (if needed) for a JenkinsInstance. It also sets
// the appropriate OwnerReferences on the resources so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newNetworkPolicy(instanceName types.NamespacedName) (*netv1.NetworkPolicy, error) {
	exists := false

	jenkinsInstance, err := bc.getJenkinsInstance(instanceName)
	if err != nil {
		return nil, err
	}

	if !jenkinsInstance.Spec.NetworkPolicy {
		return nil, nil
	}

	policy, err := bc.getNetworkPolicy(instanceName)

	// If the policy doesn't exist, we'll create it
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		// If the role binding is not controlled by this JenkinsInstance resource, we should log
		// a warning to the event recorder
		if !metav1.IsControlledBy(policy, jenkinsInstance) {
			msg := fmt.Sprintf(MessageResourceExists, policy.GetName())
			bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
			return policy, fmt.Errorf(msg)
		}

		exists = true
	}

	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	if exists {
		policyCopy := policy.DeepCopy()
		policyCopy.Labels = labels
		policyCopy.Spec.PodSelector = metav1.LabelSelector{
			MatchLabels: labels,
		}
		policyCopy.Spec.Ingress = []netv1.NetworkPolicyIngressRule{
			{
				Ports: []netv1.NetworkPolicyPort{
					{
						Port: &intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: JenkinsMasterPort,
						},
					},
					{
						Port: &intstr.IntOrString{
							Type:   intstr.Int,
							IntVal: JenkinsAgentPort,
						},
					},
				},
			},
		}

		if reflect.DeepEqual(policyCopy.Spec.PodSelector, policy.Spec.PodSelector) {
			masterPortPresent := false
			agentPortPresent := false
			for _, policyRule := range policy.Spec.Ingress {
				for _, port := range policyRule.Ports {
					if port.Port.IntVal == JenkinsMasterPort {
						masterPortPresent = true
					}

					if port.Port.IntVal == JenkinsAgentPort {
						agentPortPresent = true
					}
				}
			}

			if agentPortPresent && masterPortPresent {
				return policy, nil
			}
		}

		glog.Info("updating policy")
		err = bc.Client.Update(context.TODO(), policyCopy)
		return policyCopy, err
	} else {
		policy = &netv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jenkinsInstance.GetName(),
				Namespace: jenkinsInstance.GetNamespace(),
				Labels:    labels,
			},

			Spec: netv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{
					MatchLabels: labels,
				},
				Ingress: []netv1.NetworkPolicyIngressRule{
					{
						Ports: []netv1.NetworkPolicyPort{
							{
								Port: &intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: JenkinsMasterPort,
								},
							},
							{
								Port: &intstr.IntOrString{
									Type:   intstr.Int,
									IntVal: JenkinsAgentPort,
								},
							},
						},
					},
				},
			},
		}

		err = controllerutil.SetControllerReference(jenkinsInstance, policy, bc.scheme)
		if err != nil {
			glog.Error("Could not set controller reference on network policy %s", jenkinsInstance.GetName())
			return nil, err
		}

		err = bc.Client.Create(context.TODO(), policy)
		return policy, err
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

	labels := map[string]string{
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
	commandString += "/usr/local/bin/install-plugins.sh $(cat /var/jenkins_home/init.groovy.d/plugins.txt | tr '\\n' ' ') && "
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
	var volumeSource corev1.VolumeSource
	if jenkinsInstance.Spec.Storage == nil {
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

	var replicas int32 = JenkinsReplicas
	var runAsUser int64 = 0

	// map the configuration-as-code files from setup secret
	setupSecret, err := bc.getSetupSecret(instanceName)
	if err != nil {
		return nil, err
	}
	var cascKeys []corev1.KeyToPath
	for index, _ := range setupSecret.Data {
		if strings.HasPrefix(index, "plugin-") {
			cascKeys = append(cascKeys, corev1.KeyToPath {
				Key: index,
				Path: index,
			})
		}
	}
	// add the jenkins.yaml casc config
	cascKeys = append(cascKeys, corev1.KeyToPath {
		Key: "jenkins.yaml",
		Path: "jenkins.yaml",
	})

	// setup volumes
	deploymentVolumes := []corev1.Volume {
		{
			Name: "casc_configs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: jenkinsInstance.GetName(),
					Items: cascKeys,
				},
			},
		},
		{
			Name:         "job-storage",
			VolumeSource: volumeSource,
		},
	}

	// add casc secrets
	if jenkinsInstance.Spec.CascSecrets != nil {
		for _, secret := range jenkinsInstance.Spec.CascSecrets {
			deploymentVolumes = append(deploymentVolumes, corev1.Volume{
				Name: secret.Secret,
			})
		}
	}
	// add groovy secrets
	if jenkinsInstance.Spec.GroovySecrets != nil {
		for _, secret := range jenkinsInstance.Spec.GroovySecrets {
			deploymentVolumes = append(deploymentVolumes, corev1.Volume{
				Name: secret.Secret,
			})
		}
	}


	// setup volume mounts
	deploymentVolumeMounts := []corev1.VolumeMount{
		{
			Name:      "casc_configs",
			ReadOnly:  true,
			MountPath: "/var/jenkins_home/casc_configs",
		},
		{
			Name:      "job-storage",
			ReadOnly:  false,
			MountPath: "/var/jenkins_home/jobs",
		},
	}
	// add casc secrets
	if jenkinsInstance.Spec.CascSecrets != nil {
		for _, secret := range jenkinsInstance.Spec.CascSecrets {
			cascSecret := &corev1.Secret{}
			err := bc.Client.Get(context.TODO(), types.NewNamespacedNameFromString(
				fmt.Sprintf("%s%c%s", instanceName.Namespace, types.Separator, secret.Secret)), cascSecret)
			if err != nil {
				return nil, err
			}

			for cascSecretKeyName, _ := range cascSecret.Data {
				deploymentVolumeMounts = append(deploymentVolumeMounts, corev1.VolumeMount{
					Name:      secret.Secret,
					MountPath: "/var/jenkins_home/casc_secrets",
					ReadOnly:  true,
					SubPath:   cascSecretKeyName,
				})
			}
		}
	}
	// add groovy secrets
	if jenkinsInstance.Spec.GroovySecrets != nil {
		for _, secret := range jenkinsInstance.Spec.GroovySecrets {
			groovySecret := &corev1.Secret{}
			err := bc.Client.Get(context.TODO(), types.NewNamespacedNameFromString(
				fmt.Sprintf("%s%c%s", instanceName.Namespace, types.Separator, secret.Secret)), groovySecret)
			if err != nil {
				return nil, err
			}

			for groovySecretKeyName, _ := range groovySecret.Data {
				deploymentVolumeMounts = append(deploymentVolumeMounts, corev1.VolumeMount{
					Name:      secret.Secret,
					MountPath: "/var/jenkins_home/init.groovy.d",
					ReadOnly:  true,
					SubPath:   groovySecretKeyName,
				})
			}
		}
	}

	if exists {
		deploymentCopy := deployment.DeepCopy()
		deploymentCopy.Annotations = jenkinsInstance.Spec.Annotations
		deploymentCopy.Spec.Replicas = &replicas
		deploymentCopy.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: labels,
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
				VolumeMounts: deploymentVolumeMounts,
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
					MatchLabels: labels,
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: labels,
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
								VolumeMounts: deploymentVolumeMounts,
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
