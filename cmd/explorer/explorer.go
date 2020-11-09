package explorer

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

var app, env, inclHrefDstFile, exclHrefDstFile, inclHrefSrcFile, exclHrefSrcFile, exclServiceObj, inclServiceCSV, exclServiceCSV, start, end, loopFile string
var exclAllowed, exclPotentiallyBlocked, exclBlocked, appGroupLoc, ignoreIPGroup, consolidate, nonUni, debug, legacyOutput bool
var maxResults int
var pce illumioapi.PCE
var err error
var whm map[string]illumioapi.Workload

func init() {

	ExplorerCmd.Flags().StringVarP(&loopFile, "loop-label-file", "l", "", "file with columns of label hrefs on separate lines (without header). An explorer query for the label(s) as consumer OR provider is run for each app. For example, to iterate on app group, put the app label href in the first column and the environment href in the second column.")
	ExplorerCmd.Flags().StringVarP(&inclHrefDstFile, "incl-dst-file", "a", "", "file with hrefs on separate lines to be used in as a provider include. Can be a csv with hrefs in first column. Headers optional")
	ExplorerCmd.Flags().StringVarP(&exclHrefDstFile, "excl-dst-file", "b", "", "file with hrefs on separate lines to be used in as a provider exclude. Can be a csv with hrefs in first column. Headers optional")
	ExplorerCmd.Flags().StringVarP(&inclHrefSrcFile, "incl-src-file", "c", "", "file with hrefs on separate lines to be used in as a consumer include. Can be a csv with hrefs in first column. Headers optional")
	ExplorerCmd.Flags().StringVarP(&exclHrefSrcFile, "excl-src-file", "d", "", "file with hrefs on separate lines to be used in as a consumer exclude. Can be a csv with hrefs in first column. Headers optional")
	ExplorerCmd.Flags().StringVarP(&inclServiceCSV, "incl-svc-file", "i", "", "file location of csv with port/protocols to exclude. Port number in column 1 and IANA numeric protocol in Col 2. Headers optional.")
	ExplorerCmd.Flags().StringVarP(&exclServiceCSV, "excl-svc-file", "j", "", "file location of csv with port/protocols to exclude. Port number in column 1 and IANA numeric protocol in Col 2. Headers optional.")
	ExplorerCmd.Flags().StringVarP(&start, "start", "s", time.Date(time.Now().Year()-5, time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02"), "start date in the format of yyyy-mm-dd.")
	ExplorerCmd.Flags().StringVarP(&end, "end", "e", time.Now().Add(time.Hour*24).Format("2006-01-02"), "end date in the format of yyyy-mm-dd.")
	ExplorerCmd.Flags().BoolVar(&exclAllowed, "excl-allowed", false, "excludes allowed traffic flows.")
	ExplorerCmd.Flags().BoolVar(&exclPotentiallyBlocked, "excl-potentially-blocked", false, "excludes potentially blocked traffic flows.")
	ExplorerCmd.Flags().BoolVar(&exclBlocked, "excl-blocked", false, "excludes blocked traffic flows.")
	ExplorerCmd.Flags().BoolVar(&nonUni, "incl-non-unicast", false, "includes non-unicast (broadcast and multicast) flows in the output. Default is unicast only.")
	ExplorerCmd.Flags().IntVarP(&maxResults, "max-results", "m", 100000, "max results in explorer. Maximum value is 100000")
	ExplorerCmd.Flags().BoolVar(&consolidate, "consolidate", false, "consolidate flows that have same source IP, destination IP, port, and protocol.")
	ExplorerCmd.Flags().BoolVar(&appGroupLoc, "loc-in-ag", false, "includes the location in the app group in CSV output.")

	ExplorerCmd.Flags().BoolVar(&legacyOutput, "legacy", false, "legacy output")
	ExplorerCmd.Flags().MarkHidden("legacy")

	ExplorerCmd.Flags().SortFlags = false
}

// ExplorerCmd summarizes flows
var ExplorerCmd = &cobra.Command{
	Use:   "explorer",
	Short: "Export explorer traffic data enhanced with some additional information (e.g., subnet, default gateway, interface name, etc.).",
	Long: `
Export explorer traffic data enhanced with some additional information (e.g., subnet, default gateway, interface name, etc.).

See the flags for filtering options.

Use the following commands to get necessary HREFs for include/exlude files: label-export, ipl-export, wkld-export.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		// Set output to CSV only
		viper.Set("output_format", "csv")

		explorerExport()
	},
}

func explorerExport() {

	// Log start
	utils.LogStartCommand("explorer")

	// Create the default query struct
	tq := illumioapi.TrafficQuery{}

	// Check max results for valid value
	if maxResults < 1 || maxResults > 100000 {
		utils.LogError("max-results must be between 1 and 100000")
	}
	tq.MaxFLows = maxResults

	// Get LabelMap for getting workload labels
	_, err = pce.GetLabelMaps()
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get WorkloadMap by hostname
	whm, _, err = pce.GetWkldHostMap()
	if err != nil {
		utils.LogError(err.Error())
	}

	// Build policy status slice
	if !exclAllowed {
		tq.PolicyStatuses = append(tq.PolicyStatuses, "allowed")
	}
	if !exclPotentiallyBlocked {
		tq.PolicyStatuses = append(tq.PolicyStatuses, "potentially_blocked")
	}
	if !exclBlocked {
		tq.PolicyStatuses = append(tq.PolicyStatuses, "blocked")
	}
	if !exclAllowed && !exclPotentiallyBlocked && !exclBlocked {
		tq.PolicyStatuses = []string{}
	}

	// Get the start date
	tq.StartTime, err = time.Parse(fmt.Sprintf("2006-01-02 MST"), fmt.Sprintf("%s %s", start, "UTC"))
	if err != nil {
		utils.LogError(err.Error())
	}
	tq.StartTime = tq.StartTime.In(time.UTC)

	// Get the end date
	tq.EndTime, err = time.Parse(fmt.Sprintf("2006-01-02 15:04:05 MST"), fmt.Sprintf("%s 23:59:59 %s", end, "UTC"))
	if err != nil {
		utils.LogError(err.Error())
	}
	tq.EndTime = tq.EndTime.In(time.UTC)

	// Get the services
	if exclServiceCSV != "" {
		tq.PortProtoExclude, err = utils.GetServicePortsCSV(exclServiceCSV)
		if err != nil {
			utils.LogError(err.Error())
		}
	}
	if inclServiceCSV != "" {
		tq.PortProtoInclude, err = utils.GetServicePortsCSV(inclServiceCSV)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Get the source and destination include/exclude hrefs
	files := []string{inclHrefSrcFile, inclHrefDstFile, exclHrefSrcFile, exclHrefDstFile}
	targets := []*[]string{&tq.SourcesInclude, &tq.DestinationsInclude, &tq.SourcesExclude, &tq.DestinationsExclude}
	for i, f := range files {
		if f == "" {
			continue
		}
		d, err := utils.ParseCSV(f)
		if err != nil {
			utils.LogError(err.Error())
		}
		new := []string{}
		for _, n := range d {
			new = append(new, n[0])
		}
		*targets[i] = append(*targets[i], new...)
	}

	// Exclude broadcast and multicast, unless flag set to include non-unicast flows
	if !nonUni {
		tq.TransmissionExcludes = []string{"broadcast", "multicast"}
	}

	// Get the iterative list
	iterateList := [][]string{}
	if loopFile != "" {
		d, err := utils.ParseCSV(loopFile)
		if err != nil {
			utils.LogError(err.Error())
		}

		for _, n := range d {
			// If it's not an HREF, skip it since it's a header file.
			if !strings.Contains(n[0], "/orgs/") {
				continue
			}
			iterateList = append(iterateList, n)
		}
	}

	// Set some variables for traffic analysis
	var traffic, traffic2 []illumioapi.TrafficAnalysis
	var a illumioapi.APIResponse

	// If we aren't iterating - generate
	if len(iterateList) == 0 {
		traffic, a, err = pce.GetTrafficAnalysis(tq)
		utils.LogInfo("making single explorer query", false)
		utils.LogInfo(a.ReqBody, false)
		utils.LogAPIResp("GetTrafficAnalysis", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		outFileName := fmt.Sprintf("workloader-explorer-%s.csv", time.Now().Format("20060102_150405"))

		// Consolidate if needed
		originalFlowCount := len(traffic)
		if consolidate {
			cf := consolidateFlows(traffic)
			traffic = nil
			traffic = append(traffic, cf...)
		}
		createExplorerCSV(outFileName, traffic)
		if consolidate {
			utils.LogInfo(fmt.Sprintf("%d consolidated traffic records exported from %d total records", len(traffic), originalFlowCount), true)
		} else {
			utils.LogInfo(fmt.Sprintf("%d traffic records exported", len(traffic)), true)
		}
		// Log end
		utils.LogEndCommand("explorer")
		return
	}

	// Get here if we are iterating.
	for i, labels := range iterateList {

		// Build the new query struct
		newTQ := tq

		// Add the labels to the include and build a string for logging
		logString := []string{}
		fileName := []string{}
		for _, label := range labels {
			newTQ.SourcesInclude = append(newTQ.SourcesInclude, label)
			logString = append(logString, fmt.Sprintf("%s(%s)", pce.LabelMapH[label].Value, pce.LabelMapH[label].Key))
			fileName = append(fileName, pce.LabelMapH[label].Value)
		}

		// Log the query
		utils.LogInfo(fmt.Sprintf("Querying label set %d of %d - %s", i+1, len(iterateList), strings.Join(logString, ";")), true)

		// Run the first traffic query with the app as a source
		traffic, a, err = pce.GetTrafficAnalysis(newTQ)
		utils.LogAPIResp("GetTrafficAnalysis", a)
		utils.LogInfo(fmt.Sprintf("making first explorer query for %s", strings.Join(logString, ";")), false)
		utils.LogInfo(a.ReqBody, false)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get ready for second query - restore original
		newTQ = tq

		// Set the destinations include with new labels and exclude the source since we already have it from first query
		for _, label := range labels {
			newTQ.DestinationsInclude = append(newTQ.DestinationsInclude, label)
			newTQ.SourcesExclude = append(newTQ.SourcesExclude, label)
		}

		traffic2, a, err = pce.GetTrafficAnalysis(newTQ)
		utils.LogAPIResp("GetTrafficAnalysis", a)
		utils.LogInfo(fmt.Sprintf("making second explorer query for %s", strings.Join(logString, ";")), false)
		utils.LogInfo(a.ReqBody, false)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Append the results
		combinedTraffic := append(traffic, traffic2...)

		// Consolidate if needed
		originalFlowCount := len(combinedTraffic)
		if consolidate {
			cf := consolidateFlows(combinedTraffic)
			combinedTraffic = nil
			combinedTraffic = append(combinedTraffic, cf...)
		}

		// Generate the CSV
		if len(combinedTraffic) > 0 {
			badChars := []string{"/", "\\", "$", "^", "&", "%", "!", "@", "#", "*", "(", ")", "{", "}", "[", "]", "~", "`"}
			f := strings.Join(fileName, "-")
			for _, b := range badChars {
				f = strings.ReplaceAll(f, b, "")
			}

			outFileName := fmt.Sprintf("workloader-explorer-%s-%s.csv", f, time.Now().Format("20060102_150405"))
			createExplorerCSV(outFileName, combinedTraffic)
			if consolidate {
				utils.LogInfo(fmt.Sprintf("%d consolidated traffic records exported from %d total records", len(combinedTraffic), originalFlowCount), true)
			}
			utils.LogInfo(fmt.Sprintf("Exported %d traffic records.", len(combinedTraffic)), true)
		} else {
			utils.LogInfo(fmt.Sprintln("No traffic records."), true)
		}
	}

	// Log end
	utils.LogEndCommand("explorer")
}

