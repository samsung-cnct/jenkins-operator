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
	"github.com/spf13/viper"
	"flag"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"strings"
	"github.com/golang/glog"
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
	portNumber := viper.GetInt("port")
	kubeconfigLocation := viper.GetString("kubeconfig")

	glog.Info("Port: %i, kubeconfig: %s", portNumber, kubeconfigLocation)

	
}