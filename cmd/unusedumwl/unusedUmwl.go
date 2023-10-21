package unusedumwl

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
)

func unusedUmwl() {

	// Get the unmanaged workloads
	umwls, a, err := pce.GetWklds(map[string]string{"managed": "false"})
	utils.LogAPIResp("GetAllWorkloadsQP", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create the default query struct and an "or" operator
	tq := illumioapi.TrafficQuery{QueryOperator: "or", PolicyStatuses: []string{}, ExcludeWorkloadsFromIPListQuery: true}

	// Check max results for valid value
	if maxResults < 1 || maxResults > 100000 {
		utils.LogError("max-results must be between 1 and 100000")
	}
	tq.MaxFLows = maxResults

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

	// Get the services
	if exclServiceCSV != "" {
		tq.PortProtoExclude, err = utils.GetServicePortsCSV(exclServiceCSV)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Exclude broadcast and multicast, unless flag set to include non-unicast flows
	if !nonUni {
		tq.TransmissionExcludes = []string{"broadcast", "multicast"}
	}

	// Start the CSV data
	csvData := [][]string{{"hostname", "name", "href", "role", "app", "env", "loc", "interfaces", "traffic_count"}}

	// Iterate over UMWLs
	for _, umwl := range umwls {
		tq.SourcesInclude = [][]string{{umwl.Href}}
		tq.DestinationsInclude = [][]string{{umwl.Href}}
		traffic, a, err := pce.GetTrafficAnalysis(tq)
		utils.LogAPIResp("GetTrafficAnalysis", a)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get interfaces
		interfaces := []string{}
		for _, i := range umwl.Interfaces {
			ipAddress := fmt.Sprintf("%s:%s", i.Name, i.Address)
			if i.CidrBlock != nil && *i.CidrBlock != 0 {
				ipAddress = fmt.Sprintf("%s:%s/%s", i.Name, i.Address, strconv.Itoa(*i.CidrBlock))
			}
			interfaces = append(interfaces, ipAddress)
		}

		// Append to the CSV
		if len(traffic) == 0 || includeAllUmwls {
			csvData = append(csvData, []string{umwl.Hostname, umwl.Name, umwl.Href, umwl.GetRole(pce.Labels).Value, umwl.GetApp(pce.Labels).Value, umwl.GetEnv(pce.Labels).Value, umwl.GetLoc(pce.Labels).Value, strings.Join(interfaces, ";"), strconv.Itoa(len(traffic))})
		}

		// Log iteration
		str := ""
		if len(traffic) == maxResults {
			str = " (query max results)"
		}
		utils.LogInfo(fmt.Sprintf("href: %s - hostname: %s - name: %s - %d traffic records%s", umwl.Href, umwl.Hostname, umwl.Name, len(traffic), str), true)
	}

	// Output the CSV Data
	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-unused-umwl-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d umwls exported", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no records exported matching criteria", true)
	}

}
