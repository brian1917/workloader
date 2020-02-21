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
var debug bool
var outFormat string

// WkldExportCmd runs the workload identifier
var WkldExportCmd = &cobra.Command{
	Use:   "wkld-export",
	Short: "Create a CSV export of all workloads in the PCE.",
	Long: `
Create a CSV export of all workloads in the PCE. The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.Log(1, err.Error())
		}

		// Get the viper values
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		exportWorkloads()
	},
}

func exportWorkloads() {

	// Log command execution
	utils.Log(0, "running export command")

	// Start the data slice with headers
	csvData := [][]string{[]string{"hostname", "role", "app", "env", "loc", "interfaces", "ip_with_default_gw", "netmask_of_ip_with_def_gw", "default_gw", "default_gw_network", "href", "name", "mode", "online", "policy_sync_status", "policy_applied", "policy_received", "policy_refreshed", "last_heartbeat", "hours_since_last_heartbeat", "os_id", "os_details", "ven_version", "ven_id"}}
	stdOutData := [][]string{[]string{"hostname", "role", "app", "env", "loc", "mode"}}

	// GetAllWorkloads
	wklds, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloads", a)
		// Logging code added to debug specific issue
		for _, w := range wklds {
			utils.Log(2, fmt.Sprintf("%s mode: %s", w.Hostname, w.GetMode()))
			utils.Log(2, fmt.Sprintf("%s Agent Href: %s", w.Hostname, w.Agent.Href))
			utils.Log(2, fmt.Sprintf("%s Agent Mode: %s", w.Hostname, w.Agent.Config.Mode))
		}
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting all workloads - %s", err))
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

		// Get interfaces
		for _, i := range w.Interfaces {
			ipAddress := fmt.Sprintf("%s:%s", i.Name, i.Address)
			if i.CidrBlock != nil {
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
		} else {
			venID = w.Agent.GetID()
			venVersion = w.Agent.Status.AgentVersion
			policySyncStatus = w.Agent.Status.SecurityPolicySyncState
			policyAppliedAt = w.Agent.Status.SecurityPolicyAppliedAt
			poicyReceivedAt = w.Agent.Status.SecurityPolicyReceivedAt
			policyRefreshAt = w.Agent.Status.SecurityPolicyRefreshAt
			lastHeartBeat = w.Agent.Status.LastHeartbeatOn
			hoursSinceLastHB = fmt.Sprintf("%f", w.HoursSinceLastHeartBeat())
		}

		// Set online status
		var online string
		if w.GetMode() == "unmanaged" {
			online = "unmanaged"
		} else {
			online = strconv.FormatBool(w.Online)
		}

		// Append to data slice
		csvData = append(csvData, []string{w.Hostname, w.GetRole(pce.LabelMapH).Value, w.GetApp(pce.LabelMapH).Value, w.GetEnv(pce.LabelMapH).Value, w.GetLoc(pce.LabelMapH).Value, strings.Join(interfaces, ";"), w.GetIPWithDefaultGW(), w.GetNetMaskWithDefaultGW(), w.GetDefaultGW(), w.GetNetworkWithDefaultGateway(), w.Href, w.Name, w.GetMode(), online, policySyncStatus, policyAppliedAt, poicyReceivedAt, policyRefreshAt, lastHeartBeat, hoursSinceLastHB, w.OsID, w.OsDetail, venVersion, venID})
		stdOutData = append(stdOutData, []string{w.Hostname, w.GetRole(pce.LabelMapH).Value, w.GetApp(pce.LabelMapH).Value, w.GetEnv(pce.LabelMapH).Value, w.GetLoc(pce.LabelMapH).Value, w.GetMode()})
	}

	if len(csvData) > 1 {
		utils.WriteOutput(csvData, stdOutData, fmt.Sprintf("workloader-export-%s.csv", time.Now().Format("20060102_150405")))
		fmt.Printf("\r\n%d workloads exported.\r\n", len(csvData)-1)
		fmt.Println("Note - the CSV export will include additional columns: interfaces, default_gw, href, name, online, os, ven version, and ven id.")
		utils.Log(0, fmt.Sprintf("export complete - %d workloads exported", len(csvData)-1))
	} else {
		// Log command execution for 0 results
		fmt.Println("No workloads in PCE.")
		utils.Log(0, "no workloads in PCE.")
	}

}
