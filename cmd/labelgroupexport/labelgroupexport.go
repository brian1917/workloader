package labelgroupexport

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
var debug, useActive bool
var search, outFormat string

func init() {
	LabelGroupExportCmd.Flags().BoolVar(&useActive, "active", false, "Use active policy versus draft. Draft is default.")
}

// LabelGroupExportCmd runs the label-export command
var LabelGroupExportCmd = &cobra.Command{
	Use:   "labelgroup-export",
	Short: "Create a CSV export of all label groups in the PCE.",
	Long: `
Create a CSV export of all label groups in the PCE. The update-pce and --no-prompt flags are ignored for this command.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
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
	utils.LogStartCommand("labelgroup-export")

	// Check active/draft
	provisionStatus := "draft"
	if useActive {
		provisionStatus = "active"
	}
	utils.LogInfo(fmt.Sprintf("provision status: %s", provisionStatus), false)

	// Start the data slice with headers
	csvData := [][]string{[]string{"href", "name", "key", "members", "fully_exapnded_members"}}

	// GetAllLabelGroups
	lgs, a, err := pce.GetAllLabelGroups(provisionStatus)
	utils.LogAPIResp("GetAllLabelGroups", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Populuate the LabelGroupMap
	pce.LabelGroupMapH = make(map[string]illumioapi.LabelGroup)
	for _, lg := range lgs {
		pce.LabelGroupMapH[lg.Href] = lg
	}

	// Get labelMaps
	pce.GetLabelMaps()

	for _, lg := range lgs {
		// Find members
		labels := []string{}
		sgs := []string{}

		// Iterate labels
		for _, l := range lg.Labels {
			labels = append(labels, l.Value)
		}
		// Iterate sub groups
		for _, sg := range lg.SubGroups {
			sgs = append(sgs, fmt.Sprintf("%s (label_group)", sg.Name))
		}

		// Expand all subgroupspce.LabelGroupMapH := make(map[string]illumioapi.LabelGroup)
		fullLabelHrefs := pce.ExpandLabelGroup(lg.Href)
		fullLabels := []string{}
		for _, f := range fullLabelHrefs {
			fullLabels = append(fullLabels, pce.LabelMapH[f].Value)
		}

		// Append to data slice
		csvData = append(csvData, []string{lg.Href, lg.Name, lg.Key, strings.Join(append(labels, sgs...), "; "), strings.Join(fullLabels, "; ")})
	}

	if len(csvData) > 1 {
		utils.WriteOutput(csvData, csvData, fmt.Sprintf("workloader-label-group-export-%s.csv", time.Now().Format("20060102_150405")))
		utils.LogInfo(fmt.Sprintf("%d label-groups exported.", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no label-groups in PCE.", true)
	}

	utils.LogEndCommand("labelgroup-export")

}
