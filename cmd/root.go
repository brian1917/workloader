package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/workloader/utils"

	"github.com/brian1917/workloader/cmd/compatibility"
	"github.com/brian1917/workloader/cmd/dupecheck"
	"github.com/brian1917/workloader/cmd/explorer"
	"github.com/brian1917/workloader/cmd/export"
	"github.com/brian1917/workloader/cmd/extract"
	"github.com/brian1917/workloader/cmd/flowsummary"
	"github.com/brian1917/workloader/cmd/flowupload"
	"github.com/brian1917/workloader/cmd/hostparse"
	"github.com/brian1917/workloader/cmd/importer"
	"github.com/brian1917/workloader/cmd/login"
	"github.com/brian1917/workloader/cmd/mislabel"
	"github.com/brian1917/workloader/cmd/mode"
	"github.com/brian1917/workloader/cmd/nic"
	"github.com/brian1917/workloader/cmd/subnet"
	"github.com/brian1917/workloader/cmd/traffic"
	"github.com/brian1917/workloader/cmd/unpair"
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

// All subcommand flags are taken care of in their package's init.
// Root init sets up everything else - all usage templates, Viper, etc.
func init() {

	// Disable sorting
	cobra.EnableCommandSorting = false

	// Set the short description for logout based on OS. We need to do it here before we add it to the RootCmd.
	login.LogoutCmd.Short = utils.LogOutDesc()

	// Login
	RootCmd.AddCommand(login.LoginCmd)
	RootCmd.AddCommand(login.LogoutCmd)

	// Import/Export
	RootCmd.AddCommand(export.ExportCmd)
	RootCmd.AddCommand(importer.ImportCmd)
	RootCmd.AddCommand(flowupload.FlowUpload)

	// Automated Labeling
	RootCmd.AddCommand(traffic.TrafficCmd)
	RootCmd.AddCommand(subnet.SubnetCmd)
	RootCmd.AddCommand(hostparse.HostnameCmd)

	// Workload management
	RootCmd.AddCommand(compatibility.CompatibilityCmd)
	RootCmd.AddCommand(mode.ModeCmd)
	RootCmd.AddCommand(upgrade.UpgradeCmd)
	RootCmd.AddCommand(unpair.UnpairCmd)

	// Reporting
	RootCmd.AddCommand(mislabel.MisLabelCmd)
	RootCmd.AddCommand(dupecheck.DupeCheckCmd)
	RootCmd.AddCommand(flowsummary.FlowSummaryCmd)
	RootCmd.AddCommand(explorer.ExplorerCmd)
	RootCmd.AddCommand(nic.NICCmd)
	RootCmd.AddCommand(versionCmd)

	// Undocumented
	RootCmd.AddCommand(extract.ExtractCmd)

	// Set the usage templates
	for _, c := range RootCmd.Commands() {
		c.SetUsageTemplate(utils.SubCmdTemplate())
	}
	RootCmd.SetUsageTemplate(utils.RootTemplate())
	flowsummary.FlowSummaryCmd.SetUsageTemplate(utils.SRootCmdTemplate())

	// Setup Viper
	viper.SetConfigType("yaml")
	if os.Getenv("ILLUMIO_CONFIG") != "" {
		viper.SetConfigFile(os.Getenv("ILLUMIO_CONFIG"))
	} else {
		viper.SetConfigFile("./pce.yaml")
	}
	viper.ReadInConfig()

	// Persistent flags that will be passed into root command pre-run.
	RootCmd.PersistentFlags().BoolVar(&updatePCE, "update-pce", false, "Command will update the PCE after a single user prompt. Default will just log potentialy changes to workloads.")
	RootCmd.PersistentFlags().BoolVar(&noPrompt, "no-prompt", false, "Remove the user prompt when used with update-pce.")
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

// Version is set by build variable
var Version string

// Commit is the latest commit
var Commit string

//versionCmd returns the version of workloader
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print workloader version.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version %s\r\n", Version)
		fmt.Printf("Previous commit: %s \r\n", Commit)
	},
}
