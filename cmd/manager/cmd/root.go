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
	"log"

	"github.com/maratoid/jenkins-operator/pkg/apis"
	"github.com/maratoid/jenkins-operator/pkg/controller"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/maratoid/jenkins-operator/config/crds"
	"io/ioutil"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"strings"
)

var (
	rootCmd = &cobra.Command{
		Use:   "jenkins-operator",
		Short: "Jenkins Operator",
		Long:  `Jenkins CI Operator`,
		Run: func(cmd *cobra.Command, args []string) {
			operator(cmd)
		},
	}
)

func init() {
	viper.SetEnvPrefix("JENKINSOPERATOR")
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	rootCmd.Flags().Bool("install-crds", true, "install the CRDs used by the controller as part of startup")
	viper.BindPFlag("installCrds", rootCmd.Flags().Lookup("install-crds"))
	rootCmd.Flags().AddGoFlagSet(flag.CommandLine)

	// get rid of glog noise (https://github.com/kubernetes/kubernetes/issues/17162)
	flag.CommandLine.Parse([]string{})
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func operator(cmd *cobra.Command) {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		glog.Fatal(err)
	}

	if installCrds := viper.GetBool("installCrds"); installCrds {
		glog.Info("Installing CRDs.")

		tempDir, err := ioutil.TempDir("", "jenkins-operator")
		if err != nil {
			log.Fatal(err)
		}
		defer os.RemoveAll(tempDir) // clean up

		err = crds.RestoreAssets(tempDir, "")
		if err != nil {
			log.Fatal(err)
		}

		_, err = envtest.InstallCRDs(cfg, envtest.CRDInstallOptions{
			Paths: []string{tempDir},
		})
	}

	glog.Info("Registering Components.")

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		log.Fatal(err)
	}

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		glog.Fatal(err)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr); err != nil {
		glog.Fatal(err)
	}

	glog.Info("Starting the Cmd.")

	// Start the Cmd
	glog.Fatal(mgr.Start(signals.SetupSignalHandler()))
}
