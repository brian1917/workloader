package cwpexport

import (
	"fmt"
	"strconv"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var outputFileName string

func init() {
	ContainerProfileExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

}

// WkldExportCmd runs the workload identifier
var ContainerProfileExportCmd = &cobra.Command{
	Use:   "cwp-export",
	Short: "Create a CSV export of all container workload profiles in the PCE.",
	Long: `
Create a CSV export of all container workload profiles in the PCE.

Only label assignments are supported. Label restrictions will not be imported.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		exportContainerProfiles(pce)
	},
}

func exportContainerProfiles(pce illumioapi.PCE) {
	// Get all container clusters
	a, err := pce.GetContainerClusters(nil)
	utils.LogAPIRespV2("GetContainerClusters", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Iterate each container cluster and get the container profiles
	containerWkldProfiles := []illumioapi.ContainerWorkloadProfile{}
	for _, cc := range pce.ContainerClustersSlice {
		a, err := pce.GetContainerWkldProfiles(nil, cc.ID())
		utils.LogAPIRespV2("GetContainerWkldProfiles", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		for _, p := range pce.ContainerWorkloadProfilesSlice {
			if p.Name != nil && *p.Name == "Default Profile" {
				continue
			}
			p.ClusterName = cc.Name
			containerWkldProfiles = append(containerWkldProfiles, p)

		}
	}

	// Start the export with headers
	data := [][]string{{ContainerCluster, Name, Description, Namespace, Enforcement, Visibility, Managed, Role, App, Env, Loc, Href}}

	for _, cp := range containerWkldProfiles {
		if err != nil {
			utils.LogError(err.Error())
		}
		// Switch visibility levels
		visLevel := ""
		switch illumioapi.PtrToVal(cp.VisibilityLevel) {
		case "flow_summary":
			visLevel = "blocked_allowed"
		case "flow_drops":
			visLevel = "blocked"
		case "flow_off":
			visLevel = "off"
		case "enhanced_data_collection":
			visLevel = "enhanced_data_collection"
		}

		// Ensure we don't try to print nil pointers
		var name, desc string
		if cp.Name != nil {
			name = *cp.Name
		}
		if cp.Description != nil {
			desc = *cp.Description
		}

		// Write output
		data = append(data, []string{cp.ClusterName, name, desc, cp.Namespace, illumioapi.PtrToVal(cp.EnforcementMode), visLevel, strconv.FormatBool(*cp.Managed), cp.GetLabelByKey("role"), cp.GetLabelByKey("app"), cp.GetLabelByKey("env"), cp.GetLabelByKey("loc"), cp.Href})
	}

	// Write the csv
	if len(data) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-container-wkld-profile-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(data, data, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d container workload profiles exported", len(data)-1), true)
	}
}
