package svcexport

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug bool
var outFormat string

// SvcExportCmd runs the workload identifier
var SvcExportCmd = &cobra.Command{
	Use:   "svc-export",
	Short: "Create a CSV export of all services in the PCE.",
	Long: `
Create a CSV export of all services in the PCE.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the viper values
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		exportServices()
	},
}

func exportServices() {

	// Log command execution
	utils.LogStartCommand("svc-export")

	// Start the data slice with headers
	csvData := [][]string{[]string{"name", "description", "service_ports", "window_services", "href"}}

	// GetAllServices
	allSvs, a, err := pce.GetAllServices("draft")
	utils.LogAPIResp("GetAllSvcs", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	for _, s := range allSvs {

		// Parse the services
		windowsServices, servicePorts := s.ParseService()

		// Add to the CSV data
		csvData = append(csvData, []string{s.Name, s.Description, strings.Join(servicePorts, ";"), strings.Join(windowsServices, ";"), s.Href})
	}

	// Output the CSV Data
	if len(csvData) > 1 {
		utils.WriteOutput(csvData, csvData, fmt.Sprintf("workloader-svc-export-%s.csv", time.Now().Format("20060102_150405")))
		utils.LogInfo(fmt.Sprintf("%d services exported", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no services in PCE.", true)
	}

	utils.LogEndCommand("svc-export")
}
