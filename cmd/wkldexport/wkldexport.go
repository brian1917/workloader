package wkldexport

import (
	"fmt"
	"strconv"
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
var debug, managedOnly, unmanagedOnly bool
var outFormat string

func init() {
	WkldExportCmd.Flags().BoolVarP(&managedOnly, "managed-only", "m", false, "Only export managed workloads.")
	WkldExportCmd.Flags().BoolVarP(&unmanagedOnly, "unmanaged-only", "u", false, "Only export unmanaged workloads.")
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
		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the viper values
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		exportWorkloads()
	},
}

func exportWorkloads() {

	// Log command execution
	utils.LogStartCommand("wkld-export")

	// Start the data slice with headers
	csvData := [][]string{[]string{"hostname", "name", "role", "app", "env", "loc", "interfaces", "ip_with_default_gw", "netmask_of_ip_with_def_gw", "default_gw", "default_gw_network", "href", "description", "mode", "online", "policy_sync_status", "policy_applied", "policy_received", "policy_refreshed", "last_heartbeat", "hours_since_last_heartbeat", "os_id", "os_details", "ven_version", "agent_id", "active_pce_fqdn"}}
	stdOutData := [][]string{[]string{"hostname", "role", "app", "env", "loc", "mode"}}

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

		// Set up variables
		interfaces := []string{}
		venID := ""
		venVersion := ""
		policySyncStatus := ""
		policyAppliedAt := ""
		poicyReceivedAt := ""
		policyRefreshAt := ""
		lastHeartBeat := ""
		hoursSinceLastHB := ""
		pairedPCE := ""

		// Get interfaces
		for _, i := range w.Interfaces {
			ipAddress := fmt.Sprintf("%s:%s", i.Name, i.Address)
			if i.CidrBlock != nil && *i.CidrBlock != 0 {
				ipAddress = fmt.Sprintf("%s:%s/%s", i.Name, i.Address, strconv.Itoa(*i.CidrBlock))
			}
			interfaces = append(interfaces, ipAddress)
		}

		// Set VEN version, policy sync status, lastHeartBeat, and hours since last heartbeat
		if w.Agent == nil || w.Agent.Href == "" {
			policySyncStatus = "unmanaged"
			policyAppliedAt = "unmanaged"
			poicyReceivedAt = "unmanaged"
			policyRefreshAt = "unmanaged"
			venVersion = "unmanaged"
			lastHeartBeat = "unmanaged"
			hoursSinceLastHB = "unmanaged"
			venID = "unmanaged"
			pairedPCE = "unmanaged"
		} else {
			venID = w.Agent.GetID()
			venVersion = w.Agent.Status.AgentVersion
			policySyncStatus = w.Agent.Status.SecurityPolicySyncState
			policyAppliedAt = w.Agent.Status.SecurityPolicyAppliedAt
			poicyReceivedAt = w.Agent.Status.SecurityPolicyReceivedAt
			policyRefreshAt = w.Agent.Status.SecurityPolicyRefreshAt
			lastHeartBeat = w.Agent.Status.LastHeartbeatOn
			hoursSinceLastHB = fmt.Sprintf("%f", w.HoursSinceLastHeartBeat())
			pairedPCE = w.Agent.ActivePceFqdn
			if pairedPCE == "" {
				pairedPCE = pce.FQDN
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
		csvData = append(csvData, []string{w.Hostname, w.Name, w.GetRole(pce.LabelMapH).Value, w.GetApp(pce.LabelMapH).Value, w.GetEnv(pce.LabelMapH).Value, w.GetLoc(pce.LabelMapH).Value, strings.Join(interfaces, ";"), w.GetIPWithDefaultGW(), w.GetNetMaskWithDefaultGW(), w.GetDefaultGW(), w.GetNetworkWithDefaultGateway(), w.Href, w.Description, w.GetMode(), online, policySyncStatus, policyAppliedAt, poicyReceivedAt, policyRefreshAt, lastHeartBeat, hoursSinceLastHB, w.OsID, w.OsDetail, venVersion, venID, pairedPCE})
		stdOutData = append(stdOutData, []string{w.Hostname, w.GetRole(pce.LabelMapH).Value, w.GetApp(pce.LabelMapH).Value, w.GetEnv(pce.LabelMapH).Value, w.GetLoc(pce.LabelMapH).Value, w.GetMode()})
	}

	if len(csvData) > 1 {
		utils.WriteOutput(csvData, stdOutData, fmt.Sprintf("workloader-wkld-export-%s.csv", time.Now().Format("20060102_150405")))
		utils.LogInfo(fmt.Sprintf("%d workloads exported", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no workloads in PCE.", true)
	}

	utils.LogEndCommand("wkld-export")

}
