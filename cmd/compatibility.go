package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var verbose bool

func init() {
	compatibilityCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Include full compatibility JSON as 4th column in CSV output. Default is just hostname, href, and green/yellow/red status.")
}

// TrafficCmd runs the workload identifier
var compatibilityCmd = &cobra.Command{
	Use:   "compatibility",
	Short: "Generate a compatibility report for all Idle workloads.",
	Long: `
Generate a compatibility report for all Idle workloads.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err := utils.GetPCE("pce.json")
		if err != nil {
			utils.Logger.Fatalf("[ERROR] - getting PCE - %s", err)
		}

		compatibilityReport(pce)
	},
}

func compatibilityReport(pce illumioapi.PCE) {

	// Log command
	utils.Logger.Println("[INFO] - running compatibility command.")

	// Start the data slice with the headers. We will append data to this.
	var data [][]string
	if verbose {
		data = append(data, []string{"hostname", "href", "status", "raw_data"})
	} else {
		data = append(data, []string{"hostname", "href", "status"})
	}

	// Get all workloads
	wklds, _, err := illumioapi.GetAllWorkloads(pce)
	if err != nil {
		fmt.Println("Error - see workloader.log")
		utils.Logger.Fatalf("[ERROR] - get all workloads - %s", err)
	}

	// Iterate through each workload
	for _, w := range wklds {

		// Skip if it's not in idle
		if w.Agent.Config.Mode != "idle" {
			continue
		}

		// Get the compatibility report and append
		cr, _, err := illumioapi.GetcompatibilityReport(pce, w)
		if err != nil {
			utils.Logger.Fatalf("[ERROR] - getting compatibility report for %s (%s) - %s", w.Hostname, w.Href, err)
		}

		jsonBytes, err := json.Marshal(cr)
		if err != nil {
			fmt.Println("Error - see workloader.log")
			utils.Logger.Fatalf("[ERROR] - marshaling JSON - %s", err)
		}

		// Write result
		if verbose {
			data = append(data, []string{w.Hostname, w.Href, cr.QualifyStatus, string(jsonBytes)})
		} else {
			data = append(data, []string{w.Hostname, w.Href, cr.QualifyStatus})
		}
	}

	// If the CSV data has more than just the headers, create output file and write it.
	if len(data) > 1 {

		// Create output file
		timestamp := time.Now().Format("20060102_150405")
		outFile, err := os.Create("compatibility-report-" + timestamp + ".csv")

		if err != nil {
			utils.Logger.Fatalf("[ERROR] - Creating file - %s\n", err)
		}

		// Write CSV data
		writer := csv.NewWriter(outFile)
		writer.WriteAll(data)
		if err := writer.Error(); err != nil {
			utils.Logger.Fatalf("[ERROR] - writing csv - %s", err)
		}

		// Output status terminal
		fmt.Printf("compatibility report generated for %d idle workloads - see %s.\r\n", len(data)-1, outFile.Name())

		// Close the file
		outFile.Close()
	} else {
		fmt.Println("No workloads with compatibility report.")
		utils.Logger.Println("[INFO] - no workloads with compatibility report.")
	}

}
