package cloudinventory

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brian1917/illumiocloudapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var outputFileName, cloudCookie, cloudCredentials, cloudTenantId string
var includeLicenses bool

func init() {
	CloudInventoryCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	CloudInventoryCmd.Flags().StringVar(&cloudCookie, "cookie", "", "optionally set a cookie for authenticating to the api.")
	CloudInventoryCmd.Flags().StringVar(&cloudCredentials, "credentials", "", "optionally set cloud creds in client_id:keyformat.")
	CloudInventoryCmd.Flags().BoolVarP(&includeLicenses, "licenses", "l", false, "include license information in the output.")
	CloudInventoryCmd.Flags().StringVarP(&cloudTenantId, "tenant-id", "t", "", "optionally set the tenant id to use.")
	CloudInventoryCmd.MarkFlagsMutuallyExclusive("cookie", "credentials")
	CloudInventoryCmd.MarkFlagRequired("tenant-id")
	CloudInventoryCmd.Flags().SortFlags = false
}

// CloudInventoryCmd runs the label-export command
var CloudInventoryCmd = &cobra.Command{
	Use:   "cloud-inventory",
	Short: "Create a CSV export of the cloud inventory.",
	Long: `
Create a CSV export of the cloud inventory.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		CloudInventory()
	},
}

func CloudInventory() {

	// Create the cloud tenant
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

	apiResponses, err := tenant.GetResources(illumiocloudapi.ResourcesPostRequest{})
	utils.LogMultiAPIRespV2(apiResponses)
	if err != nil {
		utils.LogErrorf("getting resources from cloud - %s", err)
	}

	// If we have no resources, exit
	if len(tenant.Resources) == 0 {
		utils.LogInfo("no resources found.", true)
		return
	}

	// If licensing option is set, get the licenses info
	var segmentationLicenseResourceMap, insightsLicenseResourceMap map[string]float64
	var segmentationLicenseSummaryMap, insightsLicenseSummaryMap map[string]float64
	var resourceToCloudMaps map[string]string
	allUniqueLicensedResourceTypes := make(map[string]bool)

	if includeLicenses {
		segmentationLicenseResourceMap, insightsLicenseResourceMap, resourceToCloudMaps = getMappings()
		segmentationLicenseSummaryMap = make(map[string]float64)
		insightsLicenseSummaryMap = make(map[string]float64)
		for objectType := range insightsLicenseResourceMap {
			allUniqueLicensedResourceTypes[objectType] = true
		}
		for objectType := range segmentationLicenseResourceMap {
			allUniqueLicensedResourceTypes[objectType] = true
		}
	}

	// Output the data to a csv file
	csvData := [][]string{{"cloud", "account_id", "account_name", "resource_id", "resource_name", "object_type", "category", "subcategory"}}
	if includeLicenses {
		csvData[0] = append(csvData[0], "segmentation_licenses", "insights_licenses")
	}

	// Add the resource data
	for _, item := range tenant.Resources {
		row := []string{
			item.Cloud,
			item.AccountID,
			item.AccountName,
			item.Id,
			item.Name,
			item.ObjectType,
			item.Category,
			item.Subcategory,
		}
		if includeLicenses {
			row = append(row, strconv.FormatFloat(segmentationLicenseResourceMap[item.ObjectType], 'f', -1, 64), strconv.FormatFloat(insightsLicenseResourceMap[item.ObjectType], 'f', -1, 64))
			segmentationLicenseSummaryMap[item.ObjectType] = segmentationLicenseSummaryMap[item.ObjectType] + segmentationLicenseResourceMap[item.ObjectType]
			insightsLicenseSummaryMap[item.ObjectType] = insightsLicenseSummaryMap[item.ObjectType] + insightsLicenseResourceMap[item.ObjectType]
		}
		csvData = append(csvData, row)
	}

	if outputFileName == "" {
		outputFileName = utils.FileName("")
	}
	utils.WriteOutput(csvData, nil, outputFileName)
	utils.LogInfof(true, "%d cloud resources exported.", len(csvData)-1)

	// Create the summary license files
	if includeLicenses {
		csvData = [][]string{{"cloud", "object_type", "segmentation_ratio", "segmentation_licenses", "insights_ratio", "insights_licenses"}}
		for objectType := range allUniqueLicensedResourceTypes {
			// Skip zero counts
			if segmentationLicenseSummaryMap[objectType]+insightsLicenseSummaryMap[objectType] == 0 {
				continue
			}
			row := []string{
				resourceToCloudMaps[objectType],
				objectType,
				strconv.FormatFloat(segmentationLicenseResourceMap[objectType], 'f', -1, 64),
				strconv.FormatFloat(segmentationLicenseSummaryMap[objectType], 'f', 1, 64),
				strconv.FormatFloat(insightsLicenseResourceMap[objectType], 'f', -1, 64),
				strconv.FormatFloat(insightsLicenseSummaryMap[objectType], 'f', 1, 64),
			}
			csvData = append(csvData, row)
		}
		summaryFileName := fmt.Sprintf("%s-license-summary.csv", outputFileName[0:len(outputFileName)-4])
		utils.WriteOutput(csvData, nil, summaryFileName)
		utils.LogInfof(true, "license summary exported to %s", summaryFileName)
	}
}
