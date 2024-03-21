package wkldexport

import (
	"strings"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare global variables
var managedOnly, unmanagedOnly, onlineOnly, noHref, includeVuln, removeDescNewLines, labelSummary bool
var headers, labelFile, uniqueLabelKeys, globalOutputFileName, subnetInclude string

func init() {
	WkldExportCmd.Flags().StringVar(&headers, "headers", "", "comma-separated list of headers for export. default is all headers.")
	WkldExportCmd.Flags().StringVar(&labelFile, "label-file", "", "csv file with labels to filter query. see description below.")
	WkldExportCmd.Flags().BoolVarP(&managedOnly, "managed-only", "m", false, "only export managed workloads.")
	WkldExportCmd.Flags().BoolVarP(&unmanagedOnly, "unmanaged-only", "u", false, "only export unmanaged workloads.")
	WkldExportCmd.Flags().BoolVarP(&onlineOnly, "online-only", "o", false, "only export online workloads.")
	WkldExportCmd.Flags().StringVarP(&subnetInclude, "subnet", "s", "", "subnet filter to only export workloads with an interface in that subnet. multiple subnets should be comma-separated (e.g., \"10.0.0.64/26,10.0.0.128/26\")")
	WkldExportCmd.Flags().BoolVarP(&includeVuln, "incude-vuln-data", "v", false, "include vulnerability data.")
	WkldExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	WkldExportCmd.Flags().BoolVar(&labelSummary, "label-summary", false, "include an export of unique label combinations.")
	WkldExportCmd.Flags().StringVar(&uniqueLabelKeys, "label-summary-keys", "", "comma-separated list of keys to include for determining uniqueness. blank uses all keys.")
	WkldExportCmd.Flags().StringVar(&globalOutputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	WkldExportCmd.Flags().BoolVar(&removeDescNewLines, "remove-desc-newline", false, "will remove new line characters in description field.")

	WkldExportCmd.Flags().SortFlags = false

}

// WkldExportCmd runs the workload identifier
var WkldExportCmd = &cobra.Command{
	Use:   "wkld-export",
	Short: "Create a CSV export of all workloads in the PCE.",
	Long: `
Create a CSV export of all workloads in the PCE.

If only workloads with certain labels, the first row of the label-file should be label keys. The workload query uses an AND operator for entries on the same row and an OR operator for the separate rows. An example label file is below:
+------+-----+-----+-----+----+
| role | app | env | loc | bu |
+------+-----+-----+-----+----+
| web  | erp |     |     |    |
|      |     |     | bos | it |
|      | crm |     |     |    |
+------+-----+-----+-----+----+
This example queries all workloads that are
- web (role) AND erp (app) 
- OR bos(loc) AND it (bu)
- OR CRM (app)

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		var err error
		wkldExport := WkldExport{PCE: &illumioapi.PCE{}, IncludeVuln: includeVuln, RemoveDescNewLines: removeDescNewLines, IncludeLabelSummary: labelSummary, LabelSummaryKeys: uniqueLabelKeys}
		*wkldExport.PCE, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		if headers != "" {
			wkldExport.Headers = strings.Split(strings.Replace(headers, " ", "", -1), ",")
		}

		preLoadLabels := false
		if labelFile != "" {
			preLoadLabels = true
			api, err := wkldExport.PCE.GetLabels(nil)
			utils.LogAPIRespV2("GetLabels", api)
			if err != nil {
				utils.LogError(err.Error())
			}
		}

		// Load the PCE
		load := illumioapi.LoadInput{Workloads: true, Labels: !preLoadLabels}
		load.WorkloadsQueryParameters = make(map[string]string)

		// Check the label file
		if labelFile != "" {
			labelCsvData, err := utils.ParseCSV(labelFile)
			if err != nil {
				utils.LogErrorf("parsing labelFile - %s", err)
			}

			labelQuery, err := wkldExport.PCE.WorkloadQueryLabelParameter(labelCsvData)
			if err != nil {
				utils.LogErrorf("getting label parameter query - %s", err)
			}
			if len(labelQuery) > 10000 {
				utils.LogErrorf("the query is too large. the total character count is %d and the limit for this command is 10,000", len(labelQuery))
			}
			load.WorkloadsQueryParameters["labels"] = labelQuery
		}

		if unmanagedOnly {
			load.WorkloadsQueryParameters["managed"] = "false"
		}
		if managedOnly {
			load.WorkloadsQueryParameters["managed"] = "true"
		}
		if includeVuln {
			load.WorkloadsQueryParameters["representation"] = "workload_labels_vulnerabilities"
		}
		if onlineOnly {
			load.WorkloadsQueryParameters["online"] = "true"
		}

		apiResps, err := wkldExport.PCE.Load(load, utils.UseMulti())
		utils.LogMultiAPIRespV2(apiResps)
		if err != nil {
			utils.LogError(err.Error())
		}

		wkldExport.WriteToCsv(globalOutputFileName)

	},
}
