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
	"context"
	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	"github.com/maratoid/jenkins-operator/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

// TODO: add tlsSecret for ingress testing
const (
	timeout       = time.Second * 60
	name          = "test-jenkins"
	namespace     = "default"
	image         = "jenkins/jenkins:lts"
	envVar        = "TEST_ENV"
	annotation    = "cnct.io/annotation"
	secret        = "test-admin-secret"
	cascSecretN   = "test-casc-secret"
	cascConfigN   = "test-casc-config"
	groovySecretN = "test-groovy-secret"
	serviceType   = "NodePort"
	accessMode    = "ReadWriteOnce"
	storageSize   = "1Gi"
	cascConfigC   = `
jenkins:
  authorizationStrategy:
    loggedInUsersCanDoAnything:
      allowAnonymousRead: false
  crumbIssuer:
    standard:
      excludeClientIPFromCrumb: false
  disableRememberMe: false
  mode: NORMAL
  numExecutors: 1
  primaryView:
    all:
      name: "all"
  quietPeriod: 5
  scmCheckoutRetryCount: 3
  securityRealm:
    local:
      allowsSignup: false
      enableCaptcha: false
      users:
        - id: admin
          password: password
  views:
    - all:
        name: "all"
credentials:
  system:
    domainCredentials:
	- credentials:
      - usernamePassword:
      	  scope:    GLOBAL
          id:       sudo_password
          username: root
          password: ${PASSWORD_CRED}`
	cascConfigC2 = `
jenkins:
  authorizationStrategy:
    loggedInUsersCanDoAnything:
      allowAnonymousRead: false
  crumbIssuer:
    standard:
      excludeClientIPFromCrumb: false
  disableRememberMe: false
  mode: NORMAL
  numExecutors: 3
  primaryView:
    all:
      name: "all"
  quietPeriod: 5
  securityRealm:
    local:
      allowsSignup: false
      enableCaptcha: false
      users:
        - id: admin
          password: password
  views:
    - all:
        name: "all"
credentials:
  system:
    domainCredentials:
    - credentials:
      - usernamePassword:
          scope:    GLOBAL
          id:       sudo_password
          username: root
          password: ${PASSWORD_CRED}
security:
  remotingCLI:
    enabled: false`
)

