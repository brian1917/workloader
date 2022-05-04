package processexport

import (
	"fmt"
	"strconv"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var outputFileName string

func init() {
	ProcessExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
}

// IplExportCmd runs the workload identifier
var ProcessExportCmd = &cobra.Command{
	Use:   "process-export",
	Short: "Create a CSV export of all running processes on all workloads.",
	Long: `
Create a CSV export of all running processes on all workloads.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		ExportProcesses(pce, outputFileName)
	},
}

func ExportProcesses(pce illumioapi.PCE, outputFileName string) {

	// Log command execution
	utils.LogStartCommand("process-export")

	// Get all managed workloads
	managedWklds, a, err := pce.GetAllWorkloadsQP(map[string]string{"managed": "true"})
	utils.LogAPIResp("GetAllWorklaodsQP", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Setup CSV Data
	csvData := [][]string{{"hostname", "process_name", "service_name", "port", "proto"}}

	// Set up slice of workloads
	for _, mw := range managedWklds {
		w, a, err := pce.GetWkldByHref(mw.Href)
		utils.LogAPIResp("GetAllWorklaodsQP", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		if w.Services == nil || w.Services.OpenServicePorts == nil {
			continue
		}
		for _, osp := range w.Services.OpenServicePorts {
			csvData = append(csvData, []string{w.Hostname, osp.ProcessName, osp.WinServiceName, strconv.Itoa(osp.Port), strconv.Itoa(osp.Protocol)})
		}

	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-process-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d processes exported.", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no process in workloads in PCE.", true)
	}
	utils.LogEndCommand("process-export")
}
