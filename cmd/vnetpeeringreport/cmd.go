package vnetpeeringreport

import (
	"strings"

	"github.com/brian1917/illumiocloudapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var outputFileName, cloudCookie, cloudCredentials, cloudTenantId string

func init() {
	VnetPeeringReportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	VnetPeeringReportCmd.Flags().StringVar(&cloudCookie, "cookie", "", "optionally set a cookie for authenticating to the api.")
	VnetPeeringReportCmd.Flags().StringVar(&cloudCredentials, "credentials", "", "optionally set cloud creds in client_id:keyformat.")
	VnetPeeringReportCmd.Flags().StringVarP(&cloudTenantId, "tenant-id", "t", "", "optionally set the tenant id to use.")
	VnetPeeringReportCmd.MarkFlagsMutuallyExclusive("cookie", "credentials")
	VnetPeeringReportCmd.MarkFlagRequired("tenant-id")
	VnetPeeringReportCmd.Flags().SortFlags = false
}

// VnetPeeringReportCmd runs the label-export command
var VnetPeeringReportCmd = &cobra.Command{
	Use:   "azure-vnet-peering-report",
	Short: "Create a CSV export of the cloud inventory.",
	Long: `
Create a CSV export of the cloud inventory.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		VnetPeeringReport()
	},
}

func VnetPeeringReport() {

	// Get all // Create the cloud tenant
	tenant := illumiocloudapi.Tenant{
		TenantID: cloudTenantId,
	}
	if cloudCredentials != "" {
		creds := strings.Split(cloudCredentials, ":")
		if len(creds) != 2 {
			utils.LogError("invalid cloud-credentials format. expected client_id:key")
		}
		tenant.ClientID = creds[0]
		tenant.Key = creds[1]
	}
	if cloudCookie != "" {
		tenant.Cookie = cloudCookie
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
