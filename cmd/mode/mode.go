package mode

import (
	"fmt"
	"log"

	"github.com/brian1917/illumioapi"
)

func modeUpdate() {

	// Log the logonly mode
	log.Printf("INFO - Log only mode set to %t", logOnly)

	// Get all managed workloads
	wklds, _, err := illumioapi.GetAllWorkloads(pce)
	if err != nil {
		log.Fatalf("ERROR - getting all workloads - %s", err)
	}

	// Build a map of all managed workloads
	managedWklds := make(map[string]illumioapi.Workload)
	for _, w := range wklds {
		if w.Agent != nil {
			managedWklds[w.Hostname] = w
		}
	}

	// Get targets
	targets := parseCsv(hostFile)

	// Create a slice to hold all the workloads we need to update
	workloadUpdates := []illumioapi.Workload{}

	// Cycle through each entry in the CSV
	for _, t := range targets {
		// Get the current mode
		var mode string
		if managedWklds[t.workloadHostname].Agent.Config.Mode == "illuminated" && !managedWklds[t.workloadHostname].Agent.Config.LogTraffic {
			mode = "build"
		} else if managedWklds[t.workloadHostname].Agent.Config.Mode == "illuminated" && managedWklds[t.workloadHostname].Agent.Config.LogTraffic {
			mode = "test"
		} else {
			mode = managedWklds[t.workloadHostname].Agent.Config.Mode
		}
		fmt.Println(mode)
		// Check if the current mode is NOT the target mode
		if mode != t.targetMode {
			// Log the change is needed
			log.Printf("INFO - Required Change - %s - Current state: %s - Desired state: %s\r\n", managedWklds[t.workloadHostname].Hostname, mode, t.targetMode)

			// Copy workload with the right target mode and append to slice
			w := managedWklds[t.workloadHostname]
			if t.targetMode == "build" {
				w.Agent.Config.Mode = "illuminated"
				w.Agent.Config.LogTraffic = false
				fmt.Println("set to false")
			} else if t.targetMode == "test" {
				w.Agent.Config.Mode = "illuminated"
				w.Agent.Config.LogTraffic = true
			} else {
				w.Agent.Config.Mode = t.targetMode
			}
			workloadUpdates = append(workloadUpdates, w)
		}
	}

	// Bulk update the workloads if we have some
	if len(workloadUpdates) > 0 && !logOnly {
		api, err := illumioapi.BulkWorkload(pce, workloadUpdates, "update")
		if err != nil {
			log.Fatalf("ERROR - running bulk update - %s", err)
		}
		log.Println("INFO - API Responses:")
		for _, a := range api {
			log.Printf(a.RespBody)
		}
	}
}
