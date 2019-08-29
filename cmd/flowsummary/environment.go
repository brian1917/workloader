package flowsummary

// import (
// 	"fmt"
// 	"sort"
// 	"strings"
// 	"time"

// 	"github.com/brian1917/illumioapi"
// 	"github.com/brian1917/workloader/utils"
// 	"github.com/spf13/cobra"
// 	"github.com/spf13/viper"
// )

// var app, start, end string
// var exclAllowed, exclPotentiallyBlocked, exclBlocked, appGroupLoc, ignoreIPGroup, debug bool
// var pce illumioapi.PCE
// var err error

// func init() {

// 	AppGroupFlowSummaryCmd.Flags().StringVar(&start, "start", time.Date(time.Now().Year()-5, time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02"), "start date in the format of yyyy-mm-dd")
// 	AppGroupFlowSummaryCmd.Flags().StringVar(&end, "end", time.Now().Add(time.Hour*24).Format("2006-01-02"), "end date in the format of yyyy-mm-dd")
// 	AppGroupFlowSummaryCmd.Flags().BoolVar(&exclAllowed, "exclude-allowed", false, "excludes allowed traffic flows.")
// 	AppGroupFlowSummaryCmd.Flags().BoolVar(&exclPotentiallyBlocked, "exclude-potentially-blocked", false, "excludes potentially blocked traffic flows.")
// 	AppGroupFlowSummaryCmd.Flags().BoolVar(&exclBlocked, "exclude-blocked", false, "excludes blocked traffic flows.")
// 	AppGroupFlowSummaryCmd.Flags().BoolVar(&ignoreIPGroup, "ignore-ip", false, "exlude IP addresses traffic from output")
// 	AppGroupFlowSummaryCmd.Flags().SortFlags = false

// }

// // AppGroupFlowSummaryCmd summarizes flows
// var EnvFlowSummaryCmd = &cobra.Command{
// 	Use:   "envsummary",
// 	Short: "Summarize flows by port and protocol and role-app-env-loc between app groups.",
// 	Long: `
// Summarize flows by port and protocol between app groups.

// The start and end dates are set as midnight UTC.

// Example output will look like the following:
// +---------------------------+---------------------------+----------------------+----------------------------------+----------------------+
// |       SRC APP GROUP       |       DST APP GROUP       | ALLOWED FLOW SUMMARY | POTENTIALLY BLOCKED FLOW SUMMARY | BLOCKED FLOW SUMMARY |
// +---------------------------+---------------------------+----------------------+----------------------------------+----------------------+
// | 9.9.9.9                   | Ordering | Production     |                      | 443 TCP (14 flows)               |                      |
// +---------------------------+---------------------------+----------------------+----------------------------------+----------------------+
// | 85.151.14.15              | Ordering | Production     |                      | 443 TCP (28 flows)               |                      |
// +---------------------------+---------------------------+----------------------+----------------------------------+----------------------+
// | Ordering | Development    | Ordering | Development    |                      | 5432 TCP (168 flows);8080        |                      |
// |                           |                           |                      | TCP (168 flows);8070 TCP (168    |                      |
// |                           |                           |                      | flows)                           |                      |
// +---------------------------+---------------------------+----------------------+----------------------------------+----------------------+
// | Ordering | Production     | Point-of-Sale | Staging   |                      | 5432 TCP (56 flows)              |                      |
// +---------------------------+---------------------------+----------------------+----------------------------------+----------------------+

// The --update-pce and --no-prompt flags are ignored for this command.`,
// 	Run: func(cmd *cobra.Command, args []string) {

// 		pce, err = utils.GetPCE()
// 		if err != nil {
// 			utils.Log(1, err.Error())
// 		}

// 		// Get the debug value from viper
// 		debug = viper.Get("debug").(bool)

// 		flowSummary()
// 	},
// }

// // A summary consists of a common policy status, source app group, and destination app group.
// type summary struct {
// 	policyStatus string
// 	srcAppGroup  string
// 	dstAppGroup  string
// }

// // A svcSummary consists of a port and protocol and flow count
// type svcSummary struct {
// 	portProto string
// 	count     int
// }

// func flowSummary() {

// 	// Build policy status slice
// 	var pStatus []string
// 	if !exclAllowed {
// 		pStatus = append(pStatus, "allowed")
// 	}
// 	if !exclPotentiallyBlocked {
// 		pStatus = append(pStatus, "potentially_blocked")
// 	}
// 	if !exclBlocked {
// 		pStatus = append(pStatus, "blocked")
// 	}

// 	// Get the state and end date
// 	startDate, err := time.Parse(fmt.Sprintf("2006-01-02 MST"), fmt.Sprintf("%s %s", start, "UTC"))
// 	if err != nil {
// 		utils.Log(1, err.Error())
// 	}
// 	startDate = startDate.In(time.UTC)

// 	endDate, err := time.Parse(fmt.Sprintf("2006-01-02 MST"), fmt.Sprintf("%s %s", end, "UTC"))
// 	if err != nil {
// 		utils.Log(1, err.Error())
// 	}
// 	endDate = endDate.In(time.UTC)

// 	// Create the default query struct
// 	tq := illumioapi.TrafficQuery{
// 		StartTime:      startDate,
// 		EndTime:        endDate,
// 		PolicyStatuses: pStatus,
// 		MaxFLows:       100000}