func wkldGW(hostname string, wkldHostMap map[string]illumioapi.Workload) string {
	if wkld, ok := wkldHostMap[hostname]; ok {
		return wkld.GetDefaultGW()
	}
	return "NA"
}

func wkldNetMask(hostname, ip string, wkldHostMap map[string]illumioapi.Workload) string {
	if wkld, ok := wkldHostMap[hostname]; ok {
		return wkld.GetNetMask(ip)
	}
	return "NA"
}

func wkldInterfaceName(hostname, ip string, wkldHostMap map[string]illumioapi.Workload) string {
	if wkld, ok := wkldHostMap[hostname]; ok {
		return wkld.GetInterfaceName(ip)
	}
	return "NA"
}

func consolidateFlows(trafficFlows []illumioapi.TrafficAnalysis) []illumioapi.TrafficAnalysis {
	cTraffic := make(map[string]illumioapi.TrafficAnalysis)
	for _, t := range trafficFlows {
		if val, ok := cTraffic[fmt.Sprintf("%s%s%d%d", t.Src.IP, t.Dst.IP, t.ExpSrv.Port, t.ExpSrv.Proto)]; !ok {
			t.PolicyDecision = fmt.Sprintf("%s (%d)", t.PolicyDecision, t.NumConnections)
			cTraffic[fmt.Sprintf("%s%s%d%d", t.Src.IP, t.Dst.IP, t.ExpSrv.Port, t.ExpSrv.Proto)] = t
		} else {
			// We already have an entry so we consolidate. Start by building the new with src, dst, port, and proto
			tNew := illumioapi.TrafficAnalysis{Src: t.Src, Dst: t.Dst, ExpSrv: &illumioapi.ExpSrv{Port: t.ExpSrv.Port, Proto: t.ExpSrv.Proto}}
			// New connections is the old value plus this new one.
			tNew.NumConnections = val.NumConnections + t.NumConnections
			// The time stamps are semi-colon separated
			tNew.TimestampRange = &illumioapi.TimestampRange{FirstDetected: fmt.Sprintf("%s; %s", val.TimestampRange.FirstDetected, t.TimestampRange.FirstDetected), LastDetected: fmt.Sprintf("%s; %s", val.TimestampRange.LastDetected, t.TimestampRange.LastDetected)}
			// Process, windows, service, and transmission type are also semi-colon separated
			tNew.ExpSrv.Process = fmt.Sprintf("%s; %s", val.ExpSrv.Process, t.ExpSrv.Process)
			tNew.ExpSrv.WindowsService = fmt.Sprintf("%s; %s", val.ExpSrv.WindowsService, t.ExpSrv.WindowsService)
			tNew.ExpSrv.User = fmt.Sprintf("%s; %s", val.ExpSrv.User, t.ExpSrv.User)
			tNew.Transmission = fmt.Sprintf("%s; %s", val.Transmission, t.Transmission)
			// Policy decision includes the flow counter
			tNew.PolicyDecision = fmt.Sprintf("%s; %s(%d)", val.PolicyDecision, t.PolicyDecision, t.NumConnections)
			// Replace the value
			cTraffic[fmt.Sprintf("%s%s%d%d", t.Src.IP, t.Dst.IP, t.ExpSrv.Port, t.ExpSrv.Proto)] = tNew
		}
	}

	var returnResults []illumioapi.TrafficAnalysis
	for _, t := range cTraffic {
		returnResults = append(returnResults, t)
	}
	return returnResults
}

