package appgroupflowsummary

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var app, start, end, outputFileName string
var exclAllowed, exclPotentiallyBlocked, exclBlocked, appGroupLoc, ignoreIPGroup, consolidate bool
var pce illumioapi.PCE
var err error

func init() {

	AppGroupFlowSummaryCmd.Flags().StringVarP(&app, "app", "a", "", "app name to limit Explorer results to flows with that app as a provider or a consumer. default is all apps.")
	AppGroupFlowSummaryCmd.Flags().StringVarP(&start, "start", "s", time.Now().AddDate(0, 0, -88).In(time.UTC).Format("2006-01-02"), "start date in the format of yyyy-mm-dd.")
	AppGroupFlowSummaryCmd.Flags().StringVarP(&end, "end", "e", time.Now().Add(time.Hour*24).Format("2006-01-02"), "end date in the format of yyyy-mm-dd.")
	AppGroupFlowSummaryCmd.Flags().BoolVar(&exclAllowed, "excl-allowed", false, "excludes allowed traffic flows.")
	AppGroupFlowSummaryCmd.Flags().BoolVar(&exclPotentiallyBlocked, "excl-potentially-blocked", false, "excludes potentially blocked traffic flows.")
	AppGroupFlowSummaryCmd.Flags().BoolVar(&exclBlocked, "excl-blocked", false, "excludes blocked traffic flows.")
	AppGroupFlowSummaryCmd.Flags().BoolVarP(&appGroupLoc, "appgrp-loc", "l", false, "use location in app group")
	AppGroupFlowSummaryCmd.Flags().BoolVarP(&ignoreIPGroup, "ignore-ip", "i", false, "exlude IP address app groups from output")
	AppGroupFlowSummaryCmd.Flags().BoolVarP(&consolidate, "consolidate", "c", false, "consolidate all communication between 2 app groups into one CSV entry. See description below for example of output formats.")
	AppGroupFlowSummaryCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	AppGroupFlowSummaryCmd.Flags().SortFlags = false

}

// AppGroupFlowSummaryCmd summarizes flows
var AppGroupFlowSummaryCmd = &cobra.Command{
	Use:   "appgroup-flow-summary",
	Short: "Summarize flows by port and protocol between app groups.",
	Long: `
Summarize flows by port and protocol between app groups.

Default output as each unique port/proto on a separet entry:
+------------------------------+------------------------------+-----------+---------------+---------------------------+---------------+
|        SRC APP GROUP         |        DST APP GROUP         |  SERVICE  | ALLOWED FLOWS | POTENTIALLY BLOCKED FLOWS | BLOCKED FLOWS |
+------------------------------+------------------------------+-----------+---------------+---------------------------+---------------+
| AssetManagement | Production | HREnrollment | Production    | 8070 TCP  |               | 30                        |               |
+------------------------------+------------------------------+-----------+---------------+---------------------------+---------------+
| AssetManagement | Production | HREnrollment | Production    | 3306 TCP  |               | 9                         |               |
+------------------------------+------------------------------+-----------+---------------+---------------------------+---------------+
| 45.54.45.54                  | eCommerce | Production       | 443 TCP   |               | 108                       |               |
+------------------------------+------------------------------+-----------+---------------+---------------------------+---------------+



Including the consolidate flag (--consolidate, -c) will combine all entries between two groups onto one line:
+------------------------------+------------------------------+----------------------+----------------------------------+----------------------+
|        SRC APP GROUP         |        DST APP GROUP         | ALLOWED FLOW SUMMARY | POTENTIALLY BLOCKED FLOW SUMMARY | BLOCKED FLOW SUMMARY |
+------------------------------+------------------------------+----------------------+----------------------------------+----------------------+
| AssetManagement | Production | HREnrollment | Production    |                      | 8070 TCP (30 flows);3306 TCP     |                      |
|                              |                              |                      | (9 flows)                        |                      |
+------------------------------+------------------------------+----------------------+----------------------------------+----------------------+
| 45.54.45.54                  | Point-of-Sale | Staging      |                      | 443 TCP (126 flows)              |                      |
+------------------------------+------------------------------+----------------------+----------------------------------+----------------------+

The update-pce and --no-prompt flags are ignored for this command.
`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		flowSummary()
	},
}

// A summary consists of a common policy status, source app group, and destination app group.
type summary struct {
	policyStatus string
	srcAppGroup  string
	dstAppGroup  string
}

// A svcSummary consists of a port and protocol and flow count
type svcSummary struct {
	port  int
	proto string
	count int
}

