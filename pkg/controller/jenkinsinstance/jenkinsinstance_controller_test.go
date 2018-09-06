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
	"k8s.io/api/extensions/v1beta1"
	netv1 "k8s.io/api/networking/v1"
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
	timeout       = time.Second * 30
	name          = "test-jenkins"
	nameWithPVC   = "test-jenkins-pvc"
	namespace     = "default"
	image         = "jenkins/jenkins:lts"
	envVar        = "TEST_ENV"
	plugin        = "kubernetes"
	pluginVersion = "1.12.4"
	annotation    = "cnct.io/annotation"
	secret        = "test-admin-secret"
	secretWithPVC = "test-admin-secret-pvc"
	location      = "http://test-jenkins.cnct.io"
	email         = "admin@cnct.io"
	serviceType   = "NodePort"
	path          = "/"
	accessMode    = "ReadWriteOnce"
	storageSize   = "1Gi"
	executors     = 3
	groovyConfig  = `
def inputFile = new File("/var/jenkins_home/userContent/test")
inputFile.write("Hello World !")`
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

					"user": []byte("admin"),
					"pass": []byte("password"),
				},
			}
			Expect(c.Create(context.TODO(), adminSecret)).NotTo(HaveOccurred())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: secret, Namespace: namespace}, adminSecret)
			}, timeout).Should(Succeed())

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
					Plugins: []jenkinsv1alpha1.PluginSpec{
						{
							Id:      plugin,
							Version: pluginVersion,
						},
					},
					Annotations: map[string]string{
						annotation: annotation,
					},
					Executors:   executors,
					AdminSecret: secret,
					Location:    location,
					AdminEmail:  email,
					Service: &jenkinsv1alpha1.ServiceSpec{
						Name:        name,
						ServiceType: serviceType,
						Annotations: map[string]string{
							annotation: annotation,
						},
					},
					Ingress: &jenkinsv1alpha1.IngressSpec{
						Annotations: map[string]string{
							annotation: annotation,
						},
						Path: path,
					},
					ServiceAccount: &jenkinsv1alpha1.ServiceAccountSpec{
						Name: name,
					},
					NetworkPolicy: true,
					Config:        groovyConfig,
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
			setupSecret := &corev1.Secret{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, setupSecret) }, timeout).
				Should(Succeed())
			service := &corev1.Service{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, service) }, timeout).
				Should(Succeed())
			ingress := &v1beta1.Ingress{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, ingress) }, timeout).
				Should(Succeed())
			serviceAccount := &corev1.ServiceAccount{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, serviceAccount) }, timeout).
				Should(Succeed())
			policy := &netv1.NetworkPolicy{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, policy) }, timeout).
				Should(Succeed())
			pvc := &corev1.PersistentVolumeClaim{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, pvc) }, timeout).
				Should(Succeed())
			deployment := &appsv1.Deployment{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, deployment) }, timeout).
				Should(Succeed())

			Eventually(func() error { return c.Delete(context.TODO(), setupSecret) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, setupSecret) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), service) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, service) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), ingress) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, ingress) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), serviceAccount) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, serviceAccount) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), policy) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, policy) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), pvc) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, pvc) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), deployment) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, deployment) }, timeout).
				ShouldNot(Succeed())

			Expect(c.Delete(context.TODO(), adminSecret)).NotTo(HaveOccurred())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: secret, Namespace: namespace}, adminSecret)
			}, timeout).ShouldNot(Succeed())
		})

		It("created", func() {
			Context("Secret", func() {
				setupSecret := &corev1.Secret{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, setupSecret) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), setupSecret)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, setupSecret) }, timeout).Should(Succeed())
				})
			})

			Context("Service", func() {
				service := &corev1.Service{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, service) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), service)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, service) }, timeout).Should(Succeed())
				})
			})

			Context("Ingress", func() {
				ingress := &v1beta1.Ingress{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, ingress) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), ingress)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, ingress) }, timeout).Should(Succeed())
				})
			})

			Context("ServiceAccount", func() {
				serviceAccount := &corev1.ServiceAccount{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, serviceAccount) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), serviceAccount)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, serviceAccount) }, timeout).Should(Succeed())
				})
			})

			Context("NetworkPolicy", func() {
				policy := &netv1.NetworkPolicy{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, policy) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), policy)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, policy) }, timeout).Should(Succeed())
				})
			})

			Context("PersistentVolumeClaim", func() {
				pvc := &corev1.PersistentVolumeClaim{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, pvc) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), pvc)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, pvc) }, timeout).Should(Succeed())
				})
			})

			Context("Deployment", func() {
				deployment := &appsv1.Deployment{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, deployment) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), deployment)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, deployment) }, timeout).Should(Succeed())
				})
			})
		})

		It("changes to pre-existing secret with created secret", func() {
			preExistingSecret := &corev1.Secret{}
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: secret, Namespace: namespace}, preExistingSecret)
			}, timeout).Should(Succeed())

			secretCopy := preExistingSecret.DeepCopy()
			secretCopy.Data["user"] = []byte("dummy")
			Expect(c.Update(context.TODO(), secretCopy)).To(Succeed())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
		})
	})

	Describe("reconciles, with existing PVC", func() {
		var instance *jenkinsv1alpha1.JenkinsInstance
		var pvc *corev1.PersistentVolumeClaim
		var expectedRequest reconcile.Request
		var standardObjectkey types.NamespacedName
		var adminSecret *corev1.Secret

		BeforeEach(func() {
			expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: nameWithPVC, Namespace: namespace}}
			standardObjectkey = types.NamespacedName{Name: nameWithPVC, Namespace: namespace}

			adminSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretWithPVC,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{

					"user": []byte("admin"),
					"pass": []byte("password"),
				},
			}
			Expect(c.Create(context.TODO(), adminSecret)).NotTo(HaveOccurred())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: secretWithPVC, Namespace: namespace}, adminSecret)
			}, timeout).Should(Succeed())

			pvc = &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameWithPVC,
					Namespace: namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{
						accessMode,
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"storage": resource.MustParse(storageSize),
						},
					},
				},
			}
			Expect(c.Create(context.TODO(), pvc)).To(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, pvc) }, timeout).
				Should(Succeed())

			instance = &jenkinsv1alpha1.JenkinsInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nameWithPVC,
					Namespace: namespace,
				},
				Spec: jenkinsv1alpha1.JenkinsInstanceSpec{
					Image: image,
					Env: map[string]string{
						envVar: envVar,
					},
					Plugins: []jenkinsv1alpha1.PluginSpec{
						{
							Id:      plugin,
							Version: pluginVersion,
						},
					},
					Annotations: map[string]string{
						annotation: annotation,
					},
					Executors:   executors,
					AdminSecret: secretWithPVC,
					Location:    location,
					AdminEmail:  email,
					Ingress: &jenkinsv1alpha1.IngressSpec{
						Annotations: map[string]string{
							annotation: annotation,
						},
						Path: path,
					},
					ServiceAccount: &jenkinsv1alpha1.ServiceAccountSpec{
						Name: nameWithPVC,
					},
					NetworkPolicy: true,
					Config:        groovyConfig,
					Storage: &jenkinsv1alpha1.StorageSpec{
						JobsPvc: nameWithPVC,
					},
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
			setupSecret := &corev1.Secret{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, setupSecret) }, timeout).
				Should(Succeed())
			service := &corev1.Service{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, service) }, timeout).
				Should(Succeed())
			ingress := &v1beta1.Ingress{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, ingress) }, timeout).
				Should(Succeed())
			serviceAccount := &corev1.ServiceAccount{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, serviceAccount) }, timeout).
				Should(Succeed())
			policy := &netv1.NetworkPolicy{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, policy) }, timeout).
				Should(Succeed())
			deployment := &appsv1.Deployment{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, deployment) }, timeout).
				Should(Succeed())

			Eventually(func() error { return c.Delete(context.TODO(), setupSecret) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, setupSecret) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), service) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, service) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), ingress) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, ingress) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), serviceAccount) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, serviceAccount) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), policy) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, policy) }, timeout).
				ShouldNot(Succeed())
			Eventually(func() error { return c.Delete(context.TODO(), deployment) }, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, deployment) }, timeout).
				ShouldNot(Succeed())

			Expect(c.Delete(context.TODO(), adminSecret)).NotTo(HaveOccurred())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: secretWithPVC, Namespace: namespace}, adminSecret)
			}, timeout).ShouldNot(Succeed())

			Expect(c.Delete(context.TODO(), pvc)).NotTo(HaveOccurred())
			Eventually(func() error {
				return c.Get(context.TODO(), standardObjectkey, adminSecret)
			}, timeout).ShouldNot(Succeed())
		})

		It("created", func() {
			Context("Secret", func() {
				setupSecret := &corev1.Secret{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, setupSecret) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), setupSecret)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, setupSecret) }, timeout).Should(Succeed())
				})
			})

			Context("Service", func() {
				service := &corev1.Service{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, service) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), service)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, service) }, timeout).Should(Succeed())
				})
			})

			Context("Ingress", func() {
				ingress := &v1beta1.Ingress{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, ingress) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), ingress)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, ingress) }, timeout).Should(Succeed())
				})
			})

			Context("ServiceAccount", func() {
				serviceAccount := &corev1.ServiceAccount{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, serviceAccount) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), serviceAccount)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, serviceAccount) }, timeout).Should(Succeed())
				})
			})

			Context("NetworkPolicy", func() {
				policy := &netv1.NetworkPolicy{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, policy) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), policy)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, policy) }, timeout).Should(Succeed())
				})
			})

			Context("Deployment", func() {
				deployment := &appsv1.Deployment{}
				When("creating", func() {
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, deployment) }, timeout).
						Should(Succeed())
				})

				When("deleting", func() {
					Expect(c.Delete(context.TODO(), deployment)).To(Succeed())
					Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
					Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, deployment) }, timeout).Should(Succeed())
				})
			})
		})

		It("reconciles changes to pre-existing secret with created secret", func() {
			preExistingSecret := &corev1.Secret{}
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: secretWithPVC, Namespace: namespace}, preExistingSecret)
			}, timeout).Should(Succeed())

			secretCopy := preExistingSecret.DeepCopy()
			secretCopy.Data["user"] = []byte("dummy")
			Expect(c.Update(context.TODO(), secretCopy)).To(Succeed())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
		})
	})
})
