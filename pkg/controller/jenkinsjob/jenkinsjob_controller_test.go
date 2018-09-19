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
	jenkinsv1alpha1 "github.com/maratoid/jenkins-operator/pkg/apis/jenkins/v1alpha1"
	"github.com/maratoid/jenkins-operator/pkg/test"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"
)

// TODO: add tlsSecret for ingress testing
const (
	timeout = time.Second * 60
)

var _ = Describe("jenkins job controller", func() {
	var (
		// channel for incoming reconcile requests
		requests chan reconcile.Request
		// stop channel for controller manager
		stop chan struct{}
		// controller k8s client
		c client.Client
		// setup secret
		secret *corev1.Secret
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

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-job-secret",
				Namespace: "default",
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{

				"user":     []byte("dummy"),
				"pass":     []byte("dummy"),
				"apiToken": []byte("dummy"),
			},
		}
		Expect(c.Create(context.TODO(), secret)).To(Succeed())
		Eventually(func() error {
			return c.Get(context.TODO(), types.NamespacedName{Name: "test-job-secret", Namespace: "default"}, secret)
		}, timeout).Should(Succeed())

	})

	AfterEach(func() {
		Expect(c.Delete(context.TODO(), secret)).To(Succeed())
		Eventually(func() error {
			return c.Get(context.TODO(), types.NamespacedName{Name: "test-job-secret", Namespace: "default"}, secret)
		}, timeout).ShouldNot(Succeed())

		time.Sleep(3 * time.Second)
		close(stop)
		test.Teardown()
	})

	Describe("reconciles", func() {
		var (
			jenkinsJob *jenkinsv1alpha1.JenkinsJob

			// request and key
			expectedRequest   reconcile.Request
			standardObjectkey types.NamespacedName

			// jenkins instance
			instance *jenkinsv1alpha1.JenkinsInstance
		)

		BeforeEach(func() {
			instance = &jenkinsv1alpha1.JenkinsInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-xml-jenkins",
					Namespace: "default",
				},
				Spec: jenkinsv1alpha1.JenkinsInstanceSpec{
					Image:       "dummy/dummy:dummy",
					Executors:   1,
					AdminSecret: "dummy",
					Location:    "dummy",
					AdminEmail:  "dummy",
					Service: &jenkinsv1alpha1.ServiceSpec{
						Name:        "dummy",
						ServiceType: "NodePort",
					},
				},
				Status: jenkinsv1alpha1.JenkinsInstanceStatus{
					SetupSecret: "test-job-secret",
					Phase:       "Ready",
				},
			}
			Expect(c.Create(context.TODO(), instance)).To(Succeed())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: "test-job-xml-jenkins", Namespace: "default"}, instance)
			}, timeout).Should(Succeed())

			jenkinsJob = &jenkinsv1alpha1.JenkinsJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-xml",
					Namespace: "default",
				},
				Spec: jenkinsv1alpha1.JenkinsJobSpec{
					JenkinsInstance: "test-job-xml-jenkins",
					JobXml: `
						<?xml version="1.0" encoding="UTF-8"?><project>
        				<actions/>
        				<description>Job created from custom resource from XML</description>
        				<keepDependencies>false</keepDependencies>
        				<properties/>
        				<scm class="hudson.scm.NullSCM"/>
        				<canRoam>true</canRoam>
        				<disabled>false</disabled>
        				<blockBuildWhenDownstreamBuilding>false</blockBuildWhenDownstreamBuilding>
        				<blockBuildWhenUpstreamBuilding>false</blockBuildWhenUpstreamBuilding>
        				<triggers/>
        				<concurrentBuild>false</concurrentBuild>
        				<builders>
            				<hudson.tasks.Shell>
                				<command>echo Hello World with xml!</command>
            				</hudson.tasks.Shell>
        				</builders>
        				<publishers/>
        				<buildWrappers/>
        				<displayName>From custom resource XML</displayName>
    					</project>`,
				},
			}
			standardObjectkey = types.NamespacedName{Name: "test-job-xml", Namespace: "default"}
			expectedRequest = reconcile.Request{NamespacedName: standardObjectkey}
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), instance)).To(Succeed())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: "test-job-xml-jenkins", Namespace: "default"}, instance)
			}, timeout).ShouldNot(Succeed())
		})

		It("Xml job", func() {
			By("creating")
			Expect(c.Create(context.TODO(), jenkinsJob)).To(Succeed())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			By("deleting")
			Expect(c.Delete(context.TODO(), jenkinsJob)).NotTo(HaveOccurred())

			By("cleaning up finalizers")
			existingJob := &jenkinsv1alpha1.JenkinsJob{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, existingJob) }, timeout).
				Should(Succeed())
			Expect(existingJob.Finalizers).NotTo(BeEmpty())

			Eventually(func() error {
				err := c.Get(context.TODO(), standardObjectkey, existingJob)
				if err != nil {
					return err
				}
				existingJobCopy := existingJob.DeepCopy()
				existingJobCopy.Finalizers = []string{}
				return c.Update(context.TODO(), existingJobCopy)
			}, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, jenkinsJob) }, timeout).
				ShouldNot(Succeed())
		})
	})

	Describe("reconciles", func() {
		var (
			jenkinsJob *jenkinsv1alpha1.JenkinsJob

			// request and key
			expectedRequest   reconcile.Request
			standardObjectkey types.NamespacedName

			// jenkins instance
			instance *jenkinsv1alpha1.JenkinsInstance
		)

		BeforeEach(func() {
			instance = &jenkinsv1alpha1.JenkinsInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-dsl-jenkins",
					Namespace: "default",
				},
				Spec: jenkinsv1alpha1.JenkinsInstanceSpec{
					Image:       "dummy/dummy:dummy",
					Executors:   1,
					AdminSecret: "dummy",
					Location:    "dummy",
					AdminEmail:  "dummy",
					Service: &jenkinsv1alpha1.ServiceSpec{
						Name:        "dummy",
						ServiceType: "NodePort",
					},
				},
				Status: jenkinsv1alpha1.JenkinsInstanceStatus{
					SetupSecret: "test-job-secret",
					Phase:       "Ready",
				},
			}
			Expect(c.Create(context.TODO(), instance)).To(Succeed())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: "test-job-dsl-jenkins", Namespace: "default"}, instance)
			}, timeout).Should(Succeed())

			jenkinsJob = &jenkinsv1alpha1.JenkinsJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-dsl",
					Namespace: "default",
				},
				Spec: jenkinsv1alpha1.JenkinsJobSpec{
					JenkinsInstance: "test-job-dsl-jenkins",
					JobDsl: `
						freeStyleJob('test-job-dsl') {
							description('Job created from custom resource with JobDSL')
							displayName('From custom resource DSL')
							steps {
								shell('echo Hello World!')
							}
						}`,
				},
			}
			standardObjectkey = types.NamespacedName{Name: "test-job-dsl", Namespace: "default"}
			expectedRequest = reconcile.Request{NamespacedName: standardObjectkey}
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), instance)).To(Succeed())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: "test-job-dsl-jenkins", Namespace: "default"}, instance)
			}, timeout).ShouldNot(Succeed())
		})

		It("Dsl job", func() {
			By("creating")
			Expect(c.Create(context.TODO(), jenkinsJob)).To(Succeed())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			By("deleting")
			Expect(c.Delete(context.TODO(), jenkinsJob)).NotTo(HaveOccurred())

			By("cleaning up finalizers")
			existingJob := &jenkinsv1alpha1.JenkinsJob{}
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, existingJob) }, timeout).
				Should(Succeed())
			Expect(existingJob.Finalizers).NotTo(BeEmpty())

			Eventually(func() error {
				err := c.Get(context.TODO(), standardObjectkey, existingJob)
				if err != nil {
					return err
				}
				existingJobCopy := existingJob.DeepCopy()
				existingJobCopy.Finalizers = []string{}
				return c.Update(context.TODO(), existingJobCopy)
			}, timeout).Should(Succeed())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, jenkinsJob) }, timeout).
				ShouldNot(Succeed())
		})
	})

	Describe("fails to reconcile", func() {
		var (
			jenkinsJob *jenkinsv1alpha1.JenkinsJob

			// request and key
			expectedRequest   reconcile.Request
			standardObjectkey types.NamespacedName

			// jenkins instance
			instance *jenkinsv1alpha1.JenkinsInstance
		)

		BeforeEach(func() {
			instance = &jenkinsv1alpha1.JenkinsInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-invalid-jenkins",
					Namespace: "default",
				},
				Spec: jenkinsv1alpha1.JenkinsInstanceSpec{
					Image:       "dummy/dummy:dummy",
					Executors:   1,
					AdminSecret: "dummy",
					Location:    "dummy",
					AdminEmail:  "dummy",
					Service: &jenkinsv1alpha1.ServiceSpec{
						Name:        "dummy",
						ServiceType: "NodePort",
					},
				},
				Status: jenkinsv1alpha1.JenkinsInstanceStatus{
					SetupSecret: "test-job-secret",
					Phase:       "Ready",
				},
			}
			Expect(c.Create(context.TODO(), instance)).To(Succeed())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: "test-job-invalid-jenkins", Namespace: "default"}, instance)
			}, timeout).Should(Succeed())

			jenkinsJob = &jenkinsv1alpha1.JenkinsJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-invalid",
					Namespace: "default",
				},
				Spec: jenkinsv1alpha1.JenkinsJobSpec{
					JenkinsInstance: "test-job-invalid-jenkins",
					JobXml: `
						<?xml version="1.0" encoding="UTF-8"?><project>
        				<actions/>
        				<description>Job created from custom resource from XML</description>
        				<keepDependencies>false</keepDependencies>
        				<properties/>
        				<scm class="hudson.scm.NullSCM"/>
        				<canRoam>true</canRoam>
        				<disabled>false</disabled>
        				<blockBuildWhenDownstreamBuilding>false</blockBuildWhenDownstreamBuilding>
        				<blockBuildWhenUpstreamBuilding>false</blockBuildWhenUpstreamBuilding>
        				<triggers/>
        				<concurrentBuild>false</concurrentBuild>
        				<builders>
            				<hudson.tasks.Shell>
                				<command>echo Hello World with xml!</command>
            				</hudson.tasks.Shell>
        				</builders>
        				<publishers/>
        				<buildWrappers/>
        				<displayName>From custom resource XML</displayName>
    					</project>`,
					JobDsl: `
						freeStyleJob('test-job-invalid') {
							description('Job created from custom resource with JobDSL')
							displayName('From custom resource DSL')
							steps {
								shell('echo Hello World!')
							}
						}`,
				},
			}
			standardObjectkey = types.NamespacedName{Name: "test-job-invalid", Namespace: "default"}
			expectedRequest = reconcile.Request{NamespacedName: standardObjectkey}
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), instance)).To(Succeed())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: "test-job-invalid-jenkins", Namespace: "default"}, instance)
			}, timeout).ShouldNot(Succeed())
		})

		It("job with both XML and Dsl", func() {
			By("creating")
			Expect(c.Create(context.TODO(), jenkinsJob)).To(Succeed())
			Eventually(requests, timeout).ShouldNot(Receive(Equal(expectedRequest)))

			By("deleting")
			Expect(c.Delete(context.TODO(), jenkinsJob)).NotTo(HaveOccurred())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, jenkinsJob) }, timeout).
				ShouldNot(Succeed())
		})
	})

	Describe("fails to reconcile", func() {
		var (
			jenkinsJob *jenkinsv1alpha1.JenkinsJob

			// request and key
			expectedRequest   reconcile.Request
			standardObjectkey types.NamespacedName

			// jenkins instance
			instance *jenkinsv1alpha1.JenkinsInstance
		)

		BeforeEach(func() {
			instance = &jenkinsv1alpha1.JenkinsInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-empty-jenkins",
					Namespace: "default",
				},
				Spec: jenkinsv1alpha1.JenkinsInstanceSpec{
					Image:       "dummy/dummy:dummy",
					Executors:   1,
					AdminSecret: "dummy",
					Location:    "dummy",
					AdminEmail:  "dummy",
					Service: &jenkinsv1alpha1.ServiceSpec{
						Name:        "dummy",
						ServiceType: "NodePort",
					},
				},
				Status: jenkinsv1alpha1.JenkinsInstanceStatus{
					SetupSecret: "test-job-secret",
					Phase:       "Ready",
				},
			}
			Expect(c.Create(context.TODO(), instance)).To(Succeed())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: "test-job-empty-jenkins", Namespace: "default"}, instance)
			}, timeout).Should(Succeed())

			jenkinsJob = &jenkinsv1alpha1.JenkinsJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-job-invalid",
					Namespace: "default",
				},
				Spec: jenkinsv1alpha1.JenkinsJobSpec{
					JenkinsInstance: "test-job-empty-jenkins",
				},
			}
			standardObjectkey = types.NamespacedName{Name: "test-job-invalid", Namespace: "default"}
			expectedRequest = reconcile.Request{NamespacedName: standardObjectkey}
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), instance)).To(Succeed())
			Eventually(func() error {
				return c.Get(context.TODO(), types.NamespacedName{Name: "test-job-empty-jenkins", Namespace: "default"}, instance)
			}, timeout).ShouldNot(Succeed())
		})

		It("job without both XML or Dsl", func() {
			By("creating")
			Expect(c.Create(context.TODO(), jenkinsJob)).To(Succeed())
			Eventually(requests, timeout).ShouldNot(Receive(Equal(expectedRequest)))

			By("deleting")
			Expect(c.Delete(context.TODO(), jenkinsJob)).NotTo(HaveOccurred())
			Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, jenkinsJob) }, timeout).
				ShouldNot(Succeed())
		})
	})
})
