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
	"k8s.io/api/extensions/v1beta1"
	netv1 "k8s.io/api/networking/v1"
	authv1 "k8s.io/api/rbac/v1"
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

const (
	JenkinsMasterPort = 8080
	JenkinsAgentPort  = 50000
	JenkinsReplicas   = 1
	JenkinsPullPolicy = "Always"
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
								fmt.Sprintf("%s%c%s", inst.GetNamespace(), types.Separator, inst.GetName())),
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
// +kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete
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
	setupSecret, err := bc.newSetupSecret(jenkinsInstance, adminSecret)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating setup secret: %s", err)
		return reconcile.Result{}, err
	}

	// Get the service account with the name specified in JenkinsInstance.spec
	_, err = bc.newServiceAccount(jenkinsInstance)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating serviceaccount: %s", err)
		return reconcile.Result{}, err
	}

	deployment, err := bc.newDeployment(jenkinsInstance)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating deployment: %s", err)
		return reconcile.Result{}, err
	}

	service, err := bc.newService(jenkinsInstance)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating service: %s", err)
		return reconcile.Result{}, err
	}

	// Get the ingress with the name specified in JenkinsInstance.spec
	_, err = bc.newIngress(jenkinsInstance)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating ingress: %s", err)
		return reconcile.Result{}, err
	}

	// Setup RBAC with the names specified in JenkinsInstance.spec
	_, err = bc.newRoleBinding(jenkinsInstance)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating role binding: %s", err)
		return reconcile.Result{}, err
	}

	// Setup Network policy if specified in JenkinsInstance.spec
	_, err = bc.newNetworkPolicy(jenkinsInstance)
	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating network policy: %s", err)
		return reconcile.Result{}, err
	}

	// TODO: Update all the components created by the controller if needed
	/*if jenkinsInstance.Spec.Replicas != nil && *jenkinsInstance.Spec.Replicas != *deployment.Spec.Replicas {
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

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		glog.Errorf("Error creating deployment: %s", err)
		return reconcile.Result{}, err
	}*/

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
		apiToken, err = util.GetJenkinsApiToken(jenkinsInstance, service, adminSecret, JenkinsMasterPort)
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
	exists := false
	setupSecret := &corev1.Secret{}
	err := bc.Client.Get(
		context.TODO(), types.NewNamespacedNameFromString(
			fmt.Sprintf("%s%c%s", jenkinsInstance.GetNamespace(), types.Separator,
				jenkinsInstance.GetName())),
		setupSecret)
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
		AgentPort:  JenkinsAgentPort,
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

	if exists {
		setupSecretCopy := setupSecret.DeepCopy()
		setupSecretCopy.StringData = stringData
		err = bc.Client.Update(context.TODO(), setupSecretCopy)
		return setupSecretCopy, err
	} else {
		setupSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jenkinsInstance.GetName(),
				Namespace: jenkinsInstance.GetNamespace(),
				Labels:    labels,
			},
			StringData: stringData,
			Type:       corev1.SecretTypeOpaque,
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
func (bc *ReconcileJenkinsInstance) newService(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) (*corev1.Service, error) {
	// Get the service with the name specified in JenkinsInstance.spec
	serviceName := jenkinsInstance.GetName()
	if jenkinsInstance.Spec.Service != nil && jenkinsInstance.Spec.Service.Name != "" {
		serviceName = jenkinsInstance.Spec.Service.Name
	}
	service := &corev1.Service{}
	err := bc.Client.Get(
		context.TODO(),
		types.NewNamespacedNameFromString(
			fmt.Sprintf("%s%c%s", jenkinsInstance.GetNamespace(), types.Separator, serviceName)),
		service)
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

		return service, nil
	}

	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
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

// newIngress creates a new Ingress for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newIngress(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) (*v1beta1.Ingress, error) {
	if jenkinsInstance.Spec.Ingress == nil {
		return nil, nil
	}

	ingress := &v1beta1.Ingress{}
	err := bc.Client.Get(
		context.TODO(),
		types.NewNamespacedNameFromString(
			fmt.Sprintf("%s%c%s", jenkinsInstance.GetNamespace(), types.Separator, jenkinsInstance.GetName())),
		ingress)
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

		return ingress, nil
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