var _ = Describe("jenkins instance controller", func() {
	var (
		// channel for incoming reconcile requests
		requests chan reconcile.Request
		// stop channel for controller manager
		stop chan struct{}
		// controller k8s client
		c client.Client
	)

	BeforeEach(func() {
		var recFn reconcile.Reconciler

		mgr, err := manager.New(cfg, manager.Options{})
		Expect(err).NotTo(HaveOccurred())
		c = mgr.GetClient()

		recFn, requests = SetupTestReconcile(newReconciler(mgr))
		Expect(add(mgr, recFn)).To(Succeed())

		stop = StartTestManager(mgr)
		test.Setup()
	})

	AfterEach(func() {
		test.Teardown()

		time.Sleep(3 * time.Second)
		close(stop)
	})

	Describe("reconciles", func() {
		var instance *jenkinsv1alpha1.JenkinsInstance
		var expectedRequest reconcile.Request
		var standardObjectkey types.NamespacedName
		var adminSecret *corev1.Secret
		var cascConfig *corev1.ConfigMap
		var cascSecret *corev1.Secret
		var groovySecret *corev1.Secret
		var serviceAccount *corev1.ServiceAccount

		BeforeEach(func() {
			expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: namespace}}
			standardObjectkey = types.NamespacedName{Name: name, Namespace: namespace}

			adminSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secret,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{

					"JENKINS_ADMIN_USER":     []byte("admin"),
					"JENKINS_ADMIN_PASSWORD": []byte("password"),
				},
			}
			Expect(c.Create(context.TODO(), adminSecret)).To(Succeed())

			cascConfig = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cascConfigN,
					Namespace: namespace,
				},
				Data: map[string]string{
					"jenkins": cascConfigC,
				},
			}
			Expect(c.Create(context.TODO(), cascConfig)).To(Succeed())

			cascSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cascSecretN,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{

					"PASSWORD_CRED": []byte("dummy"),
				},
			}
			Expect(c.Create(context.TODO(), cascSecret)).To(Succeed())

			groovySecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      groovySecretN,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{

					"09-config.groovy": []byte("def inputFile = new File(\"/var/jenkins_home/userContent/test\");inputFile.write(\"Hello World !\")"),
				},
			}
			Expect(c.Create(context.TODO(), groovySecret)).To(Succeed())

			serviceAccount = &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
			Expect(c.Create(context.TODO(), serviceAccount)).To(Succeed())

			instance = &jenkinsv1alpha1.JenkinsInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: jenkinsv1alpha1.JenkinsInstanceSpec{
					Image: image,
					Env: map[string]string{
						envVar: envVar,
					},
					CascConfig: &jenkinsv1alpha1.CascConfigSpec{
						ConfigMap: cascConfigN,
					},
					CascSecret:   cascSecretN,
					GroovySecret: groovySecretN,
					Annotations: map[string]string{
						annotation: annotation,
					},
					AdminSecret: secret,
					Service: &jenkinsv1alpha1.ServiceSpec{
						Name:        name,
						ServiceType: serviceType,
						Annotations: map[string]string{
							annotation: annotation,
						},
					},
					Storage: &jenkinsv1alpha1.StorageSpec{
						JobsPvc: name,
						JobsPvcSpec: &corev1.PersistentVolumeClaimSpec{
							AccessModes: []corev1.PersistentVolumeAccessMode{
								accessMode,
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									"storage": resource.MustParse(storageSize),
								},
							},
						},
					},
					ServiceAccount: name,
				},
			}
			Expect(c.Create(context.TODO(), instance)).To(Succeed())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, instance) }, timeout).
				Should(Succeed())
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), instance)).To(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, instance) }, timeout).
				ShouldNot(Succeed())

			// manually delete all objects, since garbage collection is not enabled in test control plane
			deployment := &appsv1.Deployment{}
			Expect(c.Get(context.TODO(), standardObjectkey, deployment)).To(Succeed())
			Expect(c.Delete(context.TODO(), deployment)).To(Succeed())

			setupConfigMap := &corev1.ConfigMap{}
			Expect(c.Get(context.TODO(), standardObjectkey, setupConfigMap)).To(Succeed())
			Expect(c.Delete(context.TODO(), setupConfigMap)).To(Succeed())

			service := &corev1.Service{}
			Expect(c.Get(context.TODO(), standardObjectkey, service)).To(Succeed())
			Expect(c.Delete(context.TODO(), service)).To(Succeed())

			pvc := &corev1.PersistentVolumeClaim{}
			Expect(c.Get(context.TODO(), standardObjectkey, pvc)).To(Succeed())
			Expect(c.Delete(context.TODO(), pvc)).To(Succeed())

			Expect(c.Delete(context.TODO(), adminSecret)).To(Succeed())

			Expect(c.Delete(context.TODO(), cascSecret)).To(Succeed())

			Expect(c.Delete(context.TODO(), cascConfig)).To(Succeed())

			Expect(c.Delete(context.TODO(), groovySecret)).To(Succeed())

			Expect(c.Delete(context.TODO(), serviceAccount)).To(Succeed())
		})

		It("created", func() {
			Context("Secret", func() {
				setupConfigMap := &corev1.ConfigMap{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, setupConfigMap) }, timeout).
						Should(Succeed())
				})
			})

			Context("Service", func() {
				service := &corev1.Service{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, service) }, timeout).
						Should(Succeed())
				})
			})

			Context("PersistentVolumeClaim", func() {
				pvc := &corev1.PersistentVolumeClaim{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, pvc) }, timeout).
						Should(Succeed())
				})
			})

			Context("Deployment", func() {
				deployment := &appsv1.Deployment{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, deployment) }, timeout).
						Should(Succeed())
				})
			})
		})

		It("changes to pre-existing configmap", func() {
			configMap := &corev1.ConfigMap{}
			Expect(c.Get(context.TODO(), types.NamespacedName{Name: cascConfigN, Namespace: namespace}, configMap)).To(Succeed())

			mapCopy := configMap.DeepCopy()
			mapCopy.Data["jenkins.yaml"] = cascConfigC2
			Expect(c.Update(context.TODO(), mapCopy)).To(Succeed())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
		})
	})
})
