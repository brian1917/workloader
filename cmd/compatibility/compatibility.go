package compatibility

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var modeChangeInput, issuesOnly, single bool
var pce illumioapi.PCE
var outputFileName, labelFile, hrefFile string
var err error

func init() {
	CompatibilityCmd.Flags().StringVar(&labelFile, "label-file", "", "csv file with labels to filter query. the file should have 4 headers: role, app, env, and loc. The four columns in each row is an \"AND\" operation. Each row is an \"OR\" operation.")
	CompatibilityCmd.Flags().StringVar(&hrefFile, "href-file", "", "csv file with hrefs.")
	CompatibilityCmd.Flags().BoolVarP(&modeChangeInput, "mode-input", "m", false, "generate the input file to change all idle workloads to build using workloader mode command")
	CompatibilityCmd.Flags().BoolVarP(&issuesOnly, "issues-only", "i", false, "only export compatibility checks with an issue")
	CompatibilityCmd.Flags().BoolVar(&single, "single", false, "only used with --host-file. gets hosts by individual api calls vs. getting all workloads and filtering after.")
	CompatibilityCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	CompatibilityCmd.Flags().SortFlags = false
}

// CompatibilityCmd runs the workload identifier
var CompatibilityCmd = &cobra.Command{
	Use:   "compatibility",
	Short: "Generate a compatibility report for all Idle workloads.",
	Long: `
Generate a compatibility report for idle workloads.

If no label-file or href-file are used all idle workloads are processed.

The first row of a label-file should be label keys. The workload query uses an AND operator for entries on the same row and an OR operator for the separate rows. An example label file is below:
+------+-----+-----+-----+----+
| role | app | env | loc | bu |
+------+-----+-----+-----+----+
| web  | erp |     |     |    |
|      |     |     | bos | it |
|      | crm |     |     |    |
+------+-----+-----+-----+----+
This example queries all idle workloads that are
- web (role) AND erp (app) 
- OR bos(loc) AND it (bu)
- OR CRM (app)

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		compatibilityReport()
	},
}

func compatibilityReport() {

	// Get labels and label dimensions
	apiResps, err := pce.Load(illumioapi.LoadInput{LabelDimensions: true, Labels: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogErrorf("loading pce - %s", err)
	}

	// If there is no href file, get workloads with query parameters
	if hrefFile == "" {
		qp := map[string]string{"mode": "idle"}
		// Process a label file if one is provided
		if labelFile != "" {
			labelCsvData, err := utils.ParseCSV(labelFile)
			if err != nil {
				utils.LogErrorf("parsing labelFile - %s", err)
			}

			labelQuery, err := pce.WorkloadQueryLabelParameter(labelCsvData)
			if err != nil {
				utils.LogErrorf("getting label parameter query - %s", err)
			}
			if len(labelQuery) > 10000 {
				utils.LogErrorf("the query is too large. the total character count is %d and the limit for this command is 10,000", len(labelQuery))
			}
			qp["labels"] = labelQuery
		}
		// Get the workloads
		api, err := pce.GetWklds(qp)
		utils.LogAPIRespV2("GetWklds", api)
		if err != nil {
			utils.LogErrorf("GetWklds - %s", err)
		}
	} else {
		// A hostn file is provided - parse it
		hrefFileData, err := utils.ParseCSV(hrefFile)
		if err != nil {
			utils.LogErrorf("parsing hrefFile - %d", err)
		}
		// Build the href list
		hrefList := []string{}
		for _, row := range hrefFileData {
			hrefList = append(hrefList, row[0])
		}
		// Get the workloads
		apiResps, err := pce.GetWkldsByHrefList(hrefList, single)
		for _, a := range apiResps {
			utils.LogAPIRespV2("GetWkldsByHrefList", a)
		}
		if err != nil {
			utils.LogErrorf("GetWkldsByHrefList - %s", err)
		}
		// Validate workload is idle
		confirmedIdle := []illumioapi.Workload{}
		for _, w := range pce.WorkloadsSlice {
			if w.GetMode() != "idle" {
				utils.LogInfof(true, "%s-%s-is not idle skipping.", illumioapi.PtrToVal(w.Hostname), w.Href)
				continue
			}
			confirmedIdle = append(confirmedIdle, w)
		}
		pce.WorkloadsSlice = confirmedIdle
	}

	// Get the label information
	labelKeys := []string{}
	for _, ld := range pce.LabelDimensionsSlice {
		labelKeys = append(labelKeys, ld.Key)
	}
	wkldExport := wkldexport.WkldExport{PCE: &pce, Headers: append([]string{"href"}, labelKeys...)}
	wkldMapData := wkldExport.MapData()

	// Start the output
	outputHeaders := append([]string{"hostname", "href", "status"}, labelKeys...)
	outputHeaders = append(outputHeaders, "os_id", "os_details", "required_packages_installed", "required_packages_missing", "ipsec_service_enabled", "ipv4_forwarding_enabled", "ipv4_forwarding_pkt_cnt", "iptables_rule_cnt", "ipv6_global_scope", "ipv6_active_conn_cnt", "ip6tables_rule_cnt", "routing_table_conflict", "IPv6_enabled", "Unwanted_nics", "GroupPolicy", "raw_data")
	csvData := [][]string{outputHeaders}
	modeChangeInputData := [][]string{{"href", "mode"}}

	// Create a warning logs holder
	warningLogs := []string{}

	// Iterate through each workload
	for i, w := range pce.WorkloadsSlice {
		utils.LogInfof(true, "reviewing compatibility report for %s - %s - %d of %d", illumioapi.PtrToVal(w.Hostname), w.Href, i+1, len(pce.WorkloadsSlice))

		// Get the compatibility report and append
		cr, a, err := pce.GetCompatibilityReport(w)
		utils.LogAPIRespV2("GetCompatibilityReport", a)
		if err != nil {
			utils.LogWarningf(true, "error compatibility report for %s (%s) - %s - skipping", illumioapi.PtrToVal(w.Hostname), w.Href, err)
			continue
		}

		// Get the online status
		onlineStatus := "false"
		if illumioapi.PtrToVal(w.Online) {
			onlineStatus = "true"
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
		if strings.Contains(illumioapi.PtrToVal(w.OsID), "win") {
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

		if cr.Results != nil {
			for _, c := range illumioapi.PtrToVal(cr.Results.QualifyTests) {
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
				checks := []interface{}{
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
						*variable = c.Status
					}
				}

				// Process missing packages separately
				if c.RequiredPackagesMissing != nil {
					requiredPackagesMissing = strings.Join(*c.RequiredPackagesMissing, ";")
				}
			}
		} else {

			warningLogs = append(warningLogs, fmt.Sprintf("%s is an idle %s workload but does not have compatibility results", illumioapi.PtrToVal(w.Hostname), onlineStatus))
			continue
		}

		if cr.QualifyStatus == "" {
			warningLogs = append(warningLogs, fmt.Sprintf("%s is an idle %s workload but does not have a compatibility report", illumioapi.PtrToVal(w.Hostname), onlineStatus))
			continue
		}

		// Put into slice if it's not green and issuesOnly is true
		if (cr.QualifyStatus != "green" && issuesOnly) || !issuesOnly {
			rowEntry := []string{illumioapi.PtrToVal(w.Hostname), w.Href, cr.QualifyStatus}
			for _, key := range labelKeys {
				rowEntry = append(rowEntry, wkldMapData[w.Href][key])
			}
			rowEntry = append(rowEntry, illumioapi.PtrToVal(w.OsID), illumioapi.PtrToVal(w.OsDetail), requiredPackagesInstalled, requiredPackagesMissing, ipsecServiceEnabled, ipv4ForwardingEnabled, ipv4ForwardingPktCnt, iptablesRuleCnt, ipv6GlobalScope, ipv6ActiveConnCnt, iP6TablesRuleCnt, routingTableConflict, iPv6Enabled, unwantedNics, groupPolicy, a.RespBody)
			csvData = append(csvData, rowEntry)
		}

		if cr.QualifyStatus == "green" {
			modeChangeInputData = append(modeChangeInputData, []string{w.Href, "visibility_only"})
		}

	}

	// Warnings
	for _, wl := range warningLogs {
		utils.LogWarning(wl, true)
	}

	// If the CSV data has more than just the headers, create output file and write it.
	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-compatibility-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		utils.LogInfof(true, "%d compatibility reports exported.", len(csvData)-1)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no workloads with compatibility reports for provided query.", true)
	}

	// Write the mode change CSV
	if modeChangeInput && len(modeChangeInputData) > 1 {
		// Create CSV
		utils.LogInfo("creating mode input file...", true)
		outputFileName = "mode-input-" + outputFileName
		utils.WriteOutput(modeChangeInputData, nil, outputFileName)
	}

}
