package unusedports

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Global variables
var inputFile, role, app, env, loc, outputFileName string
var pce illumioapi.PCE
var err error

func init() {
	UnusedPortsCmd.Flags().StringVarP(&inputFile, "input-file", "i", "", "optional input file with list of hrefs for target workloads. recommended to use wkld-export to get href file.")
	UnusedPortsCmd.Flags().StringVarP(&role, "role", "r", "", "optional role label for target workloads. if an input file is provided the labels are ignored. multiple label flags are an AND operator.")
	UnusedPortsCmd.Flags().StringVarP(&app, "app", "a", "", "optional app label for target workloads. if an input file is provided the labels are ignored. multiple label flags are an AND operator.")
	UnusedPortsCmd.Flags().StringVarP(&env, "env", "e", "", "optional env label for target workloads. if an input file is provided the labels are ignored. multiple label flags are an AND operator.")
	UnusedPortsCmd.Flags().StringVarP(&loc, "loc", "l", "", "optional role label for target workloads. if an input file is provided the labels are ignored. multiple label flags are an AND operator.")
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
		wklds, a, err = pce.GetAllWorkloadsQP(qp)
		utils.LogAPIResp("GetAllWorkloadsQP", a)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Log our target list
	utils.LogInfo(fmt.Sprintf("identified %d workloads to check open port usage.", len(wklds)), true)

	// Start the output data
	data := [][]string{{"hostname", "href", "role", "app", "env", "loc", "unused_ports"}}

	// Process each workload
	for i, w := range wklds {
		fmt.Printf("\r%s [INFO] - querying open ports and trafic for %d of %d workloads", time.Now().Format("2006-01-02 15:04:05 "), i+1, len(wklds))
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

		for _, servicePort := range wkld.Services.OpenServicePorts {
			// Create the explorer query
			tq := illumioapi.TrafficQuery{
				DestinationsInclude:  [][]string{{wkld.Href}},
				MaxFLows:             100000,
				PortProtoInclude:     [][2]int{{servicePort.Port, servicePort.Protocol}},
				TransmissionExcludes: []string{"broadcast", "multicast"},
				StartTime:            time.Now().AddDate(0, 0, -89).In(time.UTC),
				EndTime:              time.Now().AddDate(0, 0, 1).In(time.UTC),
				PolicyStatuses:       []string{"allowed", "blocked", "potentially_blocked", "unknown"},
			}

			// Run the query
			traffic, a, err := pce.GetTrafficAnalysis(tq)
			utils.LogAPIResp("GetTrafficAnalysis", a)
			if err != nil {
				utils.LogError(err.Error())
			}

			// If the query returns 0 results, append to the unused ports slice
			if len(traffic) == 0 {
				unusedPorts = append(unusedPorts, fmt.Sprintf("%d %s", servicePort.Port, illumioapi.ProtocolList()[servicePort.Protocol]))
			}
		}

		// Append results to output data slice
		data = append(data, []string{wkld.Hostname, wkld.Href, wkld.GetRole(pce.Labels).Value, wkld.GetApp(pce.Labels).Value, wkld.GetEnv(pce.Labels).Value, wkld.GetLoc(pce.Labels).Value, strings.Join(unusedPorts, "; ")})
	}

	// Print a blank line for closing out progress
	fmt.Println()

	if len(data) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-unused-ports-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(data, data, outputFileName)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no unused ports identified", true)
	}

	utils.LogEndCommand("unused-ports")
}
