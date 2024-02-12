package secprincipalexport

import (
	"fmt"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var outputFileName string
var noHref, groupsOnly bool

const (
	HeaderHref        = "href"
	HeaderName        = "name"
	HeaderDisplayName = "display_name"
	HeaderType        = "type"
)

func init() {
	SecPrincipalExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	SecPrincipalExportCmd.Flags().BoolVar(&groupsOnly, "groups-only", false, "only export groups.")
	SecPrincipalExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	SecPrincipalExportCmd.Flags().SortFlags = false
}

// SecPrincipalExportCmd runs the label-dimension-export command
var SecPrincipalExportCmd = &cobra.Command{
	Use:   "sec-principal-export",
	Short: "Create a csv export of all security principals.",
	Long: `
	Create a csv export of all security principals. The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		exportSecPrincipals(pce)
	},
}

func exportSecPrincipals(pce illumioapi.PCE) {

	// Start the data slice with headers
	csvData := [][]string{{HeaderName, HeaderDisplayName, HeaderType}}
	if !noHref {
		csvData[0] = append(csvData[0], HeaderHref)
	}

	// Get permissions and auth security principals
	apiResps, err := pce.Load(illumioapi.LoadInput{Permissions: true, AuthSecurityPrincipals: true, Labels: true, LabelGroups: true, ProvisionStatus: "active"}, true)
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogErrorf("loading pce - %s", err)
	}

	for _, asp := range pce.AuthSecurityPrincipalsSlices {
		// Skip blank group
		if asp.Name == "" && asp.DisplayName == "" && asp.Type == "group" {
			continue
		}
		// Skip if only groups
		if groupsOnly && asp.Type != "group" {
			continue
		}
		csvRow := make(map[string]string)
		csvRow[HeaderHref] = asp.Href
		csvRow[HeaderDisplayName] = asp.DisplayName
		csvRow[HeaderType] = asp.Type
		csvRow[HeaderName] = asp.Name

		// Append
		newRow := []string{}
		for _, header := range csvData[0] {
			newRow = append(newRow, csvRow[header])
		}
		csvData = append(csvData, newRow)

	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-sec-principals-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d security principals exported", len(csvData)-1), true)
	} else {
		utils.LogInfo("no security principals in PCE.", true)
	}

}
