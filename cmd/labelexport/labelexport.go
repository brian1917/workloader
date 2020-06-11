package labelexport

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug bool
var search, outFormat string

func init() {
	LabelExportCmd.Flags().StringVarP(&search, "search", "s", "", "Only export labels containing a specific string (not case sensitive)")
}

// LabelExportCmd runs the label-export command
var LabelExportCmd = &cobra.Command{
	Use:   "label-export",
	Short: "Create a CSV export of all labels in the PCE.",
	Long: `
Create a CSV export of all labels in the PCE. The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the viper values
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		exportLabels()
	},
}

func exportLabels() {

	// Log command execution
	utils.LogStartCommand("label-export")

	// Start the data slice with headers
	csvData := [][]string{[]string{"href", "key", "value", "ext_dataset", "ext_dataset_ref"}}
	stdOutData := [][]string{[]string{"href", "key", "value"}}

	// GetAllWorkloads
	labels, a, err := pce.GetAllLabels()
	utils.LogAPIResp("GetAllLabels", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Check our search term
	newLabels := []illumioapi.Label{}
	if search != "" {
		for _, l := range labels {
			if strings.Contains(strings.ToLower(l.Value), strings.ToLower(search)) {
				newLabels = append(newLabels, l)
			}
		}
		labels = newLabels
	}

	for _, l := range labels {

		// Skip deleted workloads
		if l.Deleted {
			continue
		}

		// Append to data slice
		csvData = append(csvData, []string{l.Href, l.Key, l.Value, l.ExternalDataSet, l.ExternalDataReference})
		stdOutData = append(stdOutData, []string{l.Href, l.Key, l.Value})
	}

	if len(csvData) > 1 {
		utils.WriteOutput(csvData, stdOutData, fmt.Sprintf("workloader-label-export-%s.csv", time.Now().Format("20060102_150405")))
		fmt.Printf("\r\n%d labels exported.\r\n", len(csvData)-1)
		fmt.Println("Note - the CSV export will include additional columns: external_dataset and external_dataset_ref")
		utils.LogInfo(fmt.Sprintf("label-export exported - %d labels", len(csvData)-1))
	} else {
		// Log command execution for 0 results
		fmt.Println("No labels in PCE.")
		utils.LogInfo("no labels in PCE.")
	}

	utils.LogEndCommand("label-export")

}
