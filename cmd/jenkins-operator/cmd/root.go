/*
Copyright 2018 Samsung SDS Cloud Native Computing Team.

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

package cmd

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	clientset "github.com/maratoid/jenkins-operator/pkg/client/clientset/versioned"
	informers "github.com/maratoid/jenkins-operator/pkg/client/informers/externalversions"
	jenkinsserver "github.com/maratoid/jenkins-operator/pkg/controllers/jenkins-server"
	"github.com/maratoid/jenkins-operator/pkg/signals"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	rootCmd = &cobra.Command{
		Use:   "jenkins-operator",
		Short: "Jenkins Operator",
		Long:  `Jenkins CI Operator`,
		Run: func(cmd *cobra.Command, args []string) {
			operator()
		},
	}
)

func init() {
	viper.SetEnvPrefix("JENKINSOPERATOR")
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)

	rootCmd.Flags().String("kubeconfig", "", "Location of kubeconfig file")

	viper.BindPFlag("kubeconfig", rootCmd.Flags().Lookup("kubeconfig"))

	viper.AutomaticEnv()
	rootCmd.Flags().AddGoFlagSet(flag.CommandLine)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func operator() {
	// get flags
	kubeconfigLocation := viper.GetString("kubeconfig")

	glog.Info("kubeconfig: %s", kubeconfigLocation)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigLocation)
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}

	exampleClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		glog.Fatalf("Error building example clientset: %s", err.Error())
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Second*30)
	jenkinsOperatorInformerFactory := informers.NewSharedInformerFactory(exampleClient, time.Second*30)

	controller := jenkinsserver.NewController(
		kubeClient,
		exampleClient,
		kubeInformerFactory.Apps().V1().Deployments(),
		jenkinsOperatorInformerFactory.Jenkinsoperator().V1alpha1().JenkinsServers())

	go kubeInformerFactory.Start(stopCh)
	go jenkinsOperatorInformerFactory.Start(stopCh)

	if err = controller.Run(2, stopCh); err != nil {
		glog.Fatalf("Error running controller: %s", err.Error())
	}

}
