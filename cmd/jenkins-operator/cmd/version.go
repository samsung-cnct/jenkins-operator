package cmd

import (
"encoding/json"
"flag"
"fmt"
"github.com/samsung-cnct/cma-operator/pkg/version"
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
