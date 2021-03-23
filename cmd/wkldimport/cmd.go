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
	PCE                                                             illumioapi.PCE
	ImportFile                                                      string
	RemoveValue                                                     string
	Headers                                                         map[string]int
	MatchIndex                                                      *int
	Umwl, KeepAllPCEInterfaces, FQDNtoHostname, UpdatePCE, NoPrompt bool
}

// input is a global variable for the wkld-import command's isntance of Input
var input Input
var matchIndex int

func init() {

	WkldImportCmd.Flags().BoolVar(&input.Umwl, "umwl", false, "Create unmanaged workloads if the host does not exist. Disabled if matching on href.")
	WkldImportCmd.Flags().StringVar(&input.RemoveValue, "remove-value", "", "Value in CSV used to remove existing labels. Blank values in the CSV will not change existing. If you want to delete a label do something like --remove-value DELETE and use DELETE in CSV to indicate where to clear existing labels on a workload.")
	WkldImportCmd.Flags().IntVar(&matchIndex, "match", -1, "Column number to override selected column to match workloads. -1 uses default workloader logic.")
	if matchIndex == -1 {
		input.MatchIndex = nil
	}

	// Hidden flag for use when called from SNOW command
	WkldImportCmd.Flags().BoolVarP(&input.FQDNtoHostname, "fqdn-to-hostname", "f", false, "Convert FQDN hostnames reported by Illumio VEN to short hostnames by removing everything after first period (e.g., test.domain.com becomes test).")
	WkldImportCmd.Flags().MarkHidden("fqdn-to-hostname")
	WkldImportCmd.Flags().BoolVarP(&input.KeepAllPCEInterfaces, "keep-all-pce-interfaces", "k", false, "Will not delete an interface on an unmanaged workload if it's not in the import. It will only add interfaces to the workload.")
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
		"- " + wkldexport.HeaderDescription + "\r\n" +
		"- " + wkldexport.HeaderOsID + "\r\n" +
		"- " + wkldexport.HeaderOsDetail + "\r\n" +
		"- " + wkldexport.HeaderDataCenter + "\r\n" +
		"- " + wkldexport.HeaderExternalDataSet + "\r\n" +
		"- " + wkldexport.HeaderExternalDataReference + "\r\n" + `
Besides either href or hostname for matching, no field is required.
For example, to only update the location field you can provide just two columns: href and loc (or hostname and loc). All other workload properties will be preserved.
Similarily, if to only update labels, you do not need to include an interface, name, description, etc.

If you need to override the header to to field matching you can specify the column number with any flag.
For example --name 2 will force workloader to use the second column in the CSV as the name field, regardless of what the header value is.

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
		if err := input.PCE.Load(illumioapi.LoadInput{Workloads: true}); err != nil {
			utils.LogError(err.Error())
		}

		ImportWkldsFromCSV(input)
	},
}