// 	// If an app is provided, adjust query to include it
// 	if app != "" {
// 		label, a, err := pce.GetLabelbyKeyValue("app", app)
// 		if debug {
// 			utils.LogAPIResp("GetLabelbyKeyValue", a)
// 		}
// 		if err != nil {
// 			utils.Log(1, fmt.Sprintf("getting label HREF - %s", err))
// 		}
// 		if label.Href == "" {
// 			utils.Log(1, fmt.Sprintf("%s does not exist as an app label.", app))
// 		}
// 		tq.SourcesInclude = []string{label.Href}
// 	}

// 	// Run traffic query
// 	traffic, a, err := pce.GetTrafficAnalysis(tq)
// 	if debug {
// 		utils.LogAPIResp("GetTrafficAnalysis", a)
// 	}
// 	if err != nil {
// 		utils.Log(1, fmt.Sprintf("making explorer API call - %s", err))
// 	}

// 	// If app is provided, switch to the destination include, clear the sources include, run query again, append to previous result
// 	if app != "" {
// 		tq.DestinationsInclude = tq.SourcesInclude
// 		tq.SourcesInclude = []string{}
// 		traffic2, a, err := pce.GetTrafficAnalysis(tq)
// 		if debug {
// 			utils.LogAPIResp("GetTrafficAnalysis", a)
// 		}
// 		if err != nil {
// 			utils.Log(1, fmt.Sprintf("making second explorer API call - %s", err))
// 		}
// 		traffic = append(traffic, traffic2...)
// 	}

// 	// Get the label map
// 	labelMap, a, err := pce.GetLabelMapH()
// 	if debug {
// 		utils.LogAPIResp("GetLabelMapH", a)
// 	}
// 	if err != nil {
// 		utils.Log(1, err.Error())
// 	}

// 	// Get the protocol list
// 	protoMap := illumioapi.ProtocolList()

// 	// Build the map of results
// 	entryMap := make(map[summary]map[string]int)

// 	// Cycle through the traffic results and build what we need
// 	for _, t := range traffic {
// 		var srcAppGroup, dstAppGroup string

// 		// Get src appgroup
// 		if t.Src.Workload == nil {
// 			if ignoreIPGroup {
// 				continue
// 			}
// 			srcAppGroup = t.Src.IP
// 		} else {
// 			srcAppGroup = t.Src.Workload.GetAppGroup(labelMap)
// 			if appGroupLoc {
// 				srcAppGroup = t.Src.Workload.GetAppGroupL(labelMap)
// 			}
// 		}

// 		// Get Dst appgroup
// 		if t.Dst.Workload == nil {
// 			if ignoreIPGroup {
// 				continue
// 			}
// 			dstAppGroup = t.Dst.IP
// 		} else {
// 			dstAppGroup = t.Dst.Workload.GetAppGroup(labelMap)
// 			if appGroupLoc {
// 				dstAppGroup = t.Dst.Workload.GetAppGroupL(labelMap)
// 			}
// 		}

// 		// Check if we already have this result captured. If we do, increment the flow counter
// 		entry := summary{srcAppGroup: srcAppGroup, dstAppGroup: dstAppGroup, policyStatus: t.PolicyDecision}
// 		if _, ok := entryMap[entry]; !ok {
// 			entryMap[entry] = make(map[string]int)
// 		}
// 		svc := fmt.Sprintf("%d %s", t.ExpSrv.Port, protoMap[t.ExpSrv.Proto])
// 		entryMap[entry][svc] = entryMap[entry][svc] + t.NumConnections
// 	}

// 	// Build the data slices
// 	data := [][]string{[]string{"src_app_group", "dst_app_group", "allowed_flow_summary", "potentially_blocked_flow_summary", "blocked_flow_summary"}}

// 	for e, v := range entryMap {
// 		x := []svcSummary{}
// 		var portProtos []string
// 		for a, b := range v {
// 			x = append(x, svcSummary{portProto: a, count: b})

// 		}
// 		sort.Slice(x, func(i, j int) bool {
// 			return x[i].count > x[j].count
// 		})
// 		for _, i := range x {
// 			portProtos = append(portProtos, fmt.Sprintf("%s (%d flows)", i.portProto, i.count))
// 		}

// 		switch e.policyStatus {
// 		case "allowed":
// 			data = append(data, []string{e.srcAppGroup, e.dstAppGroup, strings.Join(portProtos, ";"), "", ""})
// 		case "potentially_blocked":
// 			data = append(data, []string{e.srcAppGroup, e.dstAppGroup, "", strings.Join(portProtos, ";"), ""})
// 		case "blocked":
// 			data = append(data, []string{e.srcAppGroup, e.dstAppGroup, "", "", strings.Join(portProtos, ";")})
// 		}
// 	}

// 	// Write the data
// 	if len(data) > 1 {
// 		utils.WriteOutput(data, data, fmt.Sprintf("workloader-flowsummary-%s.csv", time.Now().Format("20060102_150405")))
// 		fmt.Printf("\r\n%d summaries exported.\r\n\r\n", len(data)-1)
// 		utils.Log(0, fmt.Sprintf("flowsummary complete - %d summaries exported", len(data)-1))
// 	} else {
// 		// Log command execution for 0 results
// 		fmt.Println("no explorer data to summarize")
// 		utils.Log(0, "no explorer data to summarize")
// 	}

// }
