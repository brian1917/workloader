package unusedports

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Global variables
var inputFile, role, app, env, loc, outputFileName, firstQueryDuration, exclHrefSrcFile string
var maxFlows, secondQueryDays int
var pce illumioapi.PCE
var err error

func init() {

	UnusedPortsCmd.Flags().StringVarP(&inputFile, "input-file", "i", "", "optional input file with list of hrefs for target workloads. recommended to use wkld-export to get href file.")
	UnusedPortsCmd.Flags().IntVarP(&maxFlows, "max-flows", "m", 100, "max flows returned from explorer query. set to 100 by default to keep queries small since presence of flows is all that matters.")
	UnusedPortsCmd.Flags().StringVarP(&firstQueryDuration, "first-query-duration", "f", "24h", "time for initial query. format must be in xh or xd where x is a number and h specifies hours or d specifies days. for example, 24h is 24 hours and 30d is 30 days.")
	UnusedPortsCmd.Flags().IntVarP(&secondQueryDays, "second-query-days", "s", 30, "number of days to check back if the initial query yields 0 results.")
	UnusedPortsCmd.Flags().StringVarP(&role, "role", "r", "", "optional role label for target workloads. if an input file is provided the labels are ignored. multiple label flags are an AND operator.")
	UnusedPortsCmd.Flags().StringVarP(&app, "app", "a", "", "optional app label for target workloads. if an input file is provided the labels are ignored. multiple label flags are an AND operator.")
	UnusedPortsCmd.Flags().StringVarP(&env, "env", "e", "", "optional env label for target workloads. if an input file is provided the labels are ignored. multiple label flags are an AND operator.")
	UnusedPortsCmd.Flags().StringVarP(&loc, "loc", "l", "", "optional role label for target workloads. if an input file is provided the labels are ignored. multiple label flags are an AND operator.")
	UnusedPortsCmd.Flags().StringVarP(&exclHrefSrcFile, "excl-src-file", "d", "", "file with hrefs on separate lines to be used in as a consumer exclude. can be a csv with hrefs in first column. no headers")
	UnusedPortsCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	UnusedPortsCmd.Flags().SortFlags = false
}

