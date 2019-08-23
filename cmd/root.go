package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/workloader/utils"

	"github.com/brian1917/workloader/cmd/compatibility"
	"github.com/brian1917/workloader/cmd/export"
	"github.com/brian1917/workloader/cmd/flowupload"
	"github.com/brian1917/workloader/cmd/hostparse"
	"github.com/brian1917/workloader/cmd/importer"
	"github.com/brian1917/workloader/cmd/login"
	"github.com/brian1917/workloader/cmd/mislabel"
	"github.com/brian1917/workloader/cmd/mode"
	"github.com/brian1917/workloader/cmd/subnet"
	"github.com/brian1917/workloader/cmd/traffic"
	"github.com/brian1917/workloader/cmd/upgrade"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RootCmd calls the CLI
var RootCmd = &cobra.Command{
	Use: "workloader",
	Long: `
Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		viper.Set("debug", debug)
		viper.Set("update_pce", updatePCE)
		viper.Set("no_prompt", noPrompt)

		//Output format
		outFormat = strings.ToLower(outFormat)
		if outFormat != "both" && outFormat != "stdout" && outFormat != "csv" {
			utils.Log(1, "Invalid out - must be csv, stdout, or both.")
		}
		viper.Set("output_format", outFormat)
		if err := viper.WriteConfig(); err != nil {
			utils.Log(1, err.Error())
		}

	},
	Run: func(cmd *cobra.Command, args []string) {

		cmd.Help()
	},
}

var updatePCE, noPrompt, debug bool
var outFormat string

// Init builds the commands
func init() {

	// Disable sorting
	cobra.EnableCommandSorting = false

	// Available commands
	RootCmd.AddCommand(login.LoginCmd)
	RootCmd.AddCommand(export.ExportCmd)
	RootCmd.AddCommand(importer.ImportCmd)
	RootCmd.AddCommand(traffic.TrafficCmd)
	RootCmd.AddCommand(subnet.SubnetCmd)
	RootCmd.AddCommand(hostparse.HostnameCmd)
	RootCmd.AddCommand(mislabel.MisLabelCmd)
	RootCmd.AddCommand(compatibility.CompatibilityCmd)
	RootCmd.AddCommand(mode.ModeCmd)
	RootCmd.AddCommand(upgrade.UpgradeCmd)
	RootCmd.AddCommand(flowupload.FlowUpload)

	// Setup Viper
	viper.SetConfigType("yaml")

	if os.Getenv("ILLUMIO_CONFIG") != "" {
		viper.SetConfigFile(os.Getenv("ILLUMIO_CONFIG"))
	} else {
		viper.SetConfigFile("./pce.yaml")
	}
	viper.ReadInConfig()

	// Persistent flags that will be passed into root command pre-run.
	RootCmd.PersistentFlags().BoolVar(&updatePCE, "update-pce", false, "Command will prompt user before making changes in the PCE. Default will always just log potentialy changes. Flag has not effect on commands that do not alter PCE.")
	RootCmd.PersistentFlags().BoolVar(&noPrompt, "no-prompt", false, "When used with update-pce it removes the user prompt step and automatically updates the PCE")
	RootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug level logging for troubleshooting.")
	RootCmd.PersistentFlags().StringVar(&outFormat, "out", "both", "Output format. 3 options: csv, stdout, both")

	RootCmd.Flags().SortFlags = false
}

// Execute is called by the CLI main function to initiate the Cobra application
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
