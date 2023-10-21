package labelgroupexport

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var useActive, noHref bool
var outputFileName string

func init() {
	LabelGroupExportCmd.Flags().BoolVar(&useActive, "active", false, "Use active policy versus draft. Draft is default.")
	LabelGroupExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	LabelGroupExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	LabelGroupExportCmd.Flags().SortFlags = false

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

		exportLabels()
	},
}

func exportLabels() {

	// Check active/draft
	provisionStatus := "draft"
	if useActive {
		provisionStatus = "active"
	}
	utils.LogInfo(fmt.Sprintf("provision status: %s", provisionStatus), false)

	// Start the data slice with headers
	csvData := [][]string{{HeaderName, HeaderKey, HeaderDescription, HeaderMemberLabels, HeaderMemberLabelGroups, HeaderFullyExpandedMembers, HeaderHref}}
	if noHref {
		csvData = [][]string{{HeaderName, HeaderKey, HeaderDescription, HeaderMemberLabels, HeaderMemberLabelGroups, HeaderFullyExpandedMembers}}

	}
	// GetAllLabelGroups
	apiResps, err := pce.Load(illumioapi.LoadInput{LabelGroups: true, ProvisionStatus: provisionStatus})
	utils.LogMultiAPIResp(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	for _, lg := range pce.LabelGroupsSlice {
		// Find members
		labels := []string{}
		sgs := []string{}

		// Iterate labels
		for _, l := range lg.Labels {
			labels = append(labels, l.Value)
		}
		// Iterate sub groups
		for _, sg := range lg.SubGroups {
			sgs = append(sgs, sg.Name)
		}

		// Expand all subgroups
		fullLabelHrefs := pce.ExpandLabelGroup(lg.Href)
		fullLabels := []string{}
		for _, f := range fullLabelHrefs {
			fullLabels = append(fullLabels, pce.Labels[f].Value)
		}

		// Append to data slice
		if noHref {
			csvData = append(csvData, []string{lg.Name, lg.Key, lg.Description, strings.Join(labels, "; "), strings.Join(sgs, ";"), strings.Join(fullLabels, "; ")})
		} else {
			csvData = append(csvData, []string{lg.Name, lg.Key, lg.Description, strings.Join(labels, "; "), strings.Join(sgs, ";"), strings.Join(fullLabels, "; "), lg.Href})
		}
	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-label-group-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d label-groups exported.", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no label-groups in PCE.", true)
	}

}
