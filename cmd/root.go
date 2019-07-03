package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/brian1917/workloader/cmd/hostname"
	"github.com/brian1917/workloader/cmd/mode"
	"github.com/brian1917/workloader/cmd/subnet"
	"github.com/brian1917/workloader/cmd/traffic"
	"github.com/spf13/cobra"
)

var cfgFile, projectBase, userLicense string

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
	RootCmd.AddCommand(loginCmd)
	RootCmd.AddCommand(csvCmd)
	RootCmd.AddCommand(traffic.TrafficCmd)
	RootCmd.AddCommand(subnet.SubnetCmd)
	RootCmd.AddCommand(hostname.HostnameCmd)
	RootCmd.AddCommand(compatibilityCmd)
	RootCmd.AddCommand(mode.ModeCmd)

	// Hidden Commands
	showHidden, _ := strconv.ParseBool(strings.ToLower(os.Getenv("ILLUMIO_ALL")))
	if showHidden {
		RootCmd.AddCommand(flowCmd)
	}

}

// Execute is called by the CLI main function to initiate the Cobra application
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
