package azurelabel

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/Azure/go-autorest/autorest/azure/cli"
	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var labelMapping, outputFileName string

func init() {
	AzureLabelCmd.Flags().StringVarP(&labelMapping, "mapping", "m", "", "mappings of azure tags to illumio labels. the format is a comma-separated list of azure-tag:illumio-label. For example, \"application:app,type:role\" maps the Azure tag of application to the Illumio app label and the Azure type tag to the Illumio role label.")
	AzureLabelCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	AzureLabelCmd.MarkFlagRequired("mapping")
	AzureLabelCmd.Flags().SortFlags = false
}

// TrafficCmd runs the workload identifier
var AzureLabelCmd = &cobra.Command{
	Use:   "azure-label",
	Short: "Import labels for Azure VMs.",
	Long: `
Import labels for Azure VMs.

The command relies on the Azure CLI being installed and authenticated. See here for installing the Azure CLI: https://learn.microsoft.com/en-us/cli/azure/install-azure-cli.

To test the Azure CLI is authenticated, run az vm list and ensure JSON output is displayed.

A file will be produced that is passed into the wkld-import command. 

It is recommend to run without --update-pce first to the csv produced and what impacts of the wkld-import command.
`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCE(false)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting pce - %s", err.Error()))
		}

		updatePCE := viper.Get("update_pce").(bool)
		noPrompt := viper.Get("no_prompt").(bool)

		AzureLabels(labelMapping, &pce, updatePCE, noPrompt)
	},
}

func AzureLabels(labelMapping string, pce *illumioapi.PCE, updatePCE, noPrompt bool) {

	// Create the lookup map where the illumio label is the key and the azure key is the value
	illumioAzMap := make(map[string]string)

	// Iterate through the user provider mappings
	x := strings.Replace(labelMapping, ", ", ",", -1)
	for _, lm := range strings.Split(x, ",") {
		s := strings.Split(lm, ":")
		if len(s) != 2 {
			utils.LogError(fmt.Sprintf("%s is an invalid mapping", lm))
		}
		illumioAzMap[s[1]] = s[0]
	}

	// Set up the csv headers
	csvData := [][]string{{"hostname"}}
	for illumioLabel := range illumioAzMap {
		csvData[0] = append(csvData[0], illumioLabel)
	}

	// Get the location of the Azure
	cmd := cli.GetAzureCLICommand()

	// Build the VM list command with a pipe
	cmd.Args = []string{cmd.Path, "vm", "list"}
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		utils.LogError(fmt.Sprintf("pipe error - %s", err.Error()))
	}

	// Run the command
	if err := cmd.Start(); err != nil {
		utils.LogError(fmt.Sprintf("run error - %s", err.Error()))
	}

	// Read the stout
	bytes, err := io.ReadAll(pipe)
	if err != nil {
		log.Fatal(err)
	}

	// Unmarshall the JSON
	var azureVMs []AzureVM
	json.Unmarshal(bytes, &azureVMs)

	// Iterate through the azure VMs
	for _, vm := range azureVMs {
		// Start the new csv row
		csvRow := []string{vm.OsProfile.ComputerName}
		for _, header := range csvData[0] {
			// Process hostname
			if header == "hostname" {
				continue
			}
			csvRow = append(csvRow, vm.Tags[illumioAzMap[header]])
		}
		csvData = append(csvData, csvRow)
	}

	// Create the output file and call wkld-import
	if len(azureVMs) > 0 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-azure-label-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d azure vms with label data exported", len(csvData)-1), true)

		utils.LogInfo("passing output into wkld-import...", true)

		wkldimport.ImportWkldsFromCSV(wkldimport.Input{
			PCE:             *pce,
			ImportFile:      outputFileName,
			RemoveValue:     "azure-label-delete",
			Umwl:            false,
			UpdateWorkloads: true,
			UpdatePCE:       updatePCE,
			NoPrompt:        noPrompt,
		})

	} else {
		utils.LogInfo("no azure vms found", true)
	}

	utils.LogEndCommand("az-label")

}
