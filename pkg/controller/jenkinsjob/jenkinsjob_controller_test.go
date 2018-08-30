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
	timeout     = time.Second * 15
	xmlName     = "xml-jenkins-job"
	dslName     = "dsl-jenkins-job"
	failingName = "fail-jenkins-job"
	namespace   = "default"
)

var _ = Describe("jenkins job controller", func() {
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

	})

	AfterEach(func() {
		time.Sleep(3 * time.Second)
		close(stop)
	})

	Describe("when creating a new JenkinsJob with XML source", func() {
		var expectedRequest reconcile.Request
		var jenkinsJob *jenkinsv1alpha1.JenkinsJob
		var standardObjectkey types.NamespacedName
		var instance *jenkinsv1alpha1.JenkinsInstance
		var secret *corev1.Secret

		BeforeEach(func() {
			standardObjectkey = types.NamespacedName{Name: xmlName, Namespace: namespace}
			expectedRequest = reconcile.Request{NamespacedName: standardObjectkey}
			instance = &jenkinsv1alpha1.JenkinsInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      xmlName,
					Namespace: namespace,
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
					SetupSecret: xmlName,
					Api:         "http://127.0.0.1:12345",
					Phase:       "Ready",
				},
			}
			Expect(c.Create(context.TODO(), instance)).To(Succeed())

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      xmlName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{

					"user":     []byte("dummy"),
					"pass":     []byte("dummy"),
					"apiToken": []byte("dummy"),
				},
			}
			Expect(c.Create(context.TODO(), secret)).To(Succeed())
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), instance)).To(Succeed())
			Expect(c.Delete(context.TODO(), secret)).To(Succeed())
		})

		It("reconciles", func() {
			jenkinsJob = &jenkinsv1alpha1.JenkinsJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      xmlName,
					Namespace: namespace,
				},
				Spec: jenkinsv1alpha1.JenkinsJobSpec{
					JenkinsInstance: xmlName,
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
			Expect(c.Create(context.TODO(), jenkinsJob)).To(Succeed())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			When("deleting the jenkins job", func() {
				err := c.Delete(context.TODO(), jenkinsJob)
				Expect(err).NotTo(HaveOccurred())

				jenkinsJob = &jenkinsv1alpha1.JenkinsJob{}
				Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, jenkinsJob) }, timeout).
					Should(Succeed())
				Expect(jenkinsJob.Finalizers).NotTo(BeEmpty())
			})
		})
	})

	Describe("when creating a new JenkinsJob with JobDSL source", func() {
		var expectedRequest reconcile.Request
		var jenkinsJob *jenkinsv1alpha1.JenkinsJob
		var standardObjectkey types.NamespacedName
		var instance *jenkinsv1alpha1.JenkinsInstance
		var secret *corev1.Secret

		BeforeEach(func() {
			standardObjectkey = types.NamespacedName{Name: dslName, Namespace: namespace}
			expectedRequest = reconcile.Request{NamespacedName: standardObjectkey}
			instance = &jenkinsv1alpha1.JenkinsInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dslName,
					Namespace: namespace,
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
					SetupSecret: dslName,
					Api:         "http://127.0.0.1:12345",
					Phase:       "Ready",
				},
			}
			Expect(c.Create(context.TODO(), instance)).To(Succeed())

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dslName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{

					"user":     []byte("dummy"),
					"pass":     []byte("dummy"),
					"apiToken": []byte("dummy"),
				},
			}
			Expect(c.Create(context.TODO(), secret)).To(Succeed())
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), instance)).To(Succeed())
			Expect(c.Delete(context.TODO(), secret)).To(Succeed())
		})

		It("reconciles", func() {
			jenkinsJob = &jenkinsv1alpha1.JenkinsJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dslName,
					Namespace: namespace,
				},
				Spec: jenkinsv1alpha1.JenkinsJobSpec{
					JenkinsInstance: dslName,
					JobDsl: `
						freeStyleJob('jenkinsjob-sample-dsl') {
							description('Job created from custom resource with JobDSL')
							displayName('From custom resource DSL')
							steps {
								shell('echo Hello World!')
							}
						}`,
				},
			}
			Expect(c.Create(context.TODO(), jenkinsJob)).To(Succeed())
			Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))

			When("deleting the jenkins job", func() {
				err := c.Delete(context.TODO(), jenkinsJob)
				Expect(err).NotTo(HaveOccurred())

				jenkinsJob = &jenkinsv1alpha1.JenkinsJob{}
				Eventually(func() error { return c.Get(context.TODO(), standardObjectkey, jenkinsJob) }, timeout).
					Should(Succeed())
				Expect(jenkinsJob.Finalizers).NotTo(BeEmpty())
			})
		})

	})

	Describe("when creating a new JenkinsJob with JobDSL and XML source", func() {
		var expectedRequest reconcile.Request
		var jenkinsJob *jenkinsv1alpha1.JenkinsJob
		var standardObjectkey types.NamespacedName
		var instance *jenkinsv1alpha1.JenkinsInstance
		var secret *corev1.Secret

		BeforeEach(func() {
			standardObjectkey = types.NamespacedName{Name: failingName, Namespace: namespace}
			expectedRequest = reconcile.Request{NamespacedName: standardObjectkey}
			instance = &jenkinsv1alpha1.JenkinsInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      failingName,
					Namespace: namespace,
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
					SetupSecret: failingName,
					Api:         "http://127.0.0.1:12345",
					Phase:       "Ready",
				},
			}
			Expect(c.Create(context.TODO(), instance)).To(Succeed())

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      failingName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{

					"user":     []byte("dummy"),
					"pass":     []byte("dummy"),
					"apiToken": []byte("dummy"),
				},
			}
			Expect(c.Create(context.TODO(), secret)).To(Succeed())
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), instance)).To(Succeed())
			Expect(c.Delete(context.TODO(), secret)).To(Succeed())
		})

		It("does not reconcile", func() {
			jenkinsJob = &jenkinsv1alpha1.JenkinsJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      failingName,
					Namespace: namespace,
				},
				Spec: jenkinsv1alpha1.JenkinsJobSpec{
					JenkinsInstance: failingName,
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
						freeStyleJob('jenkinsjob-sample-dsl') {
							description('Job created from custom resource with JobDSL')
							displayName('From custom resource DSL')
							steps {
								shell('echo Hello World!')
							}
						}`,
				},
			}
			Expect(c.Create(context.TODO(), jenkinsJob)).To(Succeed())
			Eventually(requests, timeout).ShouldNot(Receive(Equal(expectedRequest)))

			When("deleting the jenkins job", func() {
				err := c.Delete(context.TODO(), jenkinsJob)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("when creating a new JenkinsJob without JobDSL or XML source", func() {
		var expectedRequest reconcile.Request
		var jenkinsJob *jenkinsv1alpha1.JenkinsJob
		var standardObjectkey types.NamespacedName
		var instance *jenkinsv1alpha1.JenkinsInstance
		var secret *corev1.Secret

		BeforeEach(func() {
			standardObjectkey = types.NamespacedName{Name: failingName, Namespace: namespace}
			expectedRequest = reconcile.Request{NamespacedName: standardObjectkey}
			instance = &jenkinsv1alpha1.JenkinsInstance{
				ObjectMeta: metav1.ObjectMeta{
					Name:      failingName,
					Namespace: namespace,
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
					SetupSecret: failingName,
					Api:         "http://127.0.0.1:12345",
					Phase:       "Ready",
				},
			}
			Expect(c.Create(context.TODO(), instance)).To(Succeed())

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      failingName,
					Namespace: namespace,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{

					"user":     []byte("dummy"),
					"pass":     []byte("dummy"),
					"apiToken": []byte("dummy"),
				},
			}
			Expect(c.Create(context.TODO(), secret)).To(Succeed())
		})

		AfterEach(func() {
			Expect(c.Delete(context.TODO(), instance)).To(Succeed())
			Expect(c.Delete(context.TODO(), secret)).To(Succeed())
		})

		It("does not reconcile", func() {
			jenkinsJob = &jenkinsv1alpha1.JenkinsJob{
				ObjectMeta: metav1.ObjectMeta{
					Name:      failingName,
					Namespace: namespace,
				},
				Spec: jenkinsv1alpha1.JenkinsJobSpec{
					JenkinsInstance: failingName,
				},
			}
			Expect(c.Create(context.TODO(), jenkinsJob)).To(Succeed())
			Eventually(requests, timeout).ShouldNot(Receive(Equal(expectedRequest)))

			When("deleting the jenkins job", func() {
				err := c.Delete(context.TODO(), jenkinsJob)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
