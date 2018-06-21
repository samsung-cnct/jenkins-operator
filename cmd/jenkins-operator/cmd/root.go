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