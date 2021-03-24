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

var app, env, inclHrefDstFile, exclHrefDstFile, inclHrefSrcFile, exclHrefSrcFile, exclServiceObj, inclServiceCSV, exclServiceCSV, start, end, loopFile, outputFileName string
var exclAllowed, exclPotentiallyBlocked, exclBlocked, appGroupLoc, ignoreIPGroup, consolidate, nonUni, debug, legacyOutput, consAndProvierOnLoop bool
var maxResults, iterativeThreshold int
var pce illumioapi.PCE
var err error
var whm map[string]illumioapi.Workload

func init() {

	ExplorerCmd.Flags().StringVarP(&loopFile, "loop-label-file", "l", "", "file with columns of label hrefs on separate lines (without header). An explorer query for the label(s) as consumer OR provider is run for each app. For example, to iterate on app group, put the app label href in the first column and the environment href in the second column.")
	ExplorerCmd.Flags().BoolVarP(&consAndProvierOnLoop, "consumer-and-provider", "z", false, "when looping, run two queries - one as the consumer and another as the provider and de-dupe")
	ExplorerCmd.Flags().StringVarP(&inclHrefDstFile, "incl-dst-file", "a", "", "file with hrefs on separate lines to be used in as a provider include. Each line is treated as OR logic. On same line, combine hrefs of same object type for an AND logic. Headers optional")
	ExplorerCmd.Flags().StringVarP(&exclHrefDstFile, "excl-dst-file", "b", "", "file with hrefs on separate lines to be used in as a provider exclude. Can be a csv with hrefs in first column. Headers optional")
	ExplorerCmd.Flags().StringVarP(&inclHrefSrcFile, "incl-src-file", "c", "", "file with hrefs on separate lines to be used in as a consumer include. Each line is treated as OR logic. On same line, combine hrefs of same object type for an AND logic. Headers optional")
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
	ExplorerCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename. If iterating through labels, the labels will be appended to the provided name before the provided file extension. To name the files for the labels, use just an extension (--output-file .csv).")
	ExplorerCmd.Flags().IntVar(&iterativeThreshold, "iterative-query-threshold", 0, "If set greater than 0, workloader will run iterative explorer queries to maximize the return records. (Not advisable for most usecases).")

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

	// Run some checks on iterative query value
	if iterativeThreshold > 0 && iterativeThreshold > maxResults {
		utils.LogError("iterative-query-threshold must be less than or equal to max results")
	}
	if float64(iterativeThreshold) > 0.9*float64(maxResults) {
		utils.LogWarning("recommended to set iterative-query-threshold lower than 90% of max results.", true)
	}

	// Create the default query struct
	tq := illumioapi.TrafficQuery{}

	// Check max results for valid value
	if maxResults < 1 || maxResults > 100000 {
		utils.LogError("max-results must be between 1 and 100000")
	}
	tq.MaxFLows = maxResults

	// Get Labels and workloads
	if err := pce.Load(illumioapi.LoadInput{Labels: true, Workloads: true}); err != nil {
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

	// Get the Include Source
	if inclHrefSrcFile != "" {
		// Parse the file
		d, err := utils.ParseCSV(inclHrefSrcFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		// For each entry in the file, add an include - OR operator
		// Semi-colons are used to differentiate hrefs in the same include - AND operator.
		for _, entry := range d {
			tq.SourcesInclude = append(tq.SourcesInclude, strings.Split(strings.ReplaceAll(entry[0], "; ", ";"), ";"))
		}
	} else {
		tq.SourcesInclude = append(tq.SourcesInclude, make([]string, 0))
	}

	// Get the Include Destination
	if inclHrefDstFile != "" {
		// Parse the file
		d, err := utils.ParseCSV(inclHrefDstFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		// For each entry in the file, add an include - OR operator
		// Semi-colons are used to differentiate hrefs in the same include - AND operator.
		for _, entry := range d {
			tq.DestinationsInclude = append(tq.DestinationsInclude, strings.Split(strings.ReplaceAll(entry[0], "; ", ";"), ";"))
		}
	} else {
		tq.DestinationsInclude = append(tq.DestinationsInclude, make([]string, 0))
	}

	// Get the Exclude Sources
	if exclHrefSrcFile != "" {
		// Parse the file
		d, err := utils.ParseCSV(exclHrefSrcFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		// For each entry in the file, add an exclude - OR operator
		for _, entry := range d {
			tq.SourcesExclude = append(tq.SourcesExclude, entry[0])
		}
	}

	// Get the Exclude Destinations
	if exclHrefDstFile != "" {
		// Parse the file
		d, err := utils.ParseCSV(exclHrefDstFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		// For each entry in the file, add an exclude - OR operator
		for _, entry := range d {
			tq.DestinationsExclude = append(tq.DestinationsExclude, entry[0])
		}
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
		if iterativeThreshold == 0 {
			traffic, a, err = pce.GetTrafficAnalysis(tq)
			utils.LogInfo("making single explorer query", false)
			utils.LogInfo(a.ReqBody, false)
			utils.LogAPIResp("GetTrafficAnalysis", a)
		} else {
			illumioapi.Threshold = iterativeThreshold
			traffic, err = pce.IterateTraffic(tq, true)
		}
		if err != nil {
			utils.LogError(err.Error())
		}

		outFileName := fmt.Sprintf("workloader-explorer-%s.csv", time.Now().Format("20060102_150405"))
		if outputFileName != "" {
			outFileName = outputFileName
		}

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

		// Append the combination of labels. Each combination of labels is it's own include (if there is multiple, they are an AND)
		newTQ.SourcesInclude = append(newTQ.SourcesInclude, labels)

		// Build the file name and log string by iterating over all the labels
		var logEntries, fileNameEntries []string
		for _, label := range labels {
			logEntries = append(logEntries, fmt.Sprintf("%s(%s)", pce.Labels[label].Value, pce.Labels[label].Key))
			fileNameEntries = append(fileNameEntries, pce.Labels[label].Value)
		}

		// Log the query
		utils.LogInfo(fmt.Sprintf("Querying label set %d of %d - %s as a source", i+1, len(iterateList), strings.Join(logEntries, ";")), true)

		// Run the first traffic query with the app as a source
		if iterativeThreshold == 0 {
			traffic, a, err = pce.GetTrafficAnalysis(newTQ)
			utils.LogAPIResp("GetTrafficAnalysis", a)
			utils.LogInfo(a.ReqBody, false)

		} else {
			traffic, err = pce.IterateTraffic(newTQ, true)
		}
		if err != nil {
			utils.LogError(err.Error())
		}

		if consAndProvierOnLoop {

			// Run the second traffic query wth the app as a desitnation
			newTQ.SourcesInclude = tq.SourcesInclude
			newTQ.DestinationsInclude = append(newTQ.SourcesInclude, labels)

			// Log the query
			utils.LogInfo(fmt.Sprintf("Querying label set %d of %d - %s as a destination and depduping from source query", i+1, len(iterateList), strings.Join(logEntries, ";")), true)

			// Run the first traffic query with the app as a source
			if iterativeThreshold == 0 {
				traffic2, a, err = pce.GetTrafficAnalysis(newTQ)
				utils.LogAPIResp("GetTrafficAnalysis", a)
				utils.LogInfo(a.ReqBody, false)
			} else {
				traffic2, err = pce.IterateTraffic(newTQ, true)
			}
			if err != nil {
				utils.LogError(err.Error())
			}

			// Now we need to de-dupe traffic1 and traffic 2
			dedupedTraffic := illumioapi.DedupeExplorerTraffic(traffic, traffic2)
			traffic = nil
			traffic = dedupedTraffic
		}

		// Consolidate if needed
		originalFlowCount := len(traffic)
		if consolidate {
			cf := consolidateFlows(traffic)
			traffic = nil
			traffic = append(traffic, cf...)
		}

		// Generate the CSV
		if len(traffic) > 0 {
			badChars := []string{"/", "\\", "$", "^", "&", "%", "!", "@", "#", "*", "{", "}", "[", "]", "~", "`"}
			f := strings.Join(fileNameEntries, "-")
			for _, b := range badChars {
				f = strings.ReplaceAll(f, b, "")
			}

			outFileName := fmt.Sprintf("workloader-explorer-%s-%s.csv", f, time.Now().Format("20060102_150405"))
			if outputFileName != "" {
				// Split it on periods
				x := strings.Split(outputFileName, ".")
				// Get the extension
				ext := x[len(x)-1]
				// Remove the extension from x
				x = x[:len(x)-1]
				// Rejoin the remaining and append the app
				if x[0] == "" {
					outFileName = fmt.Sprintf("%s.%s", f, ext)
				} else {
					outFileName = fmt.Sprintf("%s-%s.%s", strings.Join(x, "."), f, ext)
				}

				// Remove leading "-" if it exists
			}
			createExplorerCSV(outFileName, traffic)
			if consolidate {
				utils.LogInfo(fmt.Sprintf("%d consolidated traffic records exported from %d total records", len(traffic), originalFlowCount), true)
			}
			utils.LogInfo(fmt.Sprintf("Exported %d traffic records.", len(traffic)), true)
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
			sag := t.Src.Workload.GetAppGroup(pce.Labels)
			if appGroupLoc {
				sag = t.Src.Workload.GetAppGroupL(pce.Labels)
			}
			src = []string{t.Src.IP, wkldInterfaceName(t.Src.Workload.Hostname, t.Src.IP, whm), wkldNetMask(t.Src.Workload.Hostname, t.Src.IP, whm), wkldGW(t.Src.Workload.Hostname, whm), t.Src.Workload.Hostname, t.Src.Workload.GetRole(pce.Labels).Value, t.Src.Workload.GetApp(pce.Labels).Value, t.Src.Workload.GetEnv(pce.Labels).Value, t.Src.Workload.GetLoc(pce.Labels).Value, sag}
		}

		// Destination
		dst := []string{t.Dst.IP, "NA", "NA", "NA", "NA", "NA", "NA", "NA", "NA", "NA"}
		if t.Dst.Workload != nil {
			// Get the app group
			dag := t.Dst.Workload.GetAppGroup(pce.Labels)
			if appGroupLoc {
				dag = t.Src.Workload.GetAppGroupL(pce.Labels)
			}
			dst = []string{t.Dst.IP, wkldInterfaceName(t.Dst.Workload.Hostname, t.Dst.IP, whm), wkldNetMask(t.Dst.Workload.Hostname, t.Dst.IP, whm), wkldGW(t.Dst.Workload.Hostname, whm), t.Dst.Workload.Hostname, t.Dst.Workload.GetRole(pce.Labels).Value, t.Dst.Workload.GetApp(pce.Labels).Value, t.Dst.Workload.GetEnv(pce.Labels).Value, t.Dst.Workload.GetLoc(pce.Labels).Value, dag}
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
