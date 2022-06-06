package labelexport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/workloader/cmd/labelimport"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var search, outputFileName string
var noHref bool

func init() {
	LabelExportCmd.Flags().StringVarP(&search, "search", "s", "", "Only export labels containing a specific string (not case sensitive)")
	LabelExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	LabelExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	LabelExportCmd.Flags().SortFlags = false

}

// LabelExportCmd runs the label-export command
var LabelExportCmd = &cobra.Command{
	Use:   "label-export",
	Short: "Create a CSV export of all labels in the PCE.",
	Long: `
Create a CSV export of all labels in the PCE. The update-pce and --no-prompt flags are ignored for this command.`,
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

	// Log command execution
	utils.LogStartCommand("label-export")

	// Start the data slice with headers
	csvData := [][]string{{labelimport.HeaderHref, labelimport.HeaderKey, labelimport.HeaderValue, labelimport.HeaderExtDataSet, labelimport.HeaderExtDataSetRef, "virtual_server_usage", "label_group_usage", "ruleset_usage", "static_policy_scopes_usage", "pairing_profile_usage", "permission_usage", "workload_usage", "container_workload_usage", "firewall_coexistence_scope_usage", "containers_inherit_host_policy_scopes_usage", "container_workload_profile_usage", "blocked_connection_reject_scope_usage", "enforcement_boundary_usage", "loopback_interfaces_in_policy_scopes_usage", "virtual_service_usage"}}
	if noHref {
		csvData = [][]string{{labelimport.HeaderKey, labelimport.HeaderValue, labelimport.HeaderExtDataSet, labelimport.HeaderExtDataSetRef, "virtual_server_usage", "label_group_usage", "ruleset_usage", "static_policy_scopes_usage", "pairing_profile_usage", "permission_usage", "workload_usage", "container_workload_usage", "firewall_coexistence_scope_usage", "containers_inherit_host_policy_scopes_usage", "container_workload_profile_usage", "blocked_connection_reject_scope_usage", "enforcement_boundary_usage", "loopback_interfaces_in_policy_scopes_usage", "virtual_service_usage"}}

	}
	stdOutData := [][]string{{"href", "key", "value"}}

	// Get all labels
	labels, a, err := pce.GetLabels(map[string]string{"usage": "true"})
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
		if noHref {
			csvData = append(csvData, []string{l.Key, l.Value, l.ExternalDataSet, l.ExternalDataReference, strconv.FormatBool(l.LabelUsage.VirtualServer), strconv.FormatBool(l.LabelUsage.LabelGroup), strconv.FormatBool(l.LabelUsage.Ruleset), strconv.FormatBool(l.LabelUsage.StaticPolicyScopes), strconv.FormatBool(l.LabelUsage.PairingProfile), strconv.FormatBool(l.LabelUsage.Permission), strconv.FormatBool(l.LabelUsage.Workload), strconv.FormatBool(l.LabelUsage.ContainerWorkload), strconv.FormatBool(l.LabelUsage.FirewallCoexistenceScope), strconv.FormatBool(l.LabelUsage.ContainersInheritHostPolicyScopes), strconv.FormatBool(l.LabelUsage.ContainerWorkloadProfile), strconv.FormatBool(l.LabelUsage.BlockedConnectionRejectScope), strconv.FormatBool(l.LabelUsage.EnforcementBoundary), strconv.FormatBool(l.LabelUsage.LoopbackInterfacesInPolicyScopes), strconv.FormatBool(l.LabelUsage.VirtualService)})
		} else {
			csvData = append(csvData, []string{l.Href, l.Key, l.Value, l.ExternalDataSet, l.ExternalDataReference, strconv.FormatBool(l.LabelUsage.VirtualServer), strconv.FormatBool(l.LabelUsage.LabelGroup), strconv.FormatBool(l.LabelUsage.Ruleset), strconv.FormatBool(l.LabelUsage.StaticPolicyScopes), strconv.FormatBool(l.LabelUsage.PairingProfile), strconv.FormatBool(l.LabelUsage.Permission), strconv.FormatBool(l.LabelUsage.Workload), strconv.FormatBool(l.LabelUsage.ContainerWorkload), strconv.FormatBool(l.LabelUsage.FirewallCoexistenceScope), strconv.FormatBool(l.LabelUsage.ContainersInheritHostPolicyScopes), strconv.FormatBool(l.LabelUsage.ContainerWorkloadProfile), strconv.FormatBool(l.LabelUsage.BlockedConnectionRejectScope), strconv.FormatBool(l.LabelUsage.EnforcementBoundary), strconv.FormatBool(l.LabelUsage.LoopbackInterfacesInPolicyScopes), strconv.FormatBool(l.LabelUsage.VirtualService)})
		}
		stdOutData = append(stdOutData, []string{l.Href, l.Key, l.Value})
	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-label-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, stdOutData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d labels exported.", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no labels in PCE.", true)
	}

	utils.LogEndCommand("label-export")

}
