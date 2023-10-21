package servicefinder

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
var ports, processes, hrefFile, outputFileName string
var idleOnly bool
var pce illumioapi.PCE
var err error

func init() {

	ServiceFinderCmd.Flags().StringVarP(&hrefFile, "href", "f", "", "Location of file with HREFs to be used instead of starting with all workloads.")
	ServiceFinderCmd.Flags().BoolVarP(&idleOnly, "idle-only", "i", false, "Only look at idle workloads.")
	ServiceFinderCmd.Flags().StringVarP(&ports, "ports", "p", "", "Comma-separated list of ports.")
	ServiceFinderCmd.Flags().StringVarP(&processes, "process-key-words", "k", "", "Comma-separated list of processes. Matching is partial (e.g., a \"python\" will find \"/usr/bin/python2.7\").")
	ServiceFinderCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	ServiceFinderCmd.Flags().SortFlags = false

}

// ServiceFinderCmd runs the upload command
var ServiceFinderCmd = &cobra.Command{
	Use:   "service-finder",
	Short: "Find workloads with an open port or ports.",
	Long: `
Find workloads with an open port or ports.

Examples:

Find any workload listening on Port 80: workloader service-finder -p 80
Find any workload listening on Port 80 or 443: workloader service-finder -p 80,443
Find any IDLE workload listening on Port 80: workloader service-finder -i -p 80

The update-pce and --no-prompt flags are ignored for this command.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		serviceFinder()
	},
}

func serviceFinder() {

	// Remove spaces in our lists
	portList := strings.ReplaceAll(ports, ", ", ",")
	processList := strings.ReplaceAll(processes, ", ", ",")

	// Create slices
	var portStrSlice, processSlice []string
	if portList != "" {
		portStrSlice = strings.Split(portList, ",")
	}
	if processList != "" {
		processSlice = strings.Split(processList, ",")
	}

	portListInt := []int{}
	if len(portStrSlice) > 0 {
		for _, p := range portStrSlice {
			pInt, err := strconv.Atoi(p)
			if err != nil {
				utils.LogError(err.Error())
			}
			portListInt = append(portListInt, pInt)
		}
	}

	// Create Maps for lookup
	portMap := make(map[int]bool)
	for _, p := range portListInt {
		portMap[p] = true
	}

	// Start the workload slice
	wklds := []illumioapi.Workload{}
	var a illumioapi.APIResponse

	// Process the href file
	if hrefFile != "" {
		csvData, err := utils.ParseCSV(hrefFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		for _, row := range csvData {
			if strings.Contains(row[0], "/orgs/") {
				wklds = append(wklds, illumioapi.Workload{Href: row[0]})
			}
		}
	} else {
		// Get all workloads
		qp := map[string]string{"mode": "idle"}
		if !idleOnly {
			qp = nil
		}

		wklds, a, err = pce.GetWklds(qp)
		utils.LogAPIResp("GetWklds", a)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Log our target list
	utils.LogInfo(fmt.Sprintf("identified %d target workloads to check processes.", len(wklds)), true)

	// Start our data struct
	data := [][]string{{"href", "hostname", "port", "process", "role", "app", "env", "loc", "ip"}}

	// For each workload in our target list, make a single workload API call to get services
	warningMsgs := []string{}
	for i, w := range wklds {
		fmt.Printf("\r%s [INFO] - checking %d of %d workloads", time.Now().Format("2006-01-02 15:04:05 "), i+1, len(wklds))
		w, a, err = pce.GetWkldByHref(w.Href)
		utils.LogAPIResp("GetWkldByHref", a)
		if err != nil && a.StatusCode == 0 {
			utils.LogError(err.Error())
		} else if err != nil {
			warningMsgs = append(warningMsgs, fmt.Sprintf("get %s failed - status code %d", w.Href, a.StatusCode))
			continue
		}

		// Iterate through each open port
		if len(portMap) > 0 {
			for _, o := range w.Services.OpenServicePorts {
				if _, ok := portMap[o.Port]; ok {
					data = append(data, []string{w.Href, w.Hostname, strconv.Itoa(o.Port), o.ProcessName, w.GetRole(pce.Labels).Value, w.GetApp(pce.Labels).Value, w.GetEnv(pce.Labels).Value, w.GetLoc(pce.Labels).Value, w.GetIPWithDefaultGW()})
				}
			}
		}

		// Iterate through each running process
		if len(processSlice) > 0 {
			if w.Services != nil {
				for _, wkldProcess := range w.Services.OpenServicePorts {
					for _, providedProcess := range processSlice {
						if strings.Contains(wkldProcess.ProcessName, providedProcess) {
							data = append(data, []string{w.Href, w.Hostname, strconv.Itoa(wkldProcess.Port), wkldProcess.ProcessName, w.GetRole(pce.Labels).Value, w.GetApp(pce.Labels).Value, w.GetEnv(pce.Labels).Value, w.GetLoc(pce.Labels).Value, w.GetIPWithDefaultGW()})
						}
					}
				}
			}
		}

	}
	// Print a blank line for closing out progress
	fmt.Println()

	// Print warnings
	for _, msg := range warningMsgs {
		utils.LogWarning(msg, true)
	}

	if len(data) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-service-finder-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(data, data, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d workloads identified", len(data)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no workloads identified that match port requirements.", true)
	}

}
