package servicefinder

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Global variables
var ports string
var idleOnly, orOperator, debug, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

func init() {

	ServiceFinderCmd.Flags().BoolVarP(&idleOnly, "idle-only", "i", false, "Only look at idle workloads.")
	ServiceFinderCmd.Flags().StringVarP(&ports, "ports", "p", "", "Comma-separated list of ports.")

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

		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		serviceFinder()
	},
}

func serviceFinder() {

	// Log Start of command
	utils.LogStartCommand("service-finder")

	// Remove spaces in our lists
	portList := strings.ReplaceAll(ports, ", ", ",")

	// Create slices
	portStrSlice := strings.Split(portList, ",")

	portListInt := []int{}
	for _, p := range portStrSlice {
		pInt, err := strconv.Atoi(p)
		if err != nil {
			utils.LogError(err.Error())
		}
		portListInt = append(portListInt, pInt)
	}

	// Create Maps for lookup
	portMap := make(map[int]bool)
	for _, p := range portListInt {
		portMap[p] = true
	}

	// Get all workloads
	allWklds, a, err := pce.GetAllWorkloads()
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Check if we should only get idle workloads
	var wklds []illumioapi.Workload
	if idleOnly {
		for _, w := range allWklds {
			if w.GetMode() == "idle" {
				wklds = append(wklds, w)
			}
		}
	} else {
		wklds = allWklds
	}

	// Log our target list
	utils.LogInfo(fmt.Sprintf("identified %d target workloads to check processes.", len(wklds)), true)

	// Start our data struct
	data := [][]string{[]string{"href", "hostname", "port", "process", "role", "app", "env", "loc", "ip"}}

	// For each workload in our target list, make a single workload API call to get services
	for i, w := range wklds {
		fmt.Printf("\r[INFO] - Checking %d of %d workloads", i+1, len(wklds))
		wkld, a, err := pce.GetWkldByHref(w.Href)
		utils.LogAPIResp("GetAllWorkloads", a)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Iterate through each open port
		for _, o := range wkld.Services.OpenServicePorts {
			if _, ok := portMap[o.Port]; ok {
				data = append(data, []string{w.Href, w.Hostname, strconv.Itoa(o.Port), o.ProcessName, w.GetRole(pce.LabelMapH).Value, w.GetApp(pce.LabelMapH).Value, w.GetEnv(pce.LabelMapH).Value, w.GetLoc(pce.LabelMapH).Value, w.GetIPWithDefaultGW()})
			}
		}

	}
	// Print a blank line for closing out progress
	fmt.Println()

	if len(data) > 1 {
		utils.WriteOutput(data, data, fmt.Sprintf("workloader-service-finder-%s.csv", time.Now().Format("20060102_150405")))
		utils.LogInfo(fmt.Sprintf("%d workloads identified", len(data)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no workloads identified that match port requirements.", true)
	}

	utils.LogEndCommand("service-finder")
}
