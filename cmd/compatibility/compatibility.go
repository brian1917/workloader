package compatibility

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var modeChangeInput, issuesOnly bool
var pce illumioapi.PCE
var outputFileName, role, app, env, loc, labelFile string
var err error

func init() {
	CompatibilityCmd.Flags().BoolVarP(&modeChangeInput, "mode-input", "m", false, "generate the input file to change all idle workloads to build using workloader mode command")
	CompatibilityCmd.Flags().BoolVarP(&issuesOnly, "issues-only", "i", false, "only export compatibility checks with an issue")
	CompatibilityCmd.Flags().StringVarP(&role, "role", "r", "", "role label value. label flags are an \"and\" operator.")
	CompatibilityCmd.Flags().StringVarP(&app, "app", "a", "", "app label value. label flags are an \"and\" operator.")
	CompatibilityCmd.Flags().StringVarP(&env, "env", "e", "", "env label value. label flags are an \"and\" operator.")
	CompatibilityCmd.Flags().StringVarP(&loc, "loc", "l", "", "loc label value. label flags are an \"and\" operator.")
	CompatibilityCmd.Flags().StringVar(&labelFile, "label-file", "", "csv file with labels to filter query. the file should have 4 headers: role, app, env, and loc. The four columns in each row is an \"AND\" operation. Each row is an \"OR\" operation.")
	CompatibilityCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
}

