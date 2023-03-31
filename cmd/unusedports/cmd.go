package unusedports

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Global variables
var wkldInputFile, labelInputFile, outputFileName, queryDuration, exclHrefSrcFile, ignorePorts, resultsFile string
var maxFlows int
var pce illumioapi.PCE
var err error

func init() {

	UnusedPortsCmd.Flags().StringVarP(&wkldInputFile, "input-file", "i", "", "optional input file with list of hrefs for target workloads. recommended to use wkld-export to get href file.")
	UnusedPortsCmd.Flags().StringVarP(&labelInputFile, "label-file", "l", "", "optional input csv with labels to target workload query. see below for details. ")
	UnusedPortsCmd.Flags().IntVarP(&maxFlows, "max-flows", "m", 100, "max flows returned from explorer query. set to 100 by default to keep queries small since presence of flows is all that matters.")
	UnusedPortsCmd.Flags().StringVarP(&queryDuration, "query-duration", "d", "24h", "time for initial query. format must be in xh or xd where x is a number and h specifies hours or d specifies days. for example, 24h is 24 hours and 30d is 30 days.")
	UnusedPortsCmd.Flags().StringVarP(&exclHrefSrcFile, "excl-src-file", "x", "", "file with hrefs on separate lines to be used in as a consumer exclude. can be a csv with hrefs in first column. no headers")
	UnusedPortsCmd.Flags().StringVarP(&ignorePorts, "ignore-ports", "p", "49152-65535", "comma-separated list of port numbers or ranges to exclude.")
	UnusedPortsCmd.Flags().StringVarP(&resultsFile, "results", "r", "", "fileoutput from step 1 to get the traffic results.")
	UnusedPortsCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	UnusedPortsCmd.Flags().SortFlags = false
}

