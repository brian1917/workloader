package compatibility

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debug, modeChangeInput, issuesOnly bool
var pce illumioapi.PCE
var outputFileName string
var err error

func init() {
	CompatibilityCmd.Flags().BoolVarP(&modeChangeInput, "mode-input", "m", false, "generate the input file to change all idle workloads to build using workloader mode command")
	CompatibilityCmd.Flags().BoolVarP(&issuesOnly, "issues-only", "i", false, "only export compatibility checks with an issue")
	CompatibilityCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
}

// CompatibilityCmd runs the workload identifier
var CompatibilityCmd = &cobra.Command{
	Use:   "compatibility",
	Short: "Generate a compatibility report for all Idle workloads.",
	Long: `
Generate a compatibility report for all Idle workloads.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		compatibilityReport()
	},
}

func compatibilityReport() {

	// Log command
	utils.LogStartCommand("compatibility")

	// Start the data slice with the headers. We will append data to this.
	var csvData, stdOutData, modeChangeInputData [][]string
	csvData = append(csvData, []string{"hostname", "href", "status", "role", "app", "env", "loc", "os_id", "os_details", "required_packages_installed", "required_packages_missing", "ipsec_service_enabled", "ipv4_forwarding_enabled", "ipv4_forwarding_pkt_cnt", "iptables_rule_cnt", "ipv6_global_scope", "ipv6_active_conn_cnt", "ip6tables_rule_cnt", "routing_table_conflict,omitempty", "IPv6_enabled", "Unwanted_nics", "GroupPolicy", "raw_data"})
	stdOutData = append(stdOutData, []string{"hostname", "href", "status"})
	modeChangeInputData = append(modeChangeInputData, []string{"href", "mode"})

	// Get all idle  workloads
	qp := map[string]string{"mode": "idle"}
	wklds, a, err := pce.GetAllWorkloadsQP(qp)
	utils.LogAPIResp("GetAllWorkloadsH", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get Idle workload count
	idleWklds := []illumioapi.Workload{}
	for _, w := range wklds {
		if w.Agent.Config.Mode == "idle" {
			idleWklds = append(idleWklds, w)
		}
	}

	// Iterate through each workload
	for i, w := range idleWklds {

		// Get the compatibility report and append
		cr, a, err := pce.GetCompatibilityReport(w)
		utils.LogAPIResp("GetCompatibilityReport", a)
		if err != nil {
			utils.LogError(fmt.Sprintf("getting compatibility report for %s (%s) - %s", w.Hostname, w.Href, err))
		}

		// Set the initial values that should throw no status errors
		requiredPackagesInstalled := "true"
		requiredPackagesMissing := ""
		ipsecServiceEnabled := "true"
		iPv6Enabled := "na"
		unwantedNics := "na"
		groupPolicy := "na"
		ipv4ForwardingEnabled := "false"
		ipv4ForwardingPktCnt := "0"
		iptablesRuleCnt := "0"
		ipv6GlobalScope := "false"
		ipv6ActiveConnCnt := "0"
		iP6TablesRuleCnt := "0"
		routingTableConflict := "false"
		if strings.Contains(w.OsID, "win") {
			iPv6Enabled = "false"
			unwantedNics = "false"
			groupPolicy = "false"
			ipv4ForwardingEnabled = "na"
			ipv4ForwardingPktCnt = "na"
			iptablesRuleCnt = "na"
			ipv6GlobalScope = "na"
			ipv6ActiveConnCnt = "na"
			iP6TablesRuleCnt = "na"
			routingTableConflict = "na"
		}

		for _, c := range cr.Results.QualifyTests {
			// If the value is not the default, we set the value to the non-default
			if strings.ToLower(c.IpsecServiceEnabled) == "false" {
				ipsecServiceEnabled = "false"
			}
			if c.Ipv4ForwardingEnabled != false {
				ipv4ForwardingEnabled = strconv.FormatBool(c.Ipv4ForwardingEnabled)
			}
			if c.Ipv4ForwardingPktCnt != 0 {
				ipv4ForwardingPktCnt = strconv.Itoa(c.Ipv4ForwardingPktCnt)
			}
			if c.IptablesRuleCnt != 0 {
				iptablesRuleCnt = strconv.Itoa(c.IptablesRuleCnt)
			}
			if c.Ipv6GlobalScope != false {
				ipv6GlobalScope = strconv.FormatBool(c.Ipv6GlobalScope)
			}
			if c.Ipv6ActiveConnCnt != 0 {
				ipv6ActiveConnCnt = strconv.Itoa(c.Ipv6ActiveConnCnt)
			}
			if c.IP6TablesRuleCnt != 0 {
				iP6TablesRuleCnt = strconv.Itoa(c.IP6TablesRuleCnt)
			}
			if c.RoutingTableConflict != false {
				routingTableConflict = strconv.FormatBool(c.RoutingTableConflict)
			}
			if c.IPv6Enabled != false {
				iPv6Enabled = strconv.FormatBool(c.IPv6Enabled)
			}
			if c.UnwantedNics != false {
				unwantedNics = strconv.FormatBool(c.UnwantedNics)
			}
			if c.GroupPolicy != false {
				groupPolicy = strconv.FormatBool(c.GroupPolicy)
			}
			if strings.ToLower(c.RequiredPackagesInstalled) == "false" {
				requiredPackagesInstalled = "false"
			}
			if len(c.RequiredPackagesMissing) != 0 {
				requiredPackagesMissing = strings.Join(c.RequiredPackagesMissing, ";")
			}
		}

		// Put into slice if it's NOT green and issuesOnly is true
		if (cr.QualifyStatus != "green" && issuesOnly) || !issuesOnly {
			csvData = append(csvData, []string{w.Hostname, w.Href, cr.QualifyStatus, w.GetRole(pce.LabelMapH).Value, w.GetApp(pce.LabelMapH).Value, w.GetEnv(pce.LabelMapH).Value, w.GetLoc(pce.LabelMapH).Value, w.OsID, w.OsDetail, requiredPackagesInstalled, requiredPackagesMissing, ipsecServiceEnabled, ipv4ForwardingEnabled, ipv4ForwardingPktCnt, iptablesRuleCnt, ipv6GlobalScope, ipv6ActiveConnCnt, iP6TablesRuleCnt, routingTableConflict, iPv6Enabled, unwantedNics, groupPolicy, a.RespBody})
			stdOutData = append(stdOutData, []string{w.Hostname, w.Href, cr.QualifyStatus})
		}

		if cr.QualifyStatus == "green" {
			modeChangeInputData = append(modeChangeInputData, []string{w.Href, "build"})
		}

		// Update stdout
		end := ""
		if i+1 == len(idleWklds) {
			end = "\r\n"
		}
		fmt.Printf("\r[INFO] - Exported %d of %d idle workloads (%d%%).%s", i+1, len(wklds), (i+1)*100/len(wklds), end)
	}

	// If the CSV data has more than just the headers, create output file and write it.
	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-compatibility-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, stdOutData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d compatibility reports exported.", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no workloads in idle mode.", true)
	}

	// Write the mode change CSV
	if modeChangeInput && len(modeChangeInputData) > 1 {
		// Create CSV
		if outputFileName == "" {
			outputFileName = "mode-input-" + outputFileName
		}
		outFile, err := os.Create(outputFileName)
		if err != nil {
			utils.LogError(fmt.Sprintf("creating CSV - %s\n", err))
		}

		// Write CSV data
		writer := csv.NewWriter(outFile)
		writer.WriteAll(modeChangeInputData)
		if err := writer.Error(); err != nil {
			utils.LogError(fmt.Sprintf("writing CSV - %s\n", err))
		}
		// Log
		utils.LogInfo(fmt.Sprintf("Created a file to be used with workloader mode command to change all green status IDLE workloads to build: %s", outFile.Name()), true)
	}
	utils.LogEndCommand("compatibility")

}
