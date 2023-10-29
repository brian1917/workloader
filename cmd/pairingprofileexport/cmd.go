package pairingprofileexport

import (
	"fmt"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var outputFileName string

func init() {
	PairingProfileExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	PairingProfileExportCmd.Flags().SortFlags = false

}

// LabelExportCmd runs the label-export command
var PairingProfileExportCmd = &cobra.Command{
	Use:   "pairing-profile-export",
	Short: "Create a CSV export of all pairing profiles in the PCE.",
	Long: `
	Create a CSV export of all pairing profiles in the PCE.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		exportPairingProfiles()
	},
}

func exportPairingProfiles() {

	// Start the data slice with headers
	csvData := [][]string{{"href", "name"}}

	// Get all labels
	pairingProfiles, api, err := pce.GetPairingProfiles(nil)
	utils.LogAPIRespV2("GetPairingProfiles", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	for _, pp := range pairingProfiles {
		csvData = append(csvData, []string{pp.Href, pp.Name})
	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-pairing-profile-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d pairing profiles exported.", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no pairing profiles in PCE.", true)
	}

}
