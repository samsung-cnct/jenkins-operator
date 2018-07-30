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
	// Import auth/gcp to connect to GKE clusters remotely
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	configlib "github.com/kubernetes-sigs/kubebuilder/pkg/config"
	"github.com/kubernetes-sigs/kubebuilder/pkg/inject/run"
	"github.com/kubernetes-sigs/kubebuilder/pkg/install"
	"github.com/kubernetes-sigs/kubebuilder/pkg/signals"
	extensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	"github.com/maratoid/jenkins-operator/pkg/inject"
	"github.com/maratoid/jenkins-operator/pkg/inject/args"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"


	"strings"
	"fmt"
	"os"
	"github.com/golang/glog"
	"flag"
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

type InstallStrategy struct {
	install.EmptyInstallStrategy
	crds []*extensionsv1beta1.CustomResourceDefinition
}

func init() {
	viper.SetEnvPrefix("JENKINSOPERATOR")
	replacer := strings.NewReplacer("-", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()
	rootCmd.Flags().Bool("install-crds", true, "install the CRDs used by the controller as part of startup")
	rootCmd.Flags().AddGoFlagSet(flag.CommandLine)
	//rootCmd.Flags().Parse()
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func operator(cmd *cobra.Command) {
	// get flags
	installCRDs, _ := cmd.Flags().GetBool("install-crds")

	stopCh := signals.SetupSignalHandler()
	config := configlib.GetConfigOrDie()

	// list assets
	// glog.Fatalf("Assets: %v", bindata.AssetNames())

	if installCRDs {
		if err := install.NewInstaller(config).Install(&InstallStrategy{crds: inject.Injector.CRDs}); err != nil {
			glog.Fatalf("Could not create CRDs: %v", err)
		}
	}

	// Start the controllers
	if err := inject.RunAll(run.RunArguments{Stop: stopCh}, args.CreateInjectArgs(config)); err != nil {
		glog.Fatalf("%v", err)
	}
}

func (s *InstallStrategy) GetCRDs() []*extensionsv1beta1.CustomResourceDefinition {
	return s.crds
}
