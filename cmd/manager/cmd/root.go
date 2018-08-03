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
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"os"
	"strings"
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
	viper.AutomaticEnv()
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	viper.BindPFlags(pflag.CommandLine)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func operator() {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		glog.Fatal(err)
	}

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{})
	if err != nil {
		log.Fatal(err)
	}

	glog.Info("Registering Components.")

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
