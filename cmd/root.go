package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/workloader/utils"

	"github.com/brian1917/workloader/cmd/azurelabel"
	"github.com/brian1917/workloader/cmd/checkversion"
	"github.com/brian1917/workloader/cmd/compatibility"
	"github.com/brian1917/workloader/cmd/containmentswitch"
	"github.com/brian1917/workloader/cmd/cwpexport"
	"github.com/brian1917/workloader/cmd/cwpimport"
	"github.com/brian1917/workloader/cmd/dagsync"
	"github.com/brian1917/workloader/cmd/deletehrefs"
	"github.com/brian1917/workloader/cmd/deleteunusedlabels"
	"github.com/brian1917/workloader/cmd/dupecheck"
	"github.com/brian1917/workloader/cmd/ebexport"
	"github.com/brian1917/workloader/cmd/ebimport"
	"github.com/brian1917/workloader/cmd/explorer"
	"github.com/brian1917/workloader/cmd/extract"
	"github.com/brian1917/workloader/cmd/flowimport"
	"github.com/brian1917/workloader/cmd/flowsummary"
	"github.com/brian1917/workloader/cmd/getpairingkey"
	"github.com/brian1917/workloader/cmd/hostparse"
	"github.com/brian1917/workloader/cmd/increasevenupdaterate"
	"github.com/brian1917/workloader/cmd/iplexport"
	"github.com/brian1917/workloader/cmd/iplimport"
	"github.com/brian1917/workloader/cmd/iplreplace"
	"github.com/brian1917/workloader/cmd/labeldimension"
	"github.com/brian1917/workloader/cmd/labelexport"
	"github.com/brian1917/workloader/cmd/labelgroupexport"
	"github.com/brian1917/workloader/cmd/labelgroupimport"
	"github.com/brian1917/workloader/cmd/labelimport"
	"github.com/brian1917/workloader/cmd/mislabel"
	"github.com/brian1917/workloader/cmd/mode"
	"github.com/brian1917/workloader/cmd/netscalersync"
	"github.com/brian1917/workloader/cmd/nicexport"
	"github.com/brian1917/workloader/cmd/nicmanage"
	"github.com/brian1917/workloader/cmd/pcemgmt"
	"github.com/brian1917/workloader/cmd/portusage"
	"github.com/brian1917/workloader/cmd/processexport"
	"github.com/brian1917/workloader/cmd/ruleexport"
	"github.com/brian1917/workloader/cmd/ruleimport"
	"github.com/brian1917/workloader/cmd/rulesetexport"
	"github.com/brian1917/workloader/cmd/rulesetimport"
	"github.com/brian1917/workloader/cmd/servicefinder"
	"github.com/brian1917/workloader/cmd/subnet"
	"github.com/brian1917/workloader/cmd/svcexport"
	"github.com/brian1917/workloader/cmd/svcimport"
	"github.com/brian1917/workloader/cmd/templateimport"
	"github.com/brian1917/workloader/cmd/templatelist"
	"github.com/brian1917/workloader/cmd/traffic"
	"github.com/brian1917/workloader/cmd/umwlcleanup"
	"github.com/brian1917/workloader/cmd/unpair"
	"github.com/brian1917/workloader/cmd/unusedumwl"
	"github.com/brian1917/workloader/cmd/upgrade"
	"github.com/brian1917/workloader/cmd/venexport"
	"github.com/brian1917/workloader/cmd/venhealth"
	"github.com/brian1917/workloader/cmd/venimport"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/cmd/wkldiplmapping"
	"github.com/brian1917/workloader/cmd/wkldlabel"
	"github.com/brian1917/workloader/cmd/wkldreplicate"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RootCmd calls the CLI
