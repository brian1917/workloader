package ruleexport

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var pce illumioapi.PCE
var inputFile, outputFileName string

func init() {
	RuleUsageCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
}

var RuleUsageCmd = &cobra.Command{
	Use:   "rule-usage [csv output of rule-export with -traffic flag]",
	Short: "Get traffic hit count for rules.",
	Long: `
Get traffic hit count for rules.

Run workloader rule-export with the --traffic-count flags and any necessary traffic filter flags.
The output will have all the rules with an async query href.
Within 24 hours, pass the output file of rule-export into this rule-usage command to get the results of the traffic queries.
Run as many times as needed until all traffic queries have been processed. 

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(false)
		if err != nil {
			utils.LogError(err.Error())
		}
		// Get the input file
		if len(args) != 1 {
			fmt.Println("command requires 1 argument for the csv file. see usage help.")
			os.Exit(0)
		}
		inputFile = args[0]

		retrieveTraffic()
	},
}

func retrieveTraffic() {
	// parse the input csv
	csvData, err := utils.ParseCSV(inputFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Find the async_query_href and the status header
	var asyncHrefCol, asyncQueryStatusCol, flowsCol, flowsByPortCol int
	for i, col := range csvData[0] {
		if col == "async_query_href" {
			asyncHrefCol = i
		}
		if col == "async_query_status" {
			asyncQueryStatusCol = i
		}
		if col == "flows" {
			flowsCol = i
		}
		if col == "flows_by_port" {
			flowsByPortCol = i
		}
	}

	// Get all pending explorer queries
	utils.LogInfo("getting all async queries", true)
	asyncQueries, api, err := pce.GetAsyncQueries(nil)
	utils.LogAPIResp("GetAsyncQueries", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	utils.LogInfo(fmt.Sprintf("%d total async queries", len(asyncQueries)), true)

	// Create the asyncQueries map
	asyncHrefMap := make(map[string]illumioapi.AsyncTrafficQuery)
	for _, aq := range asyncQueries {
		asyncHrefMap[aq.Href] = aq
	}

	// Iterate through the csv and check for reesults
	newCsvData := [][]string{}
	var numStillPending, numAlreadyCompleted, numNewlyCompleted, numExpired int
	for i, row := range csvData {
		// Create thew new CSV data
		newCsvData = append(newCsvData, row)
		// Skip the first row
		if i == 0 {
			continue
		}
		if row[asyncQueryStatusCol] == "completed" {
			utils.LogInfo(fmt.Sprintf("csv row %d - %s already completed from previous run", i+1, row[asyncHrefCol]), true)
			numAlreadyCompleted++
			continue
		}
		// Get the async query
		var aq illumioapi.AsyncTrafficQuery
		var exists bool
		if aq, exists = asyncHrefMap[row[asyncHrefCol]]; !exists {
			utils.LogWarning(fmt.Sprintf("csv row %d - %s does not exist as an async query. invalid href or it expired.", i+1, row[asyncHrefCol]), true)
			numExpired++
			continue
		}
		if aq.Status != "completed" {
			utils.LogInfo(fmt.Sprintf("csv row %d - %s is not completed.", i+1, aq.Href), true)
			numStillPending++
			continue
		}

		traffic, api, err := pce.GetAsyncQueryResults(aq)
		utils.LogAPIResp("GetResults", api)
		if err != nil {
			utils.LogError(err.Error())
		}
		// Edit the csv
		newCsvData[len(newCsvData)-1][flowsCol], newCsvData[len(newCsvData)-1][flowsByPortCol] = processFlows(traffic)
		newCsvData[len(newCsvData)-1][asyncQueryStatusCol] = "completed"
		utils.LogInfo(fmt.Sprintf("csv row %d - %s completed and downloaded", i+1, aq.Href), true)
		numNewlyCompleted++

	}

	// Write the output
	if outputFileName == "" {
		outputFileName = fmt.Sprintf("workloader-ruleset-export-rule-usage-%s.csv", time.Now().Format("20060102_150405"))
	}

	// Summarize
	utils.LogInfo(fmt.Sprintf("%d rule traffic queries in input.", len(csvData)-1), true)
	utils.LogInfo(fmt.Sprintf("%d rule traffic queries completed prior to this run.", numAlreadyCompleted), true)
	utils.LogInfo(fmt.Sprintf("%d rule traffic queries completed on this run.", numNewlyCompleted), true)
	utils.LogInfo(fmt.Sprintf("%d rule traffic queries expired (see warnings).", numExpired), true)
	utils.LogInfo(fmt.Sprintf("%d rule traffic queries still pending.", numStillPending), true)
	utils.WriteOutput(newCsvData, [][]string{}, outputFileName)
}

func processFlows(traffic []illumioapi.TrafficAnalysis) (flowCount, flowCountByPort string) {

	// Get flow count
	flows := 0
	ports := make(map[string]int)
	protocols := illumioapi.ProtocolList()
	type entry struct {
		flows int
		port  string
		proto string
	}
	maxEntries := 20
	numBeyondMax := 0
	entries := []entry{}
	for _, t := range traffic {
		flows = flows + t.NumConnections
		ports[fmt.Sprintf("%d-%d", t.ExpSrv.Port, t.ExpSrv.Proto)] = ports[fmt.Sprintf("%d-%d", t.ExpSrv.Port, t.ExpSrv.Proto)] + t.NumConnections
	}
	for a, p := range ports {
		portProtoString := strings.Split(a, "-")
		protoInt, err := strconv.Atoi(portProtoString[1])
		if err != nil {
			utils.LogError(err.Error())
		}
		if len(entries) < maxEntries {
			entries = append(entries, entry{port: portProtoString[0], proto: protocols[protoInt], flows: p})
		} else {
			numBeyondMax++
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].flows < entries[j].flows
	})
	entriesString := []string{}
	for _, e := range entries {
		entriesString = append(entriesString, fmt.Sprintf("%s %s (%d)", e.port, e.proto, e.flows))
	}
	if numBeyondMax > 0 {
		entriesString = append(entriesString, fmt.Sprintf("+ %d more", numBeyondMax))
	}

	return strconv.Itoa(flows), strings.Join(entriesString, "; ")
}