func createExplorerCSV(filename string, traffic []illumioapi.TrafficAnalysis) {

	// Build our CSV structure
	data := [][]string{[]string{"src_ip", "src_interface_name", "src_net_mask", "src_default_gw", "src_hostname", "src_role", "src_app", "src_env", "src_loc", "src_app_group", "dst_ip", "dst_interface_name", "dst_net_mask", "dst_default_gw", "dst_hostname", "dst_role", "dst_app", "dst_env", "dst_loc", "dst_app_group", "port", "protocol", "process", "windows_service", "user", "transmission", "policy_status", "date_first", "date_last", "num_flows"}}

	if legacyOutput {
		data = [][]string{[]string{"src_ip", "src_interface_name", "src_net_mask", "src_default_gw", "src_hostname", "src_role", "src_app", "src_env", "src_loc", "src_app_group", "dst_ip", "dst_interface_name", "dst_net_mask", "dst_default_gw", "dst_hostname", "dst_role", "dst_app", "dst_env", "dst_loc", "dst_app_group", "port", "protocol", "policy_status", "date_first", "date_last", "num_flows"}}
	}

	// Add each traffic entry to the data slic
	for _, t := range traffic {
		src := []string{t.Src.IP, "NA", "NA", "NA", "NA", "NA", "NA", "NA", "NA", "NA"}
		if t.Src.Workload != nil {
			// Get the app group
			sag := t.Src.Workload.GetAppGroup(pce.LabelMapH)
			if appGroupLoc {
				sag = t.Src.Workload.GetAppGroupL(pce.LabelMapH)
			}
			src = []string{t.Src.IP, wkldInterfaceName(t.Src.Workload.Hostname, t.Src.IP, whm), wkldNetMask(t.Src.Workload.Hostname, t.Src.IP, whm), wkldGW(t.Src.Workload.Hostname, whm), t.Src.Workload.Hostname, t.Src.Workload.GetRole(pce.LabelMapH).Value, t.Src.Workload.GetApp(pce.LabelMapH).Value, t.Src.Workload.GetEnv(pce.LabelMapH).Value, t.Src.Workload.GetLoc(pce.LabelMapH).Value, sag}
		}

		// Destination
		dst := []string{t.Dst.IP, "NA", "NA", "NA", "NA", "NA", "NA", "NA", "NA", "NA"}
		if t.Dst.Workload != nil {
			// Get the app group
			dag := t.Dst.Workload.GetAppGroup(pce.LabelMapH)
			if appGroupLoc {
				dag = t.Src.Workload.GetAppGroupL(pce.LabelMapH)
			}
			dst = []string{t.Dst.IP, wkldInterfaceName(t.Dst.Workload.Hostname, t.Dst.IP, whm), wkldNetMask(t.Dst.Workload.Hostname, t.Dst.IP, whm), wkldGW(t.Dst.Workload.Hostname, whm), t.Dst.Workload.Hostname, t.Dst.Workload.GetRole(pce.LabelMapH).Value, t.Dst.Workload.GetApp(pce.LabelMapH).Value, t.Dst.Workload.GetEnv(pce.LabelMapH).Value, t.Dst.Workload.GetLoc(pce.LabelMapH).Value, dag}
		}

		// Set the transmission type variable
		transmissionType := t.Transmission
		if t.Transmission == "" {
			transmissionType = "unicast"
		}

		// Append source, destination, port, protocol, policy decision, time stamps, and number of connections to data
		protocols := illumioapi.ProtocolList()
		d := append(src, dst...)
		d = append(d, strconv.Itoa(t.ExpSrv.Port))
		d = append(d, protocols[t.ExpSrv.Proto])
		if !legacyOutput {
			d = append(d, t.ExpSrv.Process)
			d = append(d, t.ExpSrv.WindowsService)
			d = append(d, t.ExpSrv.User)
			d = append(d, transmissionType)
		}
		d = append(d, t.PolicyDecision)
		d = append(d, t.TimestampRange.FirstDetected)
		d = append(d, t.TimestampRange.LastDetected)
		d = append(d, strconv.Itoa(t.NumConnections))
		data = append(data, d)
	}
	utils.WriteOutput(data, data, filename)
}