// CompatibilityCmd runs the workload identifier
var CompatibilityCmd = &cobra.Command{
	Use:   "compatibility",
	Short: "Generate a compatibility report for all Idle workloads.",
	Long: `
Generate a compatibility report for all Idle workloads.

The --role (-r), --app (-a), --env(-e), and --loc(-l) flags can be used with one label per key and is run as an "AND" operation. The workloads must have all the labels.

If using --label-file, the other label flags are ignored. The label file first row must be "role", "app", "env", and "loc". The order does not matter. The entries in each row are an "AND" operation and the rows are combined in "OR" operations. See example below:
+------+-----+------+-----+
| role | app | env  | loc |
+------+-----+------+-----+
| WEB  | ERP | PROD |     |
| DB   | CRM |      | AWS |
+------+-----+------+-----+

With the input file above, the query will get all IDLE workloads that are labeled as WEB (role) AND ERP (app) AND PROD (env) AND any location OR IDLE workloads that are labeled DB (role) AND CRM (app) AND any environment AND AWS (loc).

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		compatibilityReport()
	},
}

func compatibilityReport() {

	// Log command
	utils.LogStartCommand("compatibility")

	// Start the data slice with the headers. We will append data to this.
	var csvData, stdOutData, modeChangeInputData [][]string
	csvData = append(csvData, []string{"hostname", "href", "status", "role", "app", "env", "loc", "os_id", "os_details", "required_packages_installed", "required_packages_missing", "ipsec_service_enabled", "ipv4_forwarding_enabled", "ipv4_forwarding_pkt_cnt", "iptables_rule_cnt", "ipv6_global_scope", "ipv6_active_conn_cnt", "ip6tables_rule_cnt", "routing_table_conflict", "IPv6_enabled", "Unwanted_nics", "GroupPolicy", "raw_data"})
	stdOutData = append(stdOutData, []string{"hostname", "href", "status"})
	modeChangeInputData = append(modeChangeInputData, []string{"href", "mode"})

	// Get all idle  workloads - start query with just idle
	qp := map[string]string{"mode": "idle"}

	// Process the file if provided
	if labelFile != "" {
		// Parse the CSV
		labelData, err := utils.ParseCSV(labelFile)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the labelQuery
		qp["labels"], err = pce.WorkloadQueryLabelParameter(labelData)
		if err != nil {
			utils.LogError(err.Error())
		}

	} else {
		providedValues := []string{role, app, env, loc}
		keys := []string{"role", "app", "env", "loc"}
		queryLabels := []string{}
		for i, labelValue := range providedValues {
			// Do nothing if the labelValue is blank
			if labelValue == "" {
				continue
			}
			// Confirm the label exists
			if label, ok := pce.Labels[keys[i]+labelValue]; !ok {
				utils.LogError(fmt.Sprintf("%s does not exist as a %s label", labelValue, keys[i]))
			} else {
				queryLabels = append(queryLabels, label.Href)
			}

		}

		// If we have query labels add to the map
		if len(queryLabels) > 0 {
			qp["labels"] = fmt.Sprintf("[[\"%s\"]]", strings.Join(queryLabels, "\",\""))
		}
	}

	if len(qp["labels"]) > 10000 {
		utils.LogError(fmt.Sprintf("the query is too large. the total character count is %d and the limit for this command is 10,000", len(qp["labels"])))
	}

	// Get all workloads from the query
	wklds, a, err := pce.GetAllWorkloadsQP(qp)
	utils.LogAPIResp("GetAllWorkloadsQP", a)
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

		// Set the initial values for Linux, AIX, and Solaris and override for Windows
		requiredPackagesInstalled := "green"
		requiredPackagesMissing := ""
		ipsecServiceEnabled := "green"
		iPv6Enabled := "na"
		unwantedNics := "na"
		groupPolicy := "na"
		ipv4ForwardingEnabled := "green"
		ipv4ForwardingPktCnt := "green"
		iptablesRuleCnt := "green"
		ipv6GlobalScope := "green"
		ipv6ActiveConnCnt := "green"
		iP6TablesRuleCnt := "green"
		routingTableConflict := "green"
		if strings.Contains(w.OsID, "win") {
			iPv6Enabled = "green"
			unwantedNics = "green"
			groupPolicy = "green"
			ipv4ForwardingEnabled = "na"
			ipv4ForwardingPktCnt = "na"
			iptablesRuleCnt = "na"
			ipv6GlobalScope = "na"
			ipv6ActiveConnCnt = "na"
			iP6TablesRuleCnt = "na"
			routingTableConflict = "na"
		}

		for _, c := range cr.Results.QualifyTests {
			variables := []*string{
				&requiredPackagesInstalled,
				&ipsecServiceEnabled,
				&iPv6Enabled,
				&unwantedNics,
				&groupPolicy,
				&ipv4ForwardingEnabled,
				&ipv4ForwardingPktCnt,
				&iptablesRuleCnt,
				&ipv6GlobalScope,
				&ipv6ActiveConnCnt,
				&iP6TablesRuleCnt,
				&routingTableConflict}
			checks := []*string{
				c.RequiredPackagesInstalled,
				c.IpsecServiceEnabled,
				c.IPv6Enabled,
				c.UnwantedNics,
				c.GroupPolicy,
				c.Ipv4ForwardingEnabled,
				c.Ipv4ForwardingPktCnt,
				c.IptablesRuleCnt,
				c.Ipv6GlobalScope,
				c.Ipv6ActiveConnCnt,
				c.IP6TablesRuleCnt,
				c.RoutingTableConflict}

			for i, variable := range variables {
				if checks[i] != nil {
					*variable = *c.Status
				}
			}

			// Process missing packages separately
			if c.RequiredPackagesMissing != nil {
				requiredPackagesMissing = strings.Join(*c.RequiredPackagesMissing, ";")
			}
		}

		// Put into slice if it's NOT green and issuesOnly is true
		if (cr.QualifyStatus != "green" && issuesOnly) || !issuesOnly {
			csvData = append(csvData, []string{w.Hostname, w.Href, cr.QualifyStatus, w.GetRole(pce.Labels).Value, w.GetApp(pce.Labels).Value, w.GetEnv(pce.Labels).Value, w.GetLoc(pce.Labels).Value, w.OsID, w.OsDetail, requiredPackagesInstalled, requiredPackagesMissing, ipsecServiceEnabled, ipv4ForwardingEnabled, ipv4ForwardingPktCnt, iptablesRuleCnt, ipv6GlobalScope, ipv6ActiveConnCnt, iP6TablesRuleCnt, routingTableConflict, iPv6Enabled, unwantedNics, groupPolicy, a.RespBody})
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
		utils.LogInfo("no workloads in idle mode for provided query.", true)
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
