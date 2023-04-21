package processexport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	ia "github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce ia.PCE
var err error
var hrefFile, labelFile, enforcementMode, outputFileName string
var single bool

func init() {
	ProcessExportCmd.Flags().StringVar(&labelFile, "label-file", "", "csv file with labels to filter query. the file should have 4 headers: role, app, env, and loc. The four columns in each row is an \"AND\" operation. Each row is an \"OR\" operation.")
	ProcessExportCmd.Flags().StringVar(&hrefFile, "href-file", "", "csv file with hrefs.")
	ProcessExportCmd.Flags().BoolVar(&single, "single", false, "only used with --host-file. gets hosts by individual api calls vs. getting all workloads and filtering after.")
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

If no label-file or href-file are used all workloads are processed.

The first row of a label-file should be label keys. The workload query uses an AND operator for entries on the same row and an OR operator for the separate rows. An example label file is below:
+------+-----+-----+-----+----+
| role | app | env | loc | bu |
+------+-----+-----+-----+----+
| web  | erp |     |     |    |
|      |     |     | bos | it |
|      | crm |     |     |    |
+------+-----+-----+-----+----+
This example queries all idle workloads that are
- web (role) AND erp (app) 
- OR bos(loc) AND it (bu)
- OR CRM (app)

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Log command execution
		utils.LogStartCommand("process-export")
		ExportProcesses(pce, outputFileName)
		utils.LogEndCommand("process-export")
	},
}

func ExportProcesses(pce ia.PCE, outputFileName string) {

	// Validate enforcement mdoe
	enforcementMode = strings.ToLower(enforcementMode)
	if enforcementMode != "" && enforcementMode != "idle" && enforcementMode != "visibility_only" && enforcementMode != "full" && enforcementMode != "selective" {
		utils.LogError("invalid enforcement mode. must be blank, idle, visibility_only, selective, or full.")
	}

	// Get labels and label dimensions
	apiResps, err := pce.Load(ia.LoadInput{LabelDimensions: true, Labels: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogErrorf("loading pce - %s", err)
	}

	// If there is no href file, get workloads with query parameters
	if hrefFile == "" {
		qp := map[string]string{"managed": "true"}
		// Process a label file if one is provided
		if labelFile != "" {
			labelCsvData, err := utils.ParseCSV(labelFile)
			if err != nil {
				utils.LogErrorf("parsing labelFile - %s", err)
			}

			labelQuery, err := pce.WorkloadQueryLabelParameter(labelCsvData)
			if err != nil {
				utils.LogErrorf("getting label parameter query - %s", err)
			}
			if len(labelQuery) > 10000 {
				utils.LogErrorf("the query is too large. the total character count is %d and the limit for this command is 10,000", len(labelQuery))
			}
			qp["labels"] = labelQuery
		}
		if enforcementMode != "" {
			qp["enforcement_mode"] = enforcementMode
		}
		// Get the workloads
		api, err := pce.GetWklds(qp)
		utils.LogAPIRespV2("GetWklds", api)
		if err != nil {
			utils.LogErrorf("GetWklds - %s", err)
		}
	} else {
		// A hostn file is provided - parse it
		hrefFileData, err := utils.ParseCSV(hrefFile)
		if err != nil {
			utils.LogErrorf("parsing hrefFile - %d", err)
		}
		// Build the href list
		hrefList := []string{}
		for _, row := range hrefFileData {
			hrefList = append(hrefList, row[0])
		}
		// Get the workloads
		apiResps, err := pce.GetWkldsByHrefList(hrefList, single)
		for _, a := range apiResps {
			utils.LogAPIRespV2("GetWkldsByHrefList", a)
		}
		if err != nil {
			utils.LogErrorf("GetWkldsByHrefList - %s", err)
		}
	}

	// Process file name
	if outputFileName == "" {
		outputFileName = fmt.Sprintf("workloader-process-export-%s.csv", time.Now().Format("20060102_150405"))
	}

	// Setup CSV Data
	headers := []string{"hostname", "href", "process_path", "service_name", "port", "proto"}
	for _, ld := range pce.LabelDimensionsSlice {
		headers = append(headers, ld.Key)
	}
	utils.WriteLineOutput(headers, outputFileName)

	// Set up slice of workloads
	for i, wkld := range pce.WorkloadsSlice {
		utils.LogInfof(true, "processing %s - %d of %d", ia.PtrToVal(wkld.Hostname), i+1, len(pce.WorkloadsSlice))
		w, a, err := pce.GetWkldByHref(wkld.Href)
		utils.LogAPIRespV2("GetWkldByHref", a)
		if err != nil {
			utils.LogWarningf(true, "error getting %s - skipping", wkld.Href)
		}
		if w.Services == nil || w.Services.OpenServicePorts == nil {
			continue
		}
		for _, osp := range ia.PtrToVal(w.Services.OpenServicePorts) {
			csvRow := []string{ia.PtrToVal(w.Hostname), w.Href, osp.ProcessName, osp.WinServiceName, strconv.Itoa(osp.Port), ia.ProtocolList()[osp.Protocol]}
			for _, ld := range pce.LabelDimensionsSlice {
				csvRow = append(csvRow, wkld.GetLabelByKey(ld.Key, pce.Labels).Value)
			}
			utils.WriteLineOutput(csvRow, outputFileName)
		}
	}

	utils.LogInfof(true, "output file: %s", outputFileName)
}
