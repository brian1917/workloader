package cmd

import (
	"fmt"
	"os"

	"github.com/brian1917/workloader/cmd/compatibility"
	"github.com/brian1917/workloader/cmd/export"
	"github.com/brian1917/workloader/cmd/hostparse"
	"github.com/brian1917/workloader/cmd/importer"
	"github.com/brian1917/workloader/cmd/login"
	"github.com/brian1917/workloader/cmd/mislabel"
	"github.com/brian1917/workloader/cmd/mode"
	"github.com/brian1917/workloader/cmd/subnet"
	"github.com/brian1917/workloader/cmd/traffic"
	"github.com/spf13/cobra"
)

// RootCmd calls the CLI
var RootCmd = &cobra.Command{
	Use: "workloader",
	Long: `
Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE.`,
	Run: func(cmd *cobra.Command, args []string) {

		cmd.Help()
	},
}

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
	RootCmd.AddCommand(compatibility.CompatibilityCmd)
	RootCmd.AddCommand(mode.ModeCmd)
	RootCmd.AddCommand(mislabel.MisLabelCmd)
}

// Execute is called by the CLI main function to initiate the Cobra application
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