// newRoleBinding creates role bindings for a JenkinsInstance. It also sets
// the appropriate OwnerReferences on the resources so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newRoleBinding(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) (*authv1.ClusterRoleBinding, error) {
	if jenkinsInstance.Spec.Rbac == nil {
		return nil, nil
	}

	binding := &authv1.ClusterRoleBinding{}
	err := bc.Client.Get(
		context.TODO(),
		types.NewNamespacedNameFromString(
			fmt.Sprintf("%s%c%s", jenkinsInstance.GetNamespace(), types.Separator, jenkinsInstance.GetName())),
		binding)
	// If the binding doesn't exist, we'll create it
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		// If the role binding is not controlled by this JenkinsInstance resource, we should log
		// a warning to the event recorder and ret
		if !metav1.IsControlledBy(binding, jenkinsInstance) {
			msg := fmt.Sprintf(MessageResourceExists, binding.GetName())
			bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
			return binding, fmt.Errorf(msg)
		}

		return binding, nil
	}

	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	if jenkinsInstance.Spec.Rbac.Clusterrole == "" {
		return nil, fmt.Errorf("RBAC is enabled but there is no clusterrole specified")
	}
	clusterrole := &authv1.ClusterRole{}
	err = bc.Client.Get(context.TODO(), types.NewNamespacedNameFromString(
		fmt.Sprintf(
			"%s%c%s",
			jenkinsInstance.Namespace,
			types.Separator,
			jenkinsInstance.Spec.Rbac.Clusterrole)),
		clusterrole)
	if err != nil {
		glog.Error("RBAC is enabled but could not find specified cluster role")
		return nil, err
	}

	if jenkinsInstance.Spec.ServiceAccount == nil {
		return nil, fmt.Errorf("RBAC is enabled, but JenkinsInstance serviceaccount is not specified")
	}

	serviceAccountName := jenkinsInstance.GetName()
	if jenkinsInstance.Spec.ServiceAccount.Name != "" {
		serviceAccountName = jenkinsInstance.Spec.ServiceAccount.Name
	}

	serviceaccount := &corev1.ServiceAccount{}
	err = bc.Client.Get(context.TODO(), types.NewNamespacedNameFromString(
		fmt.Sprintf(
			"%s%c%s",
			jenkinsInstance.Namespace,
			types.Separator,
			serviceAccountName)),
		serviceaccount)
	if err != nil {
		glog.Error("RBAC is enabled but could not find specified service account %s", serviceAccountName)
		return nil, err
	}

	binding = &authv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jenkinsInstance.GetName(),
			Namespace: jenkinsInstance.GetNamespace(),
			Labels:    labels,
		},
		Subjects: []authv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: jenkinsInstance.Namespace,
			},
		},
		RoleRef: authv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     jenkinsInstance.Spec.Rbac.Clusterrole,
		},
	}

	err = controllerutil.SetControllerReference(jenkinsInstance, binding, bc.scheme)
	if err != nil {
		glog.Error("Could not set controller reference on JenkinsInstance RoleBinding")
		return nil, err
	}

	err = bc.Client.Create(context.TODO(), binding)
	return binding, err
}

