package virtualserviceexport

import (
	"strconv"

	ia "github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

func init() {}

// VsExportCmd runs the workload identifier
var VsExportCmd = &cobra.Command{
	Use:   "virtualservice-export",
	Short: "Create a CSV export of all virtual services in the PCE.",
	Long: `
Create a CSV export of all virtual services in the PCE.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE.
		pce, err := utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogErrorf("getting pce - %s", err)
		}
		// Run the command
		vsexport(pce)
	},
}

func vsexport(pce ia.PCE) {

	// Load the pce
	apiResps, err := pce.Load(ia.LoadInput{VirtualServices: true, LabelDimensions: true, Labels: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogErrorf("loading pce - %s", err)
	}

	// Start the csv export
	labelKeys := []string{}
	for _, lk := range pce.LabelDimensionsSlice {
		labelKeys = append(labelKeys, lk.Key)
	}

	// Start the csv
	csvData := [][]string{append([]string{"name", "href", "service_ports"}, labelKeys...)}

	// Iterate over the virtual services
	for _, vs := range pce.VirtualServicesSlice {
		row := []string{vs.Name, vs.Href}

		// Service port
		spStr := ""
		for _, sp := range ia.PtrToVal(vs.ServicePorts) {
			if sp.ToPort == 0 {
				spStr = strconv.Itoa(ia.PtrToVal(sp.Port))
			} else {
				spStr = strconv.Itoa(ia.PtrToVal(sp.Port)) + "-" + strconv.Itoa(sp.ToPort)
			}
			if sp.Protocol == 6 {
				spStr += " tcp"
			} else if sp.Protocol == 17 {
				spStr += " udp"
			} else {
				spStr += " " + strconv.Itoa(sp.Protocol)
			}
		}
		row = append(row, spStr)

		// Labels
		for _, lk := range labelKeys {
			row = append(row, vs.GetLabelByKey(lk, pce.Labels).Value)
		}
		csvData = append(csvData, row)
	}

	// Write out the CSV
	utils.WriteOutput(csvData, nil, utils.FileName(""))
}