var RootCmd = &cobra.Command{
	Use: "workloader",
	Long: `
Workloader is a tool that helps manage resources in an Illumio PCE.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		viper.Set("debug", debug)
		viper.Set("update_pce", updatePCE)
		viper.Set("no_prompt", noPrompt)
		viper.Set("verbose", verbose)
		viper.Set("continue_on_error", continueOnError)
		// If the targetPCE is not set in the persistent flag, we clear it from the YAML
		if targetPCE == "" {
			viper.Set("target_pce", "")
		} else {
			viper.Set("target_pce", targetPCE)
		}

		//Output format
		outFormat = strings.ToLower(outFormat)
		if outFormat != "both" && outFormat != "stdout" && outFormat != "csv" {
			utils.LogError("Invalid out - must be csv, stdout, or both.")
		}
		viper.Set("output_format", outFormat)
		if err := viper.WriteConfig(); err != nil {
			utils.LogError(err.Error())
		}

	},
	Run: func(cmd *cobra.Command, args []string) {

		cmd.Help()
	},
}

var updatePCE, continueOnError, noPrompt, debug, verbose bool
var outFormat, targetPCE string

// All subcommand flags are taken care of in their package's init.
// Root init sets up everything else - all usage templates, Viper, etc.
func init() {

	// Disable sorting
	cobra.EnableCommandSorting = false

	// Login
	RootCmd.AddCommand(pcemgmt.AddPCECmd)
	RootCmd.AddCommand(pcemgmt.RemovePCECmd)
	RootCmd.AddCommand(pcemgmt.PCEListCmd)
	RootCmd.AddCommand(pcemgmt.AllPceCmd)
	RootCmd.AddCommand(pcemgmt.TargetPcesCmd)
	RootCmd.AddCommand(pcemgmt.SetProxyCmd)
	RootCmd.AddCommand(pcemgmt.ClearProxyCmd)
	RootCmd.AddCommand(SettingsCmd)

	// Import/Export
	RootCmd.AddCommand(wkldexport.WkldExportCmd)
	RootCmd.AddCommand(wkldimport.WkldImportCmd)
	RootCmd.AddCommand(venexport.VenExportCmd)
	RootCmd.AddCommand(venimport.VenImportCmd)
	RootCmd.AddCommand(iplexport.IplExportCmd)
	RootCmd.AddCommand(iplimport.IplImportCmd)
	RootCmd.AddCommand(iplreplace.IplReplaceCmd)
	RootCmd.AddCommand(labelexport.LabelExportCmd)
	RootCmd.AddCommand(labelimport.LabelImportCmd)
	RootCmd.AddCommand(labelgroupexport.LabelGroupExportCmd)
	RootCmd.AddCommand(labelgroupimport.LabelGroupImportCmd)
	RootCmd.AddCommand(labeldimension.LabelDimensionExportCmd)
	RootCmd.AddCommand(labeldimension.LabelDimensionImportCmd)
	RootCmd.AddCommand(svcimport.SvcImportCmd)
	RootCmd.AddCommand(svcexport.SvcExportCmd)
	RootCmd.AddCommand(rulesetexport.RuleSetExportCmd)
	RootCmd.AddCommand(rulesetimport.RuleSetImportCmd)
	RootCmd.AddCommand(ruleexport.RuleExportCmd)
	RootCmd.AddCommand(ruleimport.RuleImportCmd)
	RootCmd.AddCommand(ebexport.EbExportCmd)
	RootCmd.AddCommand(ebimport.EbImportCmd)
	RootCmd.AddCommand(cwpexport.ContainerProfileExportCmd)
	RootCmd.AddCommand(cwpimport.ContainerProfileImportCmd)
	RootCmd.AddCommand(flowimport.FlowImportCmd)
	RootCmd.AddCommand(templateimport.TemplateImportCmd)
	RootCmd.AddCommand(templatelist.TemplateListCmd)
	// RootCmd.AddCommand(templatecreate.TemplateCreateCmd)

	// Automation
	RootCmd.AddCommand(azurelabel.AzureLabelCmd)
	RootCmd.AddCommand(traffic.TrafficCmd)
	RootCmd.AddCommand(subnet.SubnetCmd)
	RootCmd.AddCommand(hostparse.HostnameCmd)
	RootCmd.AddCommand(dagsync.DAGSyncCmd)

	// Workload management
	RootCmd.AddCommand(compatibility.CompatibilityCmd)
	RootCmd.AddCommand(mode.ModeCmd)
	RootCmd.AddCommand(upgrade.UpgradeCmd)
	RootCmd.AddCommand(getpairingkey.GetPairingKey)
	RootCmd.AddCommand(unpair.UnpairCmd)
	RootCmd.AddCommand(deletehrefs.DeleteCmd)
	RootCmd.AddCommand(umwlcleanup.UMWLCleanUpCmd)
	RootCmd.AddCommand(nicmanage.NICManageCmd)
	RootCmd.AddCommand(containmentswitch.ContainmentSwitchCmd)
	RootCmd.AddCommand(increasevenupdaterate.IncreaseVENUpdateRateCmd)
	RootCmd.AddCommand(wkldreplicate.WkldReplicate)
	RootCmd.AddCommand(wkldlabel.WkldLabelCmd)

	// Label management
	RootCmd.AddCommand(deleteunusedlabels.LabelsDeleteUnusedCmd)

	// Reporting
	RootCmd.AddCommand(ruleexport.RuleUsageCmd)
	RootCmd.AddCommand(portusage.PortUsageCmd)
	RootCmd.AddCommand(mislabel.MisLabelCmd)
	RootCmd.AddCommand(dupecheck.DupeCheckCmd)
	RootCmd.AddCommand(flowsummary.FlowSummaryCmd)
	RootCmd.AddCommand(explorer.ExplorerCmd)
	RootCmd.AddCommand(nicexport.NICExportCmd)
	RootCmd.AddCommand(servicefinder.ServiceFinderCmd)
	RootCmd.AddCommand(processexport.ProcessExportCmd)
	RootCmd.AddCommand(wkldiplmapping.WkldIPLMappingCmd)
	RootCmd.AddCommand(venhealth.VenHealthCmd)
	RootCmd.AddCommand(unusedumwl.UnusedUmwlCmd)

	// Version Commands
	RootCmd.AddCommand(versionCmd)
	RootCmd.AddCommand(checkversion.CheckVersionCmd)

	// NetScaler Sync
	RootCmd.AddCommand(netscalersync.NetScalerSyncCmd)

	// Undocumented
	RootCmd.AddCommand(extract.ExtractCmd)

	// Deprecated
	RootCmd.AddCommand(SetDefaultCmd)

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
	RootCmd.PersistentFlags().BoolVar(&continueOnError, "continue-on-error", false, "Do not not exit on error. Use the workloader error-default command to set default behavior.")
	RootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug level logging for troubleshooting.")
	RootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "When debug is enabled, include the raw API responses. This makes workloader.log increase in size significantly.")
	RootCmd.PersistentFlags().StringVar(&outFormat, "out", "csv", "Output format. 3 options: csv, stdout, both")
	RootCmd.PersistentFlags().StringVar(&targetPCE, "pce", "", "PCE to use in command if not using default PCE.")

	RootCmd.Flags().SortFlags = false

}

// Execute is called by the CLI main function to initiate the Cobra application
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// versionCmd returns the version of workloader
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print workloader version.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version %s\r\n", utils.GetVersion())
		fmt.Printf("Previous commit: %s \r\n", utils.GetCommit())
	},
}

var continueOnErrorDefault, defaultPCE, getAPIBehavior string

func init() {
	SettingsCmd.Flags().StringVar(&defaultPCE, "default-pce", "", "name of pce to be the deafult")
	SettingsCmd.Flags().StringVar(&continueOnErrorDefault, "continue-on-error-default", "", "continue or stop. continue is equivalent to always using the global continue-on-error flag")
	SettingsCmd.Flags().StringVar(&getAPIBehavior, "api-behavior", "", "single or multi. single waits for each get api to the pce to complete before calling the next.")
}

var SettingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Use flags to change workloader settings for default pce, continuing on error default, and multi/single threaded get api call behavior. See flag options below.",
	Run: func(cmd *cobra.Command, args []string) {

		utils.LogStartCommand("settings")

		// Continue on error
		if continueOnErrorDefault != "" {
			if strings.ToLower(continueOnErrorDefault) != "continue" && strings.ToLower(continueOnErrorDefault) != "stop" {
				utils.LogError("continue-on-error-default must be stop or continue")
				os.Exit(1) // Force exit here regardless of what settings are
			}
			viper.Set("continue_on_error_default", strings.ToLower(continueOnErrorDefault))
			if err := viper.WriteConfig(); err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("continue_on_error_default set to %s", strings.ToLower(continueOnErrorDefault)), true)
		}

		// Default PCE
		if defaultPCE != "" {
			if viper.Get(defaultPCE+".fqdn") == nil {
				utils.LogError(fmt.Sprintf("%s pce does not exist.", defaultPCE))
			}
			viper.Set("default_pce_name", defaultPCE)
			if err := viper.WriteConfig(); err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("%s is default pce", defaultPCE), true)
		}

		// Get API behavior
		if getAPIBehavior != "" {
			if strings.ToLower(getAPIBehavior) != "single" && strings.ToLower(getAPIBehavior) != "multi" {
				utils.LogError("api-behavior must be single or muti")
				os.Exit(1) // Force exit here regardless of what settings are
			}
			viper.Set("get_api_behavior", strings.ToLower(getAPIBehavior))
			if err := viper.WriteConfig(); err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("get_api_behavior set to %s", strings.ToLower(getAPIBehavior)), true)

		}

		utils.LogEndCommand("settings")

	},
}

var SetDefaultCmd = &cobra.Command{
	Use:   "set-default",
	Short: "Deprecated command. Use workloader settings --default-pce instead.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Deprecated command. Use workloader settings --default-pce instead.")
	},
}
