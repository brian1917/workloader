package wkldimport

import (
	"fmt"
	"os"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Input is the data structure the FromCSV function expects
type Input struct {
	PCE                                         illumioapi.PCE
	ImportFile                                  string
	RemoveValue                                 string
	RolePrefix, AppPrefix, EnvPrefix, LocPrefix string
	Headers                                     map[string]int
	// MatchIndex                                                                               *int
	MatchString                                                                                               string
	Umwl, KeepAllPCEInterfaces, FQDNtoHostname, AllowEnforcementChanges, UpdateWorkloads, UpdatePCE, NoPrompt bool
	ManagedOnly                                                                                               bool
	UnmanagedOnly                                                                                             bool
	IgnoreCase                                                                                                bool
}

// input is a global variable for the wkld-import command's isntance of Input
var input Input

func init() {

	WkldImportCmd.Flags().BoolVar(&input.Umwl, "umwl", false, "create unmanaged workloads if the host does not exist. Disabled if matching on href.")
	WkldImportCmd.Flags().BoolVar(&input.UpdateWorkloads, "update", true, "update existing workloads. --update=false will only create unmanaged workloads")
	WkldImportCmd.Flags().StringVar(&input.RemoveValue, "remove-value", "", "value in CSV used to remove existing labels. Blank values in the CSV will not change existing. for example, to delete a label an option would be --remove-value DELETE and use DELETE in CSV to indicate where to clear existing labels on a workload.")
	WkldImportCmd.Flags().StringVar(&input.MatchString, "match", "", "match options. blank means to follow workloader default logic. Available options are href, hostname, name, and external_data. The default logic uses href if present, then hostname if present, then name if present. The external_data option uses the unique combinatio of external_data_set and external_data_reference.")
	WkldImportCmd.Flags().BoolVar(&input.IgnoreCase, "ignore-case", false, "ignore case on the match string.")
	WkldImportCmd.Flags().StringVar(&input.RolePrefix, "role-prefix", "", "prefix to add to role labels in CSV.")
	WkldImportCmd.Flags().StringVar(&input.AppPrefix, "app-prefix", "", "prefix to add to app labels in CSV.")
	WkldImportCmd.Flags().StringVar(&input.EnvPrefix, "env-prefix", "", "prefix to add to env labels in CSV.")
	WkldImportCmd.Flags().StringVar(&input.LocPrefix, "loc-prefix", "", "prefix to add to loc labels in CSV.")
	WkldImportCmd.Flags().BoolVar(&input.AllowEnforcementChanges, "allow-enforcement-changes", false, "allow wkld-import to update the enforcement state and visibility levels.")
	WkldImportCmd.Flags().BoolVar(&input.UnmanagedOnly, "unmanaged-only", false, "only label unmanaged workloads in the PCE.")
	WkldImportCmd.Flags().BoolVar(&input.ManagedOnly, "managed-only", false, "only label managed workloads in the PCE.")

	// Hidden flag for use when called from SNOW command
	WkldImportCmd.Flags().BoolVarP(&input.FQDNtoHostname, "fqdn-to-hostname", "f", false, "convert FQDN hostnames reported by Illumio VEN to short hostnames by removing everything after first period (e.g., test.domain.com becomes test).")
	WkldImportCmd.Flags().MarkHidden("fqdn-to-hostname")
	WkldImportCmd.Flags().BoolVarP(&input.KeepAllPCEInterfaces, "keep-all-pce-interfaces", "k", false, "will not delete an interface on an unmanaged workload if it's not in the import. It will only add interfaces to the workload.")
	WkldImportCmd.Flags().MarkHidden("keep-all-pce-interfaces")

	WkldImportCmd.Flags().SortFlags = false

}

// WkldImportCmd runs the upload command
var WkldImportCmd = &cobra.Command{
	Use:   "wkld-import [csv file to import]",
	Short: "Create and assign labels to existing workloads and/or create unmanaged workloads (using --umwl) from a CSV file.",
	Long: `
Create and assign labels to existing workloads and/or create unmanaged workloads (using --umwl) from a CSV file.

The input file requires headers and matches fields to header values. The following headers can be used:
` + "\r\n- " + wkldexport.HeaderHostname + "\r\n" +
		"- " + wkldexport.HeaderName + "\r\n" +
		"- " + wkldexport.HeaderRole + "\r\n" +
		"- " + wkldexport.HeaderApp + "\r\n" +
		"- " + wkldexport.HeaderEnv + "\r\n" +
		"- " + wkldexport.HeaderLoc + "\r\n" +
		"- " + wkldexport.HeaderInterfaces + "\r\n" +
		"- " + wkldexport.HeaderPublicIP + "\r\n" +
		"- " + wkldexport.HeaderMachineAuthenticationID + "\r\n" +
		"- " + wkldexport.HeaderSPN + " (unmanaged workloads for Kerberos only)\r\n" +
		"- " + wkldexport.HeaderPolicyState + " (only with --allow-enforcement-changes flag)\r\n" +
		"- " + wkldexport.HeaderVisibilityState + " (only with --allow-enforcement-changes flag)\r\n" +
		"- " + wkldexport.HeaderDescription + "\r\n" +
		"- " + wkldexport.HeaderOsID + "\r\n" +
		"- " + wkldexport.HeaderOsDetail + "\r\n" +
		"- " + wkldexport.HeaderDataCenter + "\r\n" +
		"- " + wkldexport.HeaderExternalDataSet + "\r\n" +
		"- " + wkldexport.HeaderExternalDataReference + "\r\n" + `
Besides either href, hostname, or name for matching, no field is required.

Other columns are allowed but will be ignored.

Interfaces should be in the format of "192.168.200.20", "192.168.200.20/24", "eth0:192.168.200.20", or "eth0:192.168.200.20/24".
If no interface name is provided with a colon (e.g., "eth0:"), then "umwl:" is used. Multiple interfaces should be separated by a semicolon.

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		var err error
		input.PCE, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		input.ImportFile = args[0]

		// Get the debug value from viper
		input.UpdatePCE = viper.Get("update_pce").(bool)
		input.NoPrompt = viper.Get("no_prompt").(bool)

		// Load the PCE with workloads
		apiResps, err := input.PCE.Load(illumioapi.LoadInput{Workloads: true})
		utils.LogMultiAPIResp(apiResps)
		if err != nil {
			utils.LogError(err.Error())
		}

		ImportWkldsFromCSV(input)
	},
}
