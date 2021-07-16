package traffic

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/workloader/utils"
)

type coreService struct {
	name               string
	provider           bool
	requiredPorts      []int
	optionalPorts      []int
	optionalPortRanges [][]int
	numOptionalPorts   int
	numFlows           int
	processes          []string
	numProcessesReq    int
	app                string
	env                string
	loc                string
	role               string
	ignoreSameSubnet   bool
}

func parseCoreServices(filename string) []coreService {

	// Set CSV columns
	csvName := 0
	csvProvider := 1
	csvReqPorts := 2
	csvOptPorts := 3
	csvNumOptPorts := 4
	csvNumFlows := 5
	csvIgnoreSameSubnet := 6
	csvProcesses := 7
	csvNumProcess := 8
	csvRole := 9
	csvApp := 10
	csvEnv := 11
	csvLoc := 12

	// Create slice to hold the parsed results
	var coreServices []coreService

	// Open CSV File
	csvFile, _ := os.Open(filename)
	reader := csv.NewReader(bufio.NewReader(csvFile))

	// Start the counters
	i := 0

	for {
		// Reset variables
		reqPortsInt := []int{}
		optPortsInt := []int{}
		optPortRangesInt := [][]int{}
		numOptPorts := 0
		numProcessesReq := 0
		numFlows := 0

		// Increment the counter
		i++

		// Read the CSV
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			utils.LogError(fmt.Sprintf("Reading CSV File - %s", err))
		}

		// Skip the header row
		if i == 1 {
			continue
		}

		// Set provider
		provider := true
		if line[csvProvider] == "0" {
			provider = false
		}
		// Set the required ports slice if there is any text in the field
		if len(line[csvReqPorts]) > 0 {
			requiredPortsStr := strings.Split(line[csvReqPorts], " ")
			for _, strPort := range requiredPortsStr {
				intPort, err := strconv.Atoi(strPort)
				if err != nil {
					utils.LogError(fmt.Sprintf("converting required port to int on line %d - %s", i, err))
				}
				reqPortsInt = append(reqPortsInt, intPort)
			}
		}

		// Set the optional ports slice if there is any text in the field
		if len(line[csvOptPorts]) > 0 {

			// Split based on spaces
			optPortsStr := strings.Split(line[csvOptPorts], " ")

			for _, strPort := range optPortsStr {
				rangePortInt := []int{}

				// Process the entry if it a range
				rangePortStr := strings.Split(strPort, "-")
				if len(rangePortStr) > 1 {
					for _, rangeValue := range rangePortStr {
						value, err := strconv.Atoi(rangeValue)
						if err != nil {
							utils.LogError(fmt.Sprintf("converting port range values to int on line %d - %s", i, err))
						}
						rangePortInt = append(rangePortInt, value)
					}
					optPortRangesInt = append(optPortRangesInt, rangePortInt)
				}

				// Process the entry if it is a single port
				if len(rangePortInt) == 0 {
					intPort, err := strconv.Atoi(strPort)
					if err != nil {
						utils.LogError(fmt.Sprintf("converting optional port to int on line %d - %s", i, err))
					}
					optPortsInt = append(optPortsInt, intPort)
				}
			}

			// Convert the number of optional ports to int if there is any text in the field
			if len(line[csvNumOptPorts]) > 0 {
				numOptPorts, err = strconv.Atoi(line[csvNumOptPorts])
				if err != nil {
					utils.LogError(fmt.Sprintf("converting number of required ports to int on line %d - %s", i, err))
				}
			}

			// Convert the number of flows to int
			if len(line[csvNumFlows]) > 0 {
				numFlows, err = strconv.Atoi(line[csvNumFlows])
				if err != nil {
					utils.LogError(fmt.Sprintf("converting number of flows to int on line %d - %s", i, err))
				}
			}

			// Convert the number of processes to int if there is any text in the field
			if len(line[csvNumProcess]) > 0 {
				numProcessesReq, err = strconv.Atoi(line[csvNumProcess])
				if err != nil {
					utils.LogError(fmt.Sprintf("converting number of required consumer services to int on line %d - %s", i, err))
				}
			}

			// Convert bool of ignore same subnet
			ignoreSameSubnetBool, err := strconv.ParseBool(line[csvIgnoreSameSubnet])
			if err != nil {
				utils.LogError(err.Error())
			}

			// Append to the coreServices slice
			coreServices = append(coreServices, coreService{
				name:               line[csvName],
				provider:           provider,
				requiredPorts:      reqPortsInt,
				optionalPorts:      optPortsInt,
				optionalPortRanges: optPortRangesInt,
				numFlows:           numFlows,
				numOptionalPorts:   numOptPorts,
				processes:          strings.Split(line[csvProcesses], " "),
				numProcessesReq:    numProcessesReq,
				app:                line[csvApp],
				env:                line[csvEnv],
				loc:                line[csvLoc],
				role:               line[csvRole],
				ignoreSameSubnet:   ignoreSameSubnetBool})
		}
	}

	return coreServices

}

func csvWriter(results []result, exclWLs bool, outputFileName string) {

	// Start the data array with headers
	data := [][]string{[]string{"ip_address", "hostname", "status", "current_role", "current_app", "current_env", "current_loc", "suggested_role", "suggested_app", "suggested_env", "suggested_loc", "reason"}}

	// Sort the slice
	sort.Slice(results, func(i, j int) bool { return results[i].matchStatus < results[j].matchStatus })

	// Iterate through the results
	for _, r := range results {

		// Set status based on matchStatus code
		var status string
		switch {
		case r.matchStatus == 0:
			status = "Existing workload matched to verify/assign labels."

		case r.matchStatus == 1:
			status = "IP address matched to create/label UMWL"

		case r.matchStatus == 2:
			status = "Existing Workload with no match."
		}

		// Append to data
		data = append(data, []string{r.ipAddress, r.hostname, status, r.eRole, r.eApp, r.eEnv, r.eLoc, r.role, r.app, r.env, r.loc, r.reason})
	}

	// Write the CSV data
	if outputFileName == "" {
		outputFileName = fmt.Sprintf("workloader-traffic-%s.csv", time.Now().Format("20060102_150405"))
	}
	utils.WriteOutput(data, data, outputFileName)
}