// UnusedPortsCmd runs the upload command
var UnusedPortsCmd = &cobra.Command{
	Use:   "unused-ports",
	Short: "Produce a report showing workloads with ports open that have no traffic to them.",
	Long: `
Produce a report showing workloads with ports open that have no traffic to them.

If no input file or label flags are used all workloads are processed.

The update-pce and --no-prompt flags are ignored for this command.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		unusedPorts()
	},
}

func unusedPorts() {

	// Log Start of command
	utils.LogStartCommand("unused-ports")

	// Get the output file name ready
	if outputFileName == "" {
		outputFileName = fmt.Sprintf("workloader-unused-ports-%s.csv", time.Now().Format("20060102_150405"))
	}

	// Check secondQuery days
	if secondQueryDays > 90 {
		utils.LogError("second-query-days cannot be more than 90")
	}
	// Check first-query-duration
	if len(strings.Split(firstQueryDuration, "h")) != 2 && len(strings.Split(firstQueryDuration, "d")) != 2 {
		utils.LogError("invalid format for first-query-duration. see help menu for acceptable formats.")
	}

	// Get query time paramaters
	endTime := time.Now().In(time.UTC)

	var startTime time.Time

	if len(strings.Split(firstQueryDuration, "d")) == 2 {
		delta, err := strconv.Atoi(strings.Split(firstQueryDuration, "d")[0])
		if err != nil {
			utils.LogError("invalid format for first-query-duration. see help menu for acceptable formats.")
		}
		startTime = endTime.AddDate(0, 0, -1*delta)
	}
	if len(strings.Split(firstQueryDuration, "h")) == 2 {
		delta, err := strconv.Atoi(strings.Split(firstQueryDuration, "h")[0])
		if err != nil {
			utils.LogError("invalid format for first-query-duration. see help menu for acceptable formats.")
		}
		startTime = endTime.Add(time.Hour * time.Duration(delta*-1))
	}

	secondQueryStartTime := startTime.AddDate(0, 0, -1*secondQueryDays).In(time.UTC)

	utils.LogInfo(fmt.Sprintf("explorer query start time: %s", startTime.String()), true)
	utils.LogInfo(fmt.Sprintf("explorer query end time: %s", endTime.String()), true)
	utils.LogInfo(fmt.Sprintf("second explorer query start time (if necessary): %s", secondQueryStartTime.String()), true)

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

	// Create slice for target workloads
	wklds := []illumioapi.Workload{}

	// If an input file is provided
	if inputFile != "" {
		inputHrefs, err := utils.ParseCSV(inputFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		for _, l := range inputHrefs {
			wklds = append(wklds, illumioapi.Workload{Href: l[0]})
		}
	} else {
		// Create the QP map
		qp := map[string]string{"managed": "true"}
		// Get the labelQuery
		qp["labels"], err = pce.WorkloadQueryLabelParameter([][]string{{"role", "app", "env", "loc"}, {role, app, env, loc}})
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the workloads
		var a illumioapi.APIResponse
		wklds, a, err = pce.GetWklds(qp)
		utils.LogAPIResp("GetWklds", a)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Log our target list
	utils.LogInfo(fmt.Sprintf("identified %d workloads to check open port usage.", len(wklds)), true)

	// Start a switch for header line being written. This is so we don't produce and output file if there is no data
	headerWritten := false

	// Process each workload
	for i, w := range wklds {
		// fmt.Printf("\r%s [INFO] - querying open ports and trafic for %d of %d workloads", time.Now().Format("2006-01-02 15:04:05 "), i+1, len(wklds))
		unusedPorts := []string{}

		// Get the individual workload so we can see the services (not available in bulk GET)
		wkld, a, err := pce.GetWkldByHref(w.Href)
		utils.LogAPIResp("GetWKldByHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Iterate through the open ports
		if wkld.Services == nil {
			continue
		}

		for serviceCounter, servicePort := range wkld.Services.OpenServicePorts {

			utils.LogInfo(fmt.Sprintf("querying port %d of %d for workload %d of %d - port %d %s on %s", serviceCounter+1, len(wkld.Services.OpenServicePorts), i+1, len(wklds), servicePort.Port, illumioapi.ProtocolList()[servicePort.Protocol], w.Href), true)

			tq := illumioapi.TrafficQuery{
				DestinationsInclude:             [][]string{{wkld.Href}},
				MaxFLows:                        maxFlows,
				PortProtoInclude:                [][2]int{{servicePort.Port, servicePort.Protocol}},
				TransmissionExcludes:            []string{"broadcast", "multicast"},
				StartTime:                       startTime,
				EndTime:                         endTime,
				PolicyStatuses:                  []string{"allowed", "blocked", "potentially_blocked", "unknown"},
				ExcludeWorkloadsFromIPListQuery: true,
				SourcesExclude:                  exclSources,
			}

			// Run the query
			traffic, a, err := pce.GetTrafficAnalysis(tq)
			utils.LogAPIResp("GetTrafficAnalysis", a)
			if err != nil {
				utils.LogError(err.Error())
			}

			// Run the query again if there are no results. This time for user configurable time
			if len(traffic) == 0 {
				tq.StartTime = secondQueryStartTime
				// Run the second query
				traffic, a, err = pce.GetTrafficAnalysis(tq)
				utils.LogAPIResp("GetTrafficAnalysis", a)
				if err != nil {
					utils.LogError(err.Error())
				}
			}

			// If the query returns 0 results, append to the unused ports slice
			if len(traffic) == 0 {
				unusedPorts = append(unusedPorts, fmt.Sprintf("%d %s", servicePort.Port, illumioapi.ProtocolList()[servicePort.Protocol]))
			}
		}

		// Append results to output data slice
		// If the header hasn't been written yet (first finding) - create the header row
		if len(unusedPorts) > 0 {
			if !headerWritten {
				utils.WriteLineOutput([]string{"hostname", "href", "role", "app", "env", "loc", "unused_ports"}, outputFileName)
				headerWritten = true
			}
			utils.LogInfo(fmt.Sprintf("%s - %s - %d unused ports. adding to csv", wkld.Hostname, wkld.Href, len(unusedPorts)), true)
			utils.WriteLineOutput([]string{wkld.Hostname, wkld.Href, wkld.GetRole(pce.Labels).Value, wkld.GetApp(pce.Labels).Value, wkld.GetEnv(pce.Labels).Value, wkld.GetLoc(pce.Labels).Value, strings.Join(unusedPorts, "; ")}, outputFileName)
		} else {
			utils.LogInfo(fmt.Sprintf("%s - %s - no unused ports.", wkld.Hostname, wkld.Href), true)

		}
	}

	// Print a blank line for closing out progress
	fmt.Println()

	if !headerWritten {
		utils.LogInfo("no unused ports identified", true)
	}

	utils.LogEndCommand("unused-ports")
}