// UnusedPortsCmd runs the upload command
var UnusedPortsCmd = &cobra.Command{
	Use:   "unused-ports",
	Short: "Produce a report showing workloads with ports open that have no traffic to them.",
	Long: `
Produce a report showing workloads with ports open that have no traffic to them.

The command is run in two step:
1) Run the command with appropriate flags to generate a CSV with async query hrefs.
2) Feed that file back into workloader unused-port with only the --results (-r) flag. 

Step 2 must be run within 24 hours of step 1 to ensure explorer queries do not expire.

If no input file or label file are used all workloads are processed. The header row should be label keys. The workload query uses an AND operator for entries on the same row and an OR operator for the separate rows. An example label file is below:
+------+-----+-----+-----+----+
| role | app | env | loc | bu |
+------+-----+-----+-----+----+
| web  | erp |     |     |    |
|      |     |     | bos | it |
|      | crm |     |     |    |
+------+-----+-----+-----+----+
This example queries all workloads that are
- web (role) AND erp (app) 
- OR bos(loc) AND it (bu)
- OR CRM (app)

The update-pce and --no-prompt flags are ignored for this command.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		unusedPorts()
	},
}

func unusedPorts() {

	// Log Start of command
	utils.LogStartCommand("unused-ports")

	if resultsFile != "" {
		getResults(resultsFile)
		utils.LogEndCommand("unused-ports")
		return

	}

	// Get the output file name ready
	if outputFileName == "" {
		outputFileName = fmt.Sprintf("workloader-unused-ports-%s.csv", time.Now().Format("20060102_150405"))
	}

	// Check query- uration
	if len(strings.Split(queryDuration, "h")) != 2 && len(strings.Split(queryDuration, "d")) != 2 {
		utils.LogError("invalid format for query-duration. see help menu for acceptable formats.")
	}

	// Get query time paramaters
	endTime := time.Now().In(time.UTC)

	var startTime time.Time

	if len(strings.Split(queryDuration, "d")) == 2 {
		delta, err := strconv.Atoi(strings.Split(queryDuration, "d")[0])
		if err != nil {
			utils.LogError("invalid format for query-duration. see help menu for acceptable formats.")
		}
		startTime = endTime.AddDate(0, 0, -1*delta)
	}
	if len(strings.Split(queryDuration, "h")) == 2 {
		delta, err := strconv.Atoi(strings.Split(queryDuration, "h")[0])
		if err != nil {
			utils.LogError("invalid format for first-query-duration. see help menu for acceptable formats.")
		}
		startTime = endTime.Add(time.Hour * time.Duration(delta*-1))
	}

	utils.LogInfo(fmt.Sprintf("explorer query start time: %s", startTime.String()), true)
	utils.LogInfo(fmt.Sprintf("explorer query end time: %s", endTime.String()), true)

	// Process the excludes
	exclSources := []string{}
	if exclHrefSrcFile != "" {
		// Parse the file
		d, err := utils.ParseCSV(exclHrefSrcFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		// For each entry in the file, add an exclude - OR operator
		for _, entry := range d {
			exclSources = append(exclSources, entry[0])
		}
	}
	sources, err := illumioapi.CreateIncludeOrExclude(exclSources, true)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create slice for target workloads
	wklds := []illumioapi.Workload{}

	// If an input file is provided
	if wkldInputFile != "" {
		inputHrefs, err := utils.ParseCSV(wkldInputFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		for _, l := range inputHrefs {
			wklds = append(wklds, illumioapi.Workload{Href: l[0]})
		}
	} else {
		// Create the QP map
		qp := map[string]string{"managed": "true"}

		// Process label file
		var labelData [][]string
		if labelInputFile != "" {
			labelData, err = utils.ParseCSV(labelInputFile)
			if err != nil {
				utils.LogError(err.Error())
			}

			// Get the labelQuery
			qp["labels"], err = pce.WorkloadQueryLabelParameter(labelData)
			if err != nil {
				utils.LogError(err.Error())
			}
		}

		// Get the workloads
		var a illumioapi.APIResponse
		a, err = pce.GetWklds(qp)
		utils.LogAPIRespV2("GetWklds", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		wklds = pce.WorkloadsSlice
	}

	// Log our target list
	utils.LogInfo(fmt.Sprintf("identified %d workloads to check open port usage.", len(wklds)), true)

	// Process the ignore map
	ignorePortsMap := make(map[int]bool)
	ignorePorts = strings.Replace(ignorePorts, ", ", ",", -1)
	for _, port := range strings.Split(ignorePorts, ",") {
		if strings.Contains(port, "-") {
			from, err := strconv.Atoi(strings.Split(port, "-")[0])
			if err != nil {
				utils.LogError(fmt.Sprintf("%s is an invalid port range", port))
			}
			to, err := strconv.Atoi(strings.Split(port, "-")[1])
			if err != nil {
				utils.LogError(fmt.Sprintf("%s is an invalid port range", port))
			}
			for x := from; x <= to; x++ {
				ignorePortsMap[x] = true
			}
		} else {
			portInt, err := strconv.Atoi(port)
			if err != nil {
				utils.LogError(fmt.Sprintf("%s is an invalid port range", port))
			}
			ignorePortsMap[portInt] = true
		}

	}

	// Start the file
	utils.WriteLineOutput([]string{"hostname", "href", "port", "protocol", "async_query_href", "async_query_status", "flows"}, outputFileName)

	// Process each workload
	for i, w := range wklds {

		utils.LogInfo(fmt.Sprintf("workload %d of %d ...", i+1, len(wklds)), true)

		// Get the individual workload so we can see the services (not available in bulk GET)
		wkld, a, err := pce.GetWkldByHref(w.Href)
		utils.LogAPIRespV2("GetWKldByHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Iterate through the open ports
		if wkld.Services == nil {
			continue
		}

		for serviceCounter, servicePort := range illumioapi.PtrToVal(wkld.Services.OpenServicePorts) {
			// Skip if should be ignoring that port
			if ignorePortsMap[servicePort.Port] {
				continue
			}

			tr := illumioapi.TrafficAnalysisRequest{
				QueryName:                       illumioapi.Ptr(fmt.Sprintf("%s - %d %d", w.Href, servicePort.Port, servicePort.Protocol)),
				Sources:                         &illumioapi.SrcOrDst{Exclude: sources},
				Destinations:                    &illumioapi.SrcOrDst{Exclude: []illumioapi.IncludeOrExclude{{Transmission: "broadcast"}, {Transmission: "multicast"}}, Include: [][]illumioapi.IncludeOrExclude{{illumioapi.IncludeOrExclude{Workload: &illumioapi.Workload{Href: w.Href}}}}},
				MaxResults:                      maxFlows,
				StartDate:                       startTime,
				EndDate:                         endTime,
				PolicyDecisions:                 &[]string{},
				ExcludeWorkloadsFromIPListQuery: illumioapi.Ptr(true),
				ExplorerServices:                &illumioapi.ExplorerServices{Exclude: []illumioapi.IncludeOrExclude{}, Include: []illumioapi.IncludeOrExclude{{Port: servicePort.Port, Proto: servicePort.Protocol}}},
			}
			if len(sources) == 0 {
				tr.Sources = &illumioapi.SrcOrDst{Include: [][]illumioapi.IncludeOrExclude{}, Exclude: []illumioapi.IncludeOrExclude{}}
			} else {
				tr.Sources = &illumioapi.SrcOrDst{Include: [][]illumioapi.IncludeOrExclude{}, Exclude: sources}
			}

			// Make the traffic request
			asyncTrafficQuery, a, err := pce.CreateAsyncTrafficRequest(tr)
			utils.LogAPIRespV2("GetTrafficAnalysisAPI", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("workload %s - %s - port %d of %d - %d %s - %s", illumioapi.PtrToVal(w.Hostname), w.Href, serviceCounter+1, len(illumioapi.PtrToVal(wkld.Services.OpenServicePorts)), servicePort.Port, illumioapi.ProtocolList()[servicePort.Protocol], asyncTrafficQuery.Href), true)

			utils.WriteLineOutput([]string{illumioapi.PtrToVal(w.Hostname), w.Href, strconv.Itoa(servicePort.Port), illumioapi.ProtocolList()[servicePort.Protocol], asyncTrafficQuery.Href, "", ""}, outputFileName)
		}
	}

	utils.LogEndCommand("unused-ports")
}