func flowSummary() {

	// Build policy status slice
	var pStatus []string
	if !exclAllowed {
		pStatus = append(pStatus, "allowed")
	}
	if !exclPotentiallyBlocked {
		pStatus = append(pStatus, "potentially_blocked")
	}
	if !exclBlocked {
		pStatus = append(pStatus, "blocked")
	}

	// Create the default query struct
	tq := illumioapi.TrafficQuery{
		PolicyStatuses:                  pStatus,
		MaxFLows:                        200000,
		ExcludeWorkloadsFromIPListQuery: true}

	// Get the start date
	tq.StartTime, err = time.Parse("2006-01-02 MST", fmt.Sprintf("%s %s", start, "UTC"))
	if err != nil {
		utils.LogError(err.Error())
	}
	tq.StartTime = tq.StartTime.In(time.UTC)

	// Get the end date
	tq.EndTime, err = time.Parse("2006-01-02 15:04:05 MST", fmt.Sprintf("%s 23:59:59 %s", end, "UTC"))
	if err != nil {
		utils.LogError(err.Error())
	}
	tq.EndTime = tq.EndTime.In(time.UTC)

	// If an app is provided, adjust query to include it
	if app != "" {
		utils.LogInfof(false, "app label value: %s", app)
		label, a, err := pce.GetLabelByKeyValue("app", app)
		utils.LogAPIRespV2("GetLabelbyKeyValue", a)
		if err != nil {
			utils.LogErrorf("getting label HREF - %s", err)
		}
		if label.Href == "" {
			utils.LogErrorf("%s does not exist as an app label.", app)
		}
		utils.LogInfof(false, "app label href: %s", label.Href)
		tq.SourcesInclude = [][]string{{label.Href}}
	}

	// Run traffic query
	traffic, a, err := pce.GetTrafficAnalysis(tq)
	utils.LogAPIRespV2("GetTrafficAnalysis", a)
	utils.LogInfof(false, "explorer query body: %s", a.ReqBody)
	if err != nil {
		utils.LogError(err.Error())
	}
	utils.LogInfof(false, "first traffic query result count: %d", len(traffic))

	// If app is provided, switch to the destination include, clear the sources include, run query again, append to previous result
	if app != "" {
		tq.DestinationsInclude = tq.SourcesInclude
		tq.SourcesInclude = [][]string{}
		traffic2, a, err := pce.GetTrafficAnalysis(tq)
		utils.LogAPIRespV2("GetTrafficAnalysis", a)
		utils.LogInfof(false, "explorer query body: %s", a.ReqBody)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("second traffic query result count: %d", len(traffic2)), false)
		traffic = append(traffic, traffic2...)
		utils.LogInfo(fmt.Sprintf("combined traffic query result count: %d", len(traffic)), false)
	}

	// Get the protocol list
	protoMap := illumioapi.ProtocolList()

	// Build the map of results
	entryMap := make(map[summary]map[svcSummary]int)

	// Cycle through the traffic results and build what we need
	for _, t := range traffic {
		var srcAppGroup, dstAppGroup string

		// Get src appgroup
		if t.Src.Workload == nil {
			if ignoreIPGroup {
				continue
			}
			srcAppGroup = t.Src.IP
		} else {
			srcAppGroup = t.Src.Workload.GetAppGroup(pce.Labels)
			if appGroupLoc {
				srcAppGroup = t.Src.Workload.GetAppGroupL(pce.Labels)
			}
		}

		// Get Dst appgroup
		if t.Dst.Workload == nil {
			if ignoreIPGroup {
				continue
			}
			dstAppGroup = t.Dst.IP
		} else {
			dstAppGroup = t.Dst.Workload.GetAppGroup(pce.Labels)
			if appGroupLoc {
				dstAppGroup = t.Dst.Workload.GetAppGroupL(pce.Labels)
			}
		}

		// Check if we already have this result captured. If we do, increment the flow counter
		entry := summary{srcAppGroup: srcAppGroup, dstAppGroup: dstAppGroup, policyStatus: t.PolicyDecision}
		if _, ok := entryMap[entry]; !ok {
			entryMap[entry] = make(map[svcSummary]int)
		}
		svc := svcSummary{port: t.ExpSrv.Port, proto: protoMap[t.ExpSrv.Proto]}
		//svc := fmt.Sprintf("%d %s", t.ExpSrv.Port, protoMap[t.ExpSrv.Proto])
		entryMap[entry][svc] = entryMap[entry][svc] + int(t.NumConnections)
	}

	// Build the data slices
	data := [][]string{{"src_app_group", "dst_app_group", "service", "allowed_flows", "potentially_blocked_flows", "blocked_flows"}}
	if consolidate {
		data = [][]string{{"src_app_group", "dst_app_group", "allowed_flow_summary", "potentially_blocked_flow_summary", "blocked_flow_summary"}}
	}

	// Cylce through our entry map, add flows to struct, sort, create dataEntry if consolidate is set, append to data
	for e, v := range entryMap {
		x := []svcSummary{}
		var dataEntry []string
		for a, b := range v {
			a.count = b
			x = append(x, a)

		}
		sort.Slice(x, func(i, j int) bool {
			return x[i].count > x[j].count
		})
		for _, i := range x {
			if consolidate {
				dataEntry = append(dataEntry, fmt.Sprintf("%d %s (%d flows)", i.port, i.proto, i.count))
			}
		}
		if consolidate {
			switch e.policyStatus {
			case "allowed":
				data = append(data, []string{e.srcAppGroup, e.dstAppGroup, strings.Join(dataEntry, ";"), "", ""})
			case "potentially_blocked":
				data = append(data, []string{e.srcAppGroup, e.dstAppGroup, "", strings.Join(dataEntry, ";"), ""})
			case "blocked":
				data = append(data, []string{e.srcAppGroup, e.dstAppGroup, "", "", strings.Join(dataEntry, ";")})
			}
		} else {
			switch e.policyStatus {
			case "allowed":
				for _, a := range x {
					data = append(data, []string{e.srcAppGroup, e.dstAppGroup, fmt.Sprintf("%d %s", a.port, a.proto), fmt.Sprintf("%d", a.count), "", ""})
				}
			case "potentially_blocked":
				for _, a := range x {
					data = append(data, []string{e.srcAppGroup, e.dstAppGroup, fmt.Sprintf("%d %s", a.port, a.proto), "", fmt.Sprintf("%d", a.count), ""})
				}
			case "blocked":
				for _, a := range x {
					data = append(data, []string{e.srcAppGroup, e.dstAppGroup, fmt.Sprintf("%d %s", a.port, a.proto), "", "", fmt.Sprintf("%d", a.count)})
				}
			}
		}
	}

	// Write the data
	if len(data) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-flowsummary-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(data, data, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d summaries exported.", len(data)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no explorer data to summarize", true)
	}

	utils.LogEndCommand("flowsummary appgroup")

}
