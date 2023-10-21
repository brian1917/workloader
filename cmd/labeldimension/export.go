package labeldimension

import (
	"fmt"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

const (
	HeaderHref            = "href"
	HeaderKey             = "key"
	HeaderDisplayName     = "display_name"
	HeaderFGColor         = "foreground_color"
	HeaderBGColor         = "background_color"
	HeaderInitial         = "initial"
	HeaderDisplayPlural   = "display_name_plural"
	HeaderIcon            = "icon"
	HeaderExternalDataSet = "external_dataset"
	HeaderExternalDataRef = "external_dataref"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var outputFileName string
var noHref bool

func init() {
	LabelDimensionExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	LabelDimensionExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	LabelDimensionExportCmd.Flags().SortFlags = false
}

// LabelDimensionExportCmd runs the label-dimension-export command
var LabelDimensionExportCmd = &cobra.Command{
	Use:   "label-dimension-export",
	Short: "Create a CSV export of all labels dimensions in the PCE.",
	Long: `
Create a CSV export of all label dimensions in the PCE. The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		exportLabelDimensions()
	},
}

func exportLabelDimensions() {

	// Start the data slice with headers
	csvData := [][]string{{HeaderKey, HeaderDisplayName, HeaderFGColor, HeaderBGColor, HeaderInitial, HeaderDisplayPlural, HeaderIcon, HeaderExternalDataSet, HeaderExternalDataRef}}
	if !noHref {
		csvData[0] = append(csvData[0], HeaderHref)
	}

	// Get label dimensions
	api, err := pce.GetLabelDimensions(nil)
	utils.LogAPIRespV2("GetLabelDimensions", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	for _, ld := range pce.LabelDimensionsSlice {
		// Create the csv row entry
		csvRow := make(map[string]string)

		// Populate the entry
		csvRow[HeaderKey] = ld.Key
		csvRow[HeaderDisplayName] = ld.DisplayName
		csvRow[HeaderExternalDataSet] = illumioapi.PtrToVal(ld.ExternalDataSet)
		csvRow[HeaderExternalDataRef] = illumioapi.PtrToVal(ld.ExternalDataReference)
		csvRow[HeaderHref] = ld.Href
		if ld.DisplayInfo != nil {
			csvRow[HeaderFGColor] = ld.DisplayInfo.ForegroundColor
			csvRow[HeaderBGColor] = ld.DisplayInfo.BackgroundColor
			csvRow[HeaderInitial] = ld.DisplayInfo.Initial
			csvRow[HeaderDisplayPlural] = ld.DisplayInfo.DisplayNamePlural
			csvRow[HeaderIcon] = ld.DisplayInfo.Icon
		}

		// Append
		newRow := []string{}
		for _, header := range csvData[0] {
			newRow = append(newRow, csvRow[header])
		}
		csvData = append(csvData, newRow)

	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-label-dimension-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d label dimensions exported.", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no label dimensions in PCE.", true)
	}

}
