package adgroupexport

import (
	"fmt"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/cmd/adgroupimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var outputFileName string
var noHref bool

func init() {
	ADGroupExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	ADGroupExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	ADGroupExportCmd.Flags().SortFlags = false
}

// LabelDimensionExportCmd runs the label-dimension-export command
var ADGroupExportCmd = &cobra.Command{
	Use:   "adgroup-export",
	Short: "Create a CSV export of all AD groups in the PCE.",
	Long: `
	Create a CSV export of all AD groups in the PCE. The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		exportADGroups()
	},
}

func exportADGroups() {

	// Start the data slice with headers
	csvData := [][]string{{adgroupimport.HeaderName, adgroupimport.HeaderSid, adgroupimport.HeaderDescription}}
	if !noHref {
		csvData[0] = append(csvData[0], "href")
	}

	// Get label dimensions
	api, err := pce.GetADUserGroups(nil)
	utils.LogAPIRespV2("GetADUserGroups", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	for _, userGroup := range pce.ConsumingSecurityPrincipalsSlice {
		// Create the csv row entry
		csvRow := make(map[string]string)

		// Populate the entry
		csvRow[adgroupimport.HeaderName] = userGroup.Name
		csvRow[adgroupimport.HeaderSid] = userGroup.SID
		csvRow[adgroupimport.HeaderDescription] = userGroup.Description
		csvRow["href"] = userGroup.Href

		// Append
		newRow := []string{}
		for _, header := range csvData[0] {
			newRow = append(newRow, csvRow[header])
		}
		csvData = append(csvData, newRow)

	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-ad-group-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d ad groups exported.", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no ad groups in PCE.", true)
	}

	utils.LogEndCommand("ad-group-export")

}
