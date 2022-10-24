package processexport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var hrefFile, enforcementMode, outputFileName string

func init() {
	ProcessExportCmd.Flags().StringVarP(&hrefFile, "href", "f", "", "optionally specify the location of a file with hrefs to be used instead of starting with all workloads. header optional")
	ProcessExportCmd.Flags().StringVar(&enforcementMode, "enforcement-mode", "", "optionally specify an enforcement mode filter. acceptable values are idle, visibility_only, selective, and full. ignored if href file is provided")
	ProcessExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	ProcessExportCmd.Flags().SortFlags = false
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

	// Validate enforcement mdoe
	enforcementMode = strings.ToLower(enforcementMode)
	if enforcementMode != "" && enforcementMode != "idle" && enforcementMode != "visibility_only" && enforcementMode != "full" && enforcementMode != "selective" {
		utils.LogError("invalid enforcement mode. must be blank, idle, visibility_only, selective, or full.")
	}

	// Setup some variables
	var wkldHrefs []string

	// If the hrefFile is provided, parse thar
	if hrefFile != "" {
		hrefCsvData, err := utils.ParseCSV(hrefFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		for _, row := range hrefCsvData {
			if strings.Contains(row[0], "/orgs/") {
				wkldHrefs = append(wkldHrefs, row[0])
			}
		}
	}

	if hrefFile == "" {
		queryParameters := map[string]string{"managed": "true"}
		if enforcementMode != "" {
			queryParameters["enforcement_mode"] = enforcementMode
		}
		wklds, api, err := pce.GetWklds(queryParameters)
		utils.LogAPIResp("GetWklds", api)
		if err != nil {
			utils.LogError(err.Error())
		}
		for _, w := range wklds {
			wkldHrefs = append(wkldHrefs, w.Href)
		}

	}

	// Setup CSV Data
	csvData := [][]string{{"hostname", "href", "process_name", "service_name", "port", "proto"}}

	// Set up slice of workloads
	for _, wHref := range wkldHrefs {
		w, a, err := pce.GetWkldByHref(wHref)
		utils.LogAPIResp("GetWkldByHref", a)
		if err != nil {
			utils.LogWarning(fmt.Sprintf("error getting %s - skipping", wHref), true)
		}
		if w.Services == nil || w.Services.OpenServicePorts == nil {
			continue
		}
		for _, osp := range w.Services.OpenServicePorts {
			csvData = append(csvData, []string{w.Hostname, w.Href, osp.ProcessName, osp.WinServiceName, strconv.Itoa(osp.Port), strconv.Itoa(osp.Protocol)})
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
