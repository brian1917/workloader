package portusage

import (
	"fmt"
	"strconv"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
)

func getResults(file string) {

	// parse the input csv
	csvData, err := utils.ParseCSV(file)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Find the async_query_href and the status header
	var asyncHrefCol, asyncQueryStatusCol, flowsCol int
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
	}

	// Get all pending explorer queries
	utils.LogInfo("getting all async queries", true)
	asyncQueries, api, err := pce.GetAsyncQueries(nil)
	utils.LogAPIRespV2("GetAsyncQueries", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	utils.LogInfo(fmt.Sprintf("%d total async queries for this user", len(asyncQueries)), true)

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
		utils.LogAPIRespV2("GetResults", api)
		if err != nil {
			utils.LogError(err.Error())
		}
		// Edit the csv
		newCsvData[len(newCsvData)-1][flowsCol] = strconv.Itoa(len(traffic))
		newCsvData[len(newCsvData)-1][asyncQueryStatusCol] = "completed"
		utils.LogInfo(fmt.Sprintf("csv row %d - %s completed and downloaded", i+1, aq.Href), true)
		numNewlyCompleted++

	}

	// Summarize
	utils.LogInfo(fmt.Sprintf("%d traffic queries in input.", len(csvData)-1), true)
	utils.LogInfo(fmt.Sprintf("%d traffic queries completed prior to this run.", numAlreadyCompleted), true)
	utils.LogInfo(fmt.Sprintf("%d traffic queries completed on this run.", numNewlyCompleted), true)
	utils.LogInfo(fmt.Sprintf("%d traffic queries expired (see warnings).", numExpired), true)
	utils.LogInfo(fmt.Sprintf("%d traffic queries still pending.", numStillPending), true)
	utils.WriteOutput(newCsvData, [][]string{}, file)
}
