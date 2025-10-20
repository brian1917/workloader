package vnetpeeringreport

import (
	"github.com/brian1917/illumiocloudapi"
	"github.com/brian1917/workloader/cmd/pcemgmt"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var tenantName, outputFileName, cloudCookie string

func init() {
	VnetPeeringReportCmd.Flags().StringVar(&tenantName, "tenant", "", "tenant name in the pce.yaml file.")
	VnetPeeringReportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	VnetPeeringReportCmd.Flags().StringVar(&cloudCookie, "cookie", "", "optionally set a cookie for authenticating to the api.")
	VnetPeeringReportCmd.Flags().SortFlags = false
}

// VnetPeeringReportCmd runs the label-export command
var VnetPeeringReportCmd = &cobra.Command{
	Use:   "azure-vnet-peering-report",
	Short: "Create a CSV report of Azure VNet peerings.",
	Long: `
Create a CSV export of the cloud inventory.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		VnetPeeringReport()
	},
}

func VnetPeeringReport() {

	// Get the cloud tenant and validate authentication information provided
	tenant, err := pcemgmt.GetTenantByName(tenantName)
	if err != nil {
		utils.LogErrorf("getting tenant by name - %s", err)

	}
	if cloudCookie != "" {
		tenant.Cookie = cloudCookie
	}
	if tenant.Cookie == "" && (tenant.ClientID == "" || tenant.Secret == "") {
		utils.LogErrorf("cannot authenticate to tenant - no client id and secret and no cookie)")
	}

	apiResponses, err := tenant.GetResources(illumiocloudapi.ResourcesPostRequest{ObjectType: []string{"Microsoft.Network/virtualNetworks"}})
	utils.LogMultiAPIRespV2(apiResponses)
	if err != nil {
		utils.LogErrorf("getting resources from cloud - %s", err)
	}

	// Start the csv output
	csvData := [][]string{{"vnet_peering_name", "requester_subscription", "requester_resource_group", "requester_vnet_name", "acceptor_subscription", "acceptor_resource_group", "acceptor_vnet_name"}}

	// Iterate over the vnets
	for _, resource := range tenant.Resources {
		for _, relation := range resource.Relations {
			if relation.ObjectType == "Microsoft.Network/virtualNetworks/virtualNetworkPeerings" {
				csvData = append(csvData, []string{
					illumiocloudapi.GetAzureResourceName(relation.CspID),
					illumiocloudapi.GetAzureSubscription(relation.Properties.RequesterCspID),
					illumiocloudapi.GetAzureResourceGroup(relation.Properties.RequesterCspID),
					illumiocloudapi.GetAzureResourceName(relation.Properties.RequesterCspID),
					illumiocloudapi.GetAzureSubscription(relation.Properties.AcceptorCspID),
					illumiocloudapi.GetAzureResourceGroup(relation.Properties.AcceptorCspID),
					illumiocloudapi.GetAzureResourceName(relation.Properties.AcceptorCspID),
				})
			}
		}
	}

	// If we have no resources, exit
	if len(csvData) == 1 {
		utils.LogInfo("no vnet peerings found.", true)
		return
	}

	if outputFileName == "" {
		outputFileName = utils.FileName("")
	}
	utils.WriteOutput(csvData, nil, outputFileName)
	utils.LogInfof(true, "%d vnet peerings exported.", len(csvData)-1)

}
