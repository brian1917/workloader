package wkldexport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var managedOnly, unmanagedOnly bool
var outputFileName string

func init() {
	WkldExportCmd.Flags().BoolVarP(&managedOnly, "managed-only", "m", false, "Only export managed workloads.")
	WkldExportCmd.Flags().BoolVarP(&unmanagedOnly, "unmanaged-only", "u", false, "Only export unmanaged workloads.")
	WkldExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

}

// WkldExportCmd runs the workload identifier
var WkldExportCmd = &cobra.Command{
	Use:   "wkld-export",
	Short: "Create a CSV export of all workloads in the PCE.",
	Long: `
Create a CSV export of all workloads in the PCE.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		exportWorkloads()
	},
}

func exportWorkloads() {

	// Log command execution
	utils.LogStartCommand("wkld-export")

	// Start the data slice with headers
	csvData := [][]string{{HeaderHostname, HeaderName, HeaderRole, HeaderApp, HeaderEnv, HeaderLoc, HeaderInterfaces, HeaderPublicIP, HeaderMachineAuthenticationID, HeaderIPWithDefaultGw, HeaderNetmaskOfIPWithDefGw, HeaderDefaultGw, HeaderDefaultGwNetwork, HeaderHref, HeaderDescription, HeaderPolicyState, HeaderOnline, HeaderAgentStatus, HeaderSecurityPolicySyncState, HeaderSecurityPolicyAppliedAt, HeaderSecurityPolicyReceivedAt, HeaderSecurityPolicyRefreshAt, HeaderLastHeartbeatOn, HeaderHoursSinceLastHeartbeat, HeaderCreatedAt, HeaderOsID, HeaderOsDetail, HeaderAgentVersion, HeaderAgentID, HeaderActivePceFqdn, HeaderServiceProvider, HeaderDataCenter, HeaderDataCenterZone, HeaderCloudInstanceID, HeaderExternalDataSet, HeaderExternalDataReference}}
	stdOutData := [][]string{{HeaderHostname, HeaderRole, HeaderApp, HeaderEnv, HeaderLoc, HeaderPolicyState}}

	// GetAllWorkloads
	qp := make(map[string]string)
	if unmanagedOnly {
		qp["managed"] = "false"
	}
	if managedOnly {
		qp["managed"] = "true"
	}
	wklds, a, err := pce.GetAllWorkloadsQP(qp)
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(fmt.Sprintf("getting all workloads - %s", err))
	}

	for _, w := range wklds {

		// Skip deleted workloads
		if *w.Deleted {
			continue
		}

		// Get interfaces
		interfaces := []string{}
		for _, i := range w.Interfaces {
			ipAddress := fmt.Sprintf("%s:%s", i.Name, i.Address)
			if i.CidrBlock != nil && *i.CidrBlock != 0 {
				ipAddress = fmt.Sprintf("%s:%s/%s", i.Name, i.Address, strconv.Itoa(*i.CidrBlock))
			}
			interfaces = append(interfaces, ipAddress)
		}

		// Assume the VEN-dependent fields are unmanaged
		policySyncStatus := "unmanaged"
		policyAppliedAt := "unmanaged"
		poicyReceivedAt := "unmanaged"
		policyRefreshAt := "unmanaged"
		venVersion := "unmanaged"
		lastHeartBeat := "unmanaged"
		hoursSinceLastHB := "unmanaged"
		venID := "unmanaged"
		pairedPCE := "unmanaged"
		agentStatus := "unmanaged"
		instanceID := "unmanaged"
		// If it is managed, get that information
		if w.Agent != nil && w.Agent.Href != "" {
			policySyncStatus = w.Agent.Status.SecurityPolicySyncState
			policyAppliedAt = w.Agent.Status.SecurityPolicyAppliedAt
			poicyReceivedAt = w.Agent.Status.SecurityPolicyReceivedAt
			policyRefreshAt = w.Agent.Status.SecurityPolicyRefreshAt
			venVersion = w.Agent.Status.AgentVersion
			lastHeartBeat = w.Agent.Status.LastHeartbeatOn
			hoursSinceLastHB = fmt.Sprintf("%f", w.HoursSinceLastHeartBeat())
			venID = w.Agent.GetID()
			pairedPCE = w.Agent.ActivePceFqdn
			if pairedPCE == "" {
				pairedPCE = pce.FQDN
			}
			agentStatus = w.Agent.Status.Status
			instanceID = w.Agent.Status.InstanceID
			if instanceID == "" {
				instanceID = "NA"
			}
		}

		// Set online status
		var online string
		if w.GetMode() == "unmanaged" {
			online = "unmanaged"
		} else {
			online = strconv.FormatBool(w.Online)
		}

		// Append to data slice
		csvData = append(csvData, []string{w.Hostname, w.Name, w.GetRole(pce.Labels).Value, w.GetApp(pce.Labels).Value, w.GetEnv(pce.Labels).Value, w.GetLoc(pce.Labels).Value, strings.Join(interfaces, ";"), w.PublicIP, w.DistinguishedName, w.GetIPWithDefaultGW(), w.GetNetMaskWithDefaultGW(), w.GetDefaultGW(), w.GetNetworkWithDefaultGateway(), w.Href, w.Description, w.GetMode(), online, agentStatus, policySyncStatus, policyAppliedAt, poicyReceivedAt, policyRefreshAt, lastHeartBeat, hoursSinceLastHB, w.CreatedAt, w.OsID, w.OsDetail, venVersion, venID, pairedPCE, w.ServiceProvider, w.DataCenter, w.DataCenterZone, instanceID, w.ExternalDataSet, w.ExternalDataReference})
		stdOutData = append(stdOutData, []string{w.Hostname, w.GetRole(pce.Labels).Value, w.GetApp(pce.Labels).Value, w.GetEnv(pce.Labels).Value, w.GetLoc(pce.Labels).Value, w.GetMode()})
	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-wkld-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, stdOutData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d workloads exported", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no workloads in PCE.", true)
	}

	utils.LogEndCommand("wkld-export")

}
