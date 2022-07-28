package venexport

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
var outputFileName string

func init() {}

// WkldExportCmd runs the workload identifier
var VenExportCmd = &cobra.Command{
	Use:   "ven-export",
	Short: "Create a CSV export of all VENs in the PCE.",
	Long: `
Create a CSV export of all VENs in the PCE. This file can be used in the ven-import command to update VENs.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		exportVens()
	},
}

func exportVens() {

	// Log command execution
	utils.LogStartCommand("ven-export")

	// Start the data slice with headers
	csvData := [][]string{{HeaderName, HeaderHostname, HeaderDescription, HeaderVenType, HeaderStatus, HeaderVersion, HeaderActivationType, HeaderActivePceFqdn, HeaderTargetPceFqdn, HeaderWorkloads, HeaderContainerCluster, HeaderHref, HeaderUID}}

	// Load the PCE
	apiResps, err := pce.Load(illumioapi.LoadInput{Workloads: true, WorkloadsQueryParameters: map[string]string{"managed": "true"}, VENs: true, ContainerClusters: true, ContainerWorkloads: true})
	utils.LogMultiAPIResp(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	for _, v := range pce.VENsSlice {

		// Get workloads
		workloadHostnames := []string{}
		if v.Workloads != nil {
			for _, w := range *v.Workloads {
				if val, ok := pce.Workloads[w.Href]; ok {
					workloadHostnames = append(workloadHostnames, val.Hostname)
				} else if val, ok := pce.ContainerWorkloads[w.Href]; ok {
					workloadHostnames = append(workloadHostnames, val.Name)
				} else {
					utils.LogError(fmt.Sprintf("%s - %s - associated workload does not exist", v.Href, v.Hostname))
				}
			}
		}

		// Get container cluster
		ccName := ""
		if v.ContainerCluster != nil {
			if val, ok := pce.ContainerClusters[v.ContainerCluster.Href]; ok {
				ccName = val.Name
			}
		}

		csvData = append(csvData, []string{v.Name, v.Hostname, v.Description, v.VenType, v.Status, v.Version, v.ActivationType, v.ActivePceFqdn, v.TargetPceFqdn, strings.Join(workloadHostnames, ";"), ccName, v.Href, v.UID})
	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-ven-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d vens exported", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no vens in PCE.", true)
	}

	utils.LogEndCommand("ven-export")
}
