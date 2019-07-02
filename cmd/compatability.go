package cmd

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// TrafficCmd runs the workload identifier
var compatabilityCmd = &cobra.Command{
	Use:   "compatability",
	Short: "Generate a compatability report for all Idle workloads.",
	Long: `
Generate a compatability report for all Idle workloads.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err := utils.GetPCE("pce.json")
		if err != nil {
			log.Fatalf("Error getting PCE for traffic command - %s", err)
		}

		compatabilityReport(pce)
	},
}

func compatabilityReport(pce illumioapi.PCE) {

	// Create output file
	timestamp := time.Now().Format("20060102_150405")
	defaultFile, err := os.Create("compatability-report-" + timestamp + ".csv")
	if err != nil {
		log.Fatalf("ERROR - Creating file - %s\n", err)
	}
	defer defaultFile.Close()

	// Print headers
	fmt.Fprintf(defaultFile, "hostname,href,status\r\n")

	// Get all workloads
	wklds, _, err := illumioapi.GetAllWorkloads(pce)
	if err != nil {
		log.Fatalf("ERROR - get all workloads - %s", err)
	}

	// Iterate through each workload
	for _, w := range wklds {

		// Skip if it's not in idle
		if w.Agent.Config.Mode != "idle" {
			continue
		}

		// Get the compatability report and append
		cr, _, err := illumioapi.GetCompatabilityReport(pce, w)
		if err != nil {
			log.Fatalf("ERROR - getting compatability report for %s (%s) - %s", w.Hostname, w.Href, err)
		}

		// Write result
		fmt.Fprintf(defaultFile, "%s,%s,%s\r\n", w.Hostname, w.Href, cr.QualifyStatus)
	}

}
