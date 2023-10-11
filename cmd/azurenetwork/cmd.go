package azurenetwork

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/iplimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var outputFileName, azureOptions string
var exclVNets, exclSubnets, prefixSubnet, provision bool

func init() {
	AzureNetworkCmd.Flags().StringVarP(&azureOptions, "options", "o", "", "AWS CLI can be extended using this option.  Anything added after -o inside quotes will be passed as is(e.g \"--region us-west-1\"")
	AzureNetworkCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	AzureNetworkCmd.Flags().BoolVarP(&provision, "provision", "p", false, "provision ip lists.")
	AzureNetworkCmd.Flags().BoolVar(&exclSubnets, "exclude-subnets", false, "do not include subnets.")
	AzureNetworkCmd.Flags().BoolVar(&exclVNets, "exclude-vnets", false, "do not include vnets.")
	AzureNetworkCmd.Flags().BoolVar(&prefixSubnet, "prefix-subnet", false, "include the vnet name as a prefix to the subnet.")
	AzureNetworkCmd.Flags().MarkHidden("debug-file")
	AzureNetworkCmd.Flags().SortFlags = false
}

// TrafficCmd runs the workload identifier
var AzureNetworkCmd = &cobra.Command{
	Use:   "azure-network",
	Short: "Import Azure Virtual Networks and subnets as iplists.",
	Long: `
Import Azure Virtual Networks and subnets as iplists .

The command relies on the Azure CLI being installed and authenticated. See here for installing the Azure CLI: https://learn.microsoft.com/en-us/cli/azure/install-azure-cli.

To test the Azure CLI is authenticated, run "az network vnet list" and ensure JSON output is displayed.

A file will be produced that is passed into the ipl-import command. 

It is recommend to run without --update-pce first to the csv produced and what impacts of the ipl-import command.
`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting pce - %s", err.Error()))
		}

		updatePCE := viper.Get("update_pce").(bool)
		noPrompt := viper.Get("no_prompt").(bool)

		AzureNetworks(&pce, provision, updatePCE, noPrompt)
	},
}

func AzureNetworks(pce *illumioapi.PCE, provision, updatePCE, noPrompt bool) {

	// Set up the csv headers
	csvData := [][]string{{iplimport.HeaderName, iplimport.HeaderInclude, iplimport.HeaderExternalDataSet, iplimport.HeaderExternalDataRef}}

	// Get the bytes from either the CLI or the debug json file
	var bytes []byte

	// Get the VNets
	cmd := exec.Command("az", "network", "vnet", "list")
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

	// Unmarshall the JSON
	var azureVNets []AzureNetwork
	if err := json.Unmarshal(bytes, &azureVNets); err != nil {
		utils.LogErrorf("unmarshaling azure vnets - %s", err)
	}

	// Iterate through the azure VMs
	for _, vnet := range azureVNets {
		if vnet.AddressSpace == nil {
			utils.LogWarningf(true, "vnet: %s - nil address space. skipping", vnet.Name)
			continue
		}
		if !exclVNets {
			csvData = append(csvData, []string{vnet.Name, strings.Join(vnet.AddressSpace.AddressPrefixes, ";"), "workloader-azure-network", vnet.Name})
		}
		if !exclSubnets {
			for _, subnet := range illumioapi.PtrToVal(vnet.Subnets) {
				subnetName := subnet.Name
				if prefixSubnet {
					subnetName = fmt.Sprintf("%s-%s", vnet.Name, subnet.Name)
				}
				csvData = append(csvData, []string{subnetName, subnet.AddressPrefix, "workloader-azure-network", fmt.Sprintf("%s-%s", vnet.Name, subnet.Name)})
			}
		}
	}

	// Create the output file and call wkld-import
	if len(azureVNets) > 0 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-azure-network-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d networks exported", len(csvData)-1), true)

		utils.LogInfo("passing output into ipl-import...", true)

		iplimport.ImportIPLists(*pce, outputFileName, updatePCE, noPrompt, false, provision)

	} else {
		utils.LogInfo("no azure networks found", true)
	}

	utils.LogEndCommand("az-network")

}
