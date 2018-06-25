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
	"encoding/json"
	"flag"
	"fmt"
	"github.com/maratoid/jenkins-operator/pkg/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var VersionOutput string

func init() {
	rootCmd.AddCommand(generateVersionCmd())
}

func generateVersionCmd() *cobra.Command {
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Returns version information",
		Long:  `Find out the version, git commit, etc of the build`,
		Run: func(cmd *cobra.Command, args []string) {
			printVersion()
		},
	}

	versionCmd.Flags().StringVarP(&VersionOutput, "output", "o", "text", "text or json")
	viper.BindPFlag("versionoutput", versionCmd.Flags().Lookup("output"))
	versionCmd.Flags().AddGoFlagSet(flag.CommandLine)

	return versionCmd
}

func printVersion() {
	info := version.Get()

	output := viper.GetString("versionoutput")

	switch output {
	case "json":
		jsonOutput, _ := json.Marshal(info)
		fmt.Printf("%s\n", jsonOutput)
	default:
		fmt.Printf("Version Information:\n")
		fmt.Printf("\tGit Data:\n")
		fmt.Printf("\t\tTagged Version:\t%s\n", info.GitVersion)
		fmt.Printf("\t\tHash:\t\t%s\n", info.GitCommit)
		fmt.Printf("\t\tTree State:\t%s\n", info.GitTreeState)
		fmt.Printf("\tBuild Data:\n")
		fmt.Printf("\t\tBuild Date:\t%s\n", info.BuildDate)
		fmt.Printf("\t\tGo Version:\t%s\n", info.GoVersion)
		fmt.Printf("\t\tCompiler:\t%s\n", info.Compiler)
		fmt.Printf("\t\tPlatform:\t%s\n\n", info.Platform)
	}

}