// newServiceAccount creates a service account (if needed) for a JenkinsInstance. It also sets
// the appropriate OwnerReferences on the resources so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newServiceAccount(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) (*corev1.ServiceAccount, error) {
	if jenkinsInstance.Spec.ServiceAccount == nil {
		glog.Warningf("No service account spec present for JenkinsInstance %s", jenkinsInstance.GetName())
		return nil, nil
	}

	serviceAccountName := jenkinsInstance.GetName()
	if jenkinsInstance.Spec.ServiceAccount.Name != "" {
		serviceAccountName = jenkinsInstance.Spec.ServiceAccount.Name
	}
	serviceAccount := &corev1.ServiceAccount{}
	err := bc.Client.Get(
		context.TODO(),
		types.NewNamespacedNameFromString(
			fmt.Sprintf("%s%c%s", jenkinsInstance.GetNamespace(), types.Separator,
				serviceAccountName)),
		serviceAccount)

	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
	} else {
		if !metav1.IsControlledBy(serviceAccount, jenkinsInstance) {
			msg := fmt.Sprintf(MessageResourceExists, serviceAccountName)
			bc.Event(jenkinsInstance, corev1.EventTypeWarning, ErrResourceExists, msg)
			return serviceAccount, fmt.Errorf(msg)
		}

		return serviceAccount, nil
	}

	// create service account
	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

	serviceAccount = &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: jenkinsInstance.GetNamespace(),
			Labels:    labels,
		},

		Secrets:                      jenkinsInstance.Spec.ServiceAccount.Secrets,
		ImagePullSecrets:             jenkinsInstance.Spec.ServiceAccount.ImagePullSecrets,
		AutomountServiceAccountToken: jenkinsInstance.Spec.ServiceAccount.AutomountServiceAccountToken,
	}

	err = controllerutil.SetControllerReference(jenkinsInstance, serviceAccount, bc.scheme)
	if err != nil {
		glog.Errorf("Could not set controller reference on serviceaccount %s", serviceAccountName)
		return nil, err
	}

	err = bc.Client.Create(context.TODO(), serviceAccount)
	return serviceAccount, err
}

// newNetworkPolicy creates a NetworkPolicy (if needed) for a JenkinsInstance. It also sets
// the appropriate OwnerReferences on the resources so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newNetworkPolicy(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) (*netv1.NetworkPolicy, error) {
	if !jenkinsInstance.Spec.NetworkPolicy {
		return nil, nil
	}

	policy := &netv1.NetworkPolicy{}
	err := bc.Client.Get(
		context.TODO(),
		types.NewNamespacedNameFromString(
			fmt.Sprintf("%s%c%s", jenkinsInstance.GetNamespace(), types.Separator,
				jenkinsInstance.GetName())),
		policy)
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

		return policy, nil
	}

	labels := map[string]string{
		"app":        "jenkinsci",
		"controller": jenkinsInstance.GetName(),
		"component":  string(jenkinsInstance.UID),
	}

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

// newDeployment creates a new Deployment for a JenkinsInstance resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the JenkinsInstance resource that 'owns' it.
func (bc *ReconcileJenkinsInstance) newDeployment(jenkinsInstance *jenkinsv1alpha1.JenkinsInstance) (*appsv1.Deployment, error) {
	// Get the deployment with the name specified in JenkinsInstance.spec
	deployment := &appsv1.Deployment{}
	err := bc.Client.Get(
		context.TODO(), types.NewNamespacedNameFromString(
			fmt.Sprintf("%s%c%s", jenkinsInstance.GetNamespace(), types.Separator,
				jenkinsInstance.GetName())),
		deployment)
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

		return deployment, nil
	}

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
	// if pvc name is specified, try to either locate it or create it
	pvcName := jenkinsInstance.GetName()
	if jenkinsInstance.Spec.Storage != nil {
		if jenkinsInstance.Spec.Storage.JobsPvc != "" {
			pvcName = jenkinsInstance.Spec.Storage.JobsPvc
		}
		pvc := &corev1.PersistentVolumeClaim{}
		err = bc.Client.Get(
			context.TODO(),
			types.NewNamespacedNameFromString(
				fmt.Sprintf("%s%c%s", jenkinsInstance.GetNamespace(), types.Separator, pvcName)),
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

	// assign service account if needed
	if jenkinsInstance.Spec.ServiceAccount != nil {
		serviceAccountName := jenkinsInstance.GetName()
		if jenkinsInstance.Spec.ServiceAccount.Name != "" {
			serviceAccountName = jenkinsInstance.Spec.ServiceAccount.Name
		}
		deployment.Spec.Template.Spec.ServiceAccountName = serviceAccountName
	}

	err = controllerutil.SetControllerReference(jenkinsInstance, deployment, bc.scheme)
	if err != nil {
		return nil, err
	}

	err = bc.Client.Create(context.TODO(), deployment)
	return deployment, err
}
