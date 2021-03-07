package edgeadmin

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/workloader/cmd/iplimport"
	"github.com/brian1917/workloader/cmd/wkldimport"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debug, updatePCE, noPrompt, doNotProvision bool
var csvFile, fromPCE, toPCE, outputFileName, edgeGroup string

func init() {
	EdgeAdminCmd.Flags().StringVarP(&fromPCE, "from-pce", "f", "", "Name of the PCE with the existing Edge Group to copy over. Required")
	EdgeAdminCmd.MarkFlagRequired("from-pce")
	EdgeAdminCmd.Flags().StringVarP(&toPCE, "to-pce", "t", "", "Name of the PCE to receive Edge Admin Group endpoint info. Only required if using --update-pce flag")
	EdgeAdminCmd.Flags().StringVarP(&edgeGroup, "edge-group", "g", "", "Name of the Edge group to be copied to Core PCE")
	EdgeAdminCmd.MarkFlagRequired("edge-group")
	EdgeAdminCmd.Flags().BoolVarP(&doNotProvision, "do-not-prov", "x", false, "Do not provision created/updated IP Lists. Will provision by default.")
	EdgeAdminCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

}

// EdgeAdminCmd runs the upload command
var EdgeAdminCmd = &cobra.Command{
	Use:   "edge-admin ",
	Short: "Copy Edge Admin group to Core PCE for authenticated Admin Access to Core Workloads.",
	Long: `
Copy every endpoint DN infrmation within specified Edge group from Edge PCE to Core PCE. Every endpoint must have a valid ipsec certificate to be copied over.  
The Edge group once copied can be used in policy using MachineAuth option to protect access requiring certificate validated ipsec connections.  
No IP address information will be copied from Edge to Core so only MachineAuth rules will work.
`,

	Run: func(cmd *cobra.Command, args []string) {

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		// Disable stdout
		viper.Set("output_format", "csv")
		if err := viper.WriteConfig(); err != nil {
			utils.LogError(err.Error())
		}

		edgeadmin()
	},
}

func edgeadmin() {

	// Log start of run
	utils.LogStartCommand("edge-admin")

	// Check if we have destination PCE if we need it
	if updatePCE && toPCE == "" {
		utils.LogError("need --to-pce (-t) flag set if using update-pce")
	}

	// Get the source pce
	sPce, err := utils.GetPCEbyName(fromPCE, true)
	if err != nil {
		utils.LogError(err.Error())
	}

	label, a, err := sPce.GetLabelbyKeyValue("role", edgeGroup)
	utils.LogAPIResp("GetLabelbyKeyValue", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	if label.Value == "" {
		utils.LogError(fmt.Sprintf("erro finding Edge group - %s", edgeGroup))
	}

	queryP := map[string]string{"labels": fmt.Sprintf("[[\"%s\"]]", label.Href)}
	// Get all workloads from the source PCE
	wklds, a, err := sPce.GetAllWorkloadsQP(queryP)
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	var input wkldimport.Input
	csvOut := [][]string{{"hostname", "name", "role", "app", "env", "loc", "interfaces", "public_ip", "ip_with_default_gw", "netmask_of_ip_with_def_gw", "default_gw", "default_gw_network", "href", "description", "policy_state", "online", "agent_status", "security_policy_sync_state", "security_policy_applied_at", "security_policy_received_at", "security_policy_refresh_at", "last_heartbea_on", "hours_since_last_heartbeat", "os_id", "os_detail", "agent_version", "agent_id", "active_pce_fqdn", "service_provider", "data_center", "data_center_zone", "cloud_instance_id", "external_data_set", "external_data_reference"}}
	stdOut := [][]string{{"hostname", "role", "app", "env", "loc", "mode"}}

	for _, w := range wklds {

		// Output the CSV
		if len(wklds) > 0 {
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
					pairedPCE = sPce.FQDN
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
			csvOut = append(csvOut, []string{w.Hostname, w.Name, w.GetRole(sPce.LabelMapH).Value, "app", "env", "loc", strings.Join(interfaces, ";"), w.PublicIP, w.GetIPWithDefaultGW(), w.GetNetMaskWithDefaultGW(), w.GetDefaultGW(), w.GetNetworkWithDefaultGateway(), w.Href, w.Description, w.GetMode(), online, agentStatus, policySyncStatus, policyAppliedAt, poicyReceivedAt, policyRefreshAt, lastHeartBeat, hoursSinceLastHB, w.OsID, w.OsDetail, venVersion, venID, pairedPCE, w.ServiceProvider, w.DataCenter, w.DataCenterZone, instanceID, w.ExternalDataSet, w.ExternalDataReference})
			stdOut = append(stdOut, []string{w.Hostname, w.GetRole(sPce.LabelMapH).Value, "app", "env", "loc", w.GetMode()})
		} else {
			utils.LogInfo("no Workloads created.", true)
		}
	}

	if outputFileName == "" {
		outputFileName = "workloader-edge-admin-output-" + time.Now().Format("20060102_150405") + ".csv"
	}
	input.ImportFile = outputFileName
	utils.WriteOutput(csvOut, csvOut, outputFileName)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintln("See the output file for IP Lists that would be created. Run again using --to-pce and --update-pce flags to create the IP lists."), true)
		utils.LogEndCommand("wkld-to-ipl")
		return
	}

	// If we get here, create the workloads on dest PCE using wkld-import
	utils.LogInfo(fmt.Sprintf("calling workloader ipl-import to import %s to %s", outputFileName, toPCE), true)
	dPce, err := utils.GetPCEbyName(toPCE, false)
	if err != nil {
		utils.LogError(fmt.Sprintf("error getting to pce - %s", err))
	}
	iplimport.ImportIPLists(dPce, outputFileName, updatePCE, noPrompt, debug, !doNotProvision)

	utils.LogEndCommand("wkld-to-ipl")
}
