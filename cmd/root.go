package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/brian1917/workloader/cmd/compatibility"
	"github.com/brian1917/workloader/cmd/export"
	"github.com/brian1917/workloader/cmd/flowupload"
	"github.com/brian1917/workloader/cmd/hostparser"
	"github.com/brian1917/workloader/cmd/login"
	"github.com/brian1917/workloader/cmd/mode"
	"github.com/brian1917/workloader/cmd/subnet"
	"github.com/brian1917/workloader/cmd/traffic"
	"github.com/brian1917/workloader/cmd/upload"
	"github.com/spf13/cobra"
)

var cfgFile, projectBase, userLicense string

// RootCmd calls the CLI
var RootCmd = &cobra.Command{
	Use:   "workloader",
	Short: "Workloader is a tool that helps discover, label, and manage workloads in an Illumio PCE.",
	Run: func(cmd *cobra.Command, args []string) {
		// Placeholder if we want logic in initial command
	},
}

// Init builds the commands
func init() {

	// Disable sorting
	cobra.EnableCommandSorting = false

	// Available commands
	RootCmd.AddCommand(login.LoginCmd)
	RootCmd.AddCommand(upload.UploadCmd)
	RootCmd.AddCommand(traffic.TrafficCmd)
	RootCmd.AddCommand(subnet.SubnetCmd)
	RootCmd.AddCommand(hostparser.HostnameCmd)
	RootCmd.AddCommand(compatibility.CompatibilityCmd)
	RootCmd.AddCommand(mode.ModeCmd)
	RootCmd.AddCommand(export.ExportCmd)

	// Hidden Commands
	showHidden, _ := strconv.ParseBool(strings.ToLower(os.Getenv("ILLUMIO_ALL")))
	if showHidden {
		RootCmd.AddCommand(flowupload.FlowCmd)
	}

}

// Execute is called by the CLI main function to initiate the Cobra application
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
