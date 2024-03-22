package azurelabel

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var labelMapping, outputFileName, azureOptions, setLabels, debugFile string
var umwl bool

func init() {
	AzureLabelCmd.Flags().StringVarP(&labelMapping, "mapping", "m", "", "mappings of azure tags to illumio labels. the format is a comma-separated list of azure-tag:illumio-label. For example, \"application:app,type:role\" maps the Azure tag of application to the Illumio app label and the Azure type tag to the Illumio role label.")
	AzureLabelCmd.Flags().BoolVarP(&umwl, "umwl", "u", false, "create and label unmanaged workloads for azure virtual machines that do not have an agent.")
	AzureLabelCmd.Flags().StringVarP(&azureOptions, "options", "o", "", "Azure CLI can be extended using this option.  Anything added after -o inside quotes will be passed as is(e.g \"--region us-west-1\"")
	AzureLabelCmd.Flags().StringVarP(&setLabels, "set-labels", "s", "", "hardcode specific labels for all workloads. The format is a comma-separated list of key:value. For example, \"env:prod,loc:azure\" will set all workloads to have the env label of prod and the location label of azure.")
	AzureLabelCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	AzureLabelCmd.Flags().StringVar(&debugFile, "debug-file", "", "file of json data to use instead of Azure CLI output.")
	AzureLabelCmd.Flags().MarkHidden("debug-file")
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
		pce, err := utils.GetTargetPCEV2(false)
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

	// Iterate through the user provided hard-coded mappigs
	hardCodedKeys := make(map[string]string)
	x = strings.Replace(setLabels, ", ", ",", -1)
	for _, kvPair := range strings.Split(x, ",") {
		split := strings.Split(kvPair, ":")
		if len(split) != 2 {
			utils.LogErrorf("%s is an invalid hard-coded label", kvPair)
		}
		hardCodedKeys[split[0]] = split[1]
	}

	// Set up the csv headers
	csvData := [][]string{{wkldexport.HeaderHostname}}
	for illumioLabel := range illumioAzMap {
		csvData[0] = append(csvData[0], illumioLabel)
	}
	for key := range hardCodedKeys {
		csvData[0] = append(csvData[0], key)
	}
	if umwl {
		csvData[0] = append(csvData[0], wkldexport.HeaderInterfaces, wkldexport.HeaderExternalDataSet, wkldexport.HeaderExternalDataReference)
	}

	// Get the bytes from either the CLI or the debug json file
	var bytes []byte
	if debugFile == "" {
		// Run the AZ command
		cmd := exec.Command("az", "vm", "list")
		if azureOptions != "" {
			cmd.Args = append(cmd.Args, strings.Split(azureOptions, " ")...)
		}
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			utils.LogError(fmt.Sprintf("pipe error - %s", err.Error()))
		}

		// Run the command
		utils.LogInfof(true, "running command: %s", cmd.String())
		if err := cmd.Start(); err != nil {
			utils.LogError(fmt.Sprintf("run error - %s", err.Error()))
		}

		// Read the stout
		bytes, err = io.ReadAll(pipe)
		if err != nil {
			utils.LogError(err.Error())
		}
	} else {
		jsonFile, err := os.Open(debugFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		bytes, err = io.ReadAll(jsonFile)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Unmarshall the JSON
	var azureVMs []AzureVM
	if err := json.Unmarshal(bytes, &azureVMs); err != nil {
		utils.LogErrorf("unmarshaling azure vms with tags - %s", err)
	}

	// Process unmanaged workloads
	if umwl {
		// Run the AZ command
		cmd := exec.Command("az", "vm", "list-ip-addresses")
		if azureOptions != "" {
			cmd.Args = append(cmd.Args, strings.Split(azureOptions, " ")...)
		}
		pipe, err := cmd.StdoutPipe()
		if err != nil {
			utils.LogError(fmt.Sprintf("pipe error - %s", err.Error()))
		}

		// Run the command
		utils.LogInfof(true, "running command: %s", cmd.String())
		if err := cmd.Start(); err != nil {
			utils.LogError(fmt.Sprintf("run error - %s", err.Error()))
		}

		// Read the stout
		bytes, err = io.ReadAll(pipe)
		if err != nil {
			utils.LogError(err.Error())
		}

		var azureVMsWithIp []AzureVirtualMachine
		if err := json.Unmarshal(bytes, &azureVMsWithIp); err != nil {
			utils.LogErrorf("unmarshaling azure vms with ip - %s", err)
		}

		// Create a map
		azureVmIpMap := make(map[string]AzureVM)
		for _, vm := range azureVMsWithIp {
			azureVmIpMap[vm.VirtualMachine.Name] = vm.VirtualMachine
		}

		// Update the list with the tags
		for i, vm := range azureVMs {
			ips := []string{}
			if vm, ok := azureVmIpMap[vm.Name]; ok {
				for i, privateIP := range vm.Network.PrivateIPAddresses {
					ips = append(ips, fmt.Sprintf("umwl-%d:%s", i, privateIP))
				}
				for i, publicIP := range vm.Network.PublicIPAddresses {
					ips = append(ips, fmt.Sprintf("umwl.public-%d:%s", i, publicIP.IPAddress))
				}
			}
			vm.InterfaceList = strings.Join(ips, ";")
			azureVMs[i] = vm
		}
	}

	// Iterate through the azure VMs
	for _, vm := range azureVMs {
		// Start the new csv row

		// Run a check on the name for os profile name and vm name
		if vm.OsProfile != nil && vm.OsProfile.ComputerName != vm.Name {
			utils.LogWarningf(true, "vm.OsProfile.ComputerName (%s) does not match vm.Name (%s)", vm.OsProfile.ComputerName, vm.Name)
		}

		csvRow := []string{vm.Name}
		for _, header := range csvData[0] {
			if header == wkldexport.HeaderHostname || header == wkldexport.HeaderInterfaces || header == wkldexport.HeaderExternalDataSet || header == wkldexport.HeaderExternalDataReference {
				continue
			}
			// Process Hardcoded keys
			if val, ok := hardCodedKeys[header]; ok {
				csvRow = append(csvRow, val)
			} else {
				csvRow = append(csvRow, vm.Tags[illumioAzMap[header]])
			}
		}
		if umwl {
			csvRow = append(csvRow, vm.InterfaceList, "workloader-azure-label", vm.Name)
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
			Umwl:            umwl,
			UpdateWorkloads: true,
			UpdatePCE:       updatePCE,
			NoPrompt:        noPrompt,
			MaxUpdate:       -1,
			MaxCreate:       -1,
		})

	} else {
		utils.LogInfo("no azure vms found", true)
	}

}
