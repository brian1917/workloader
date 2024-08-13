package unpair

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var hrefFile, labelFile, neverUnpairFile, hrefHeader, restore string
var updatePCE, noPrompt, includeOnline, singleAPI, singleUnpair bool
var hoursSinceLastHB int
var pce illumioapi.PCE
var err error

// Init handles flags
func init() {
	UnpairCmd.Flags().StringVar(&restore, "restore", "saved", "restore value. Must be saved, default, or disable.")
	UnpairCmd.Flags().StringVar(&hrefFile, "href-file", "", "csv file with target ven hrefs to unpair. the input file should have a header row. use the --header flag to specify the header with the ven href.")
	UnpairCmd.Flags().StringVar(&hrefHeader, "header", "ven_href", "column header for ven hrefs in the href-file.")
	UnpairCmd.Flags().StringVar(&labelFile, "label-file", "", "csv file with labels to filter query. see description below.")
	UnpairCmd.Flags().StringVar(&neverUnpairFile, "never-unpair-file", "", "csv file with hrefs that should never be unpaired (e.g., hrefs of VDI golden images). headers are optional.")
	UnpairCmd.Flags().IntVar(&hoursSinceLastHB, "hours", 0, "limit unpairing to workloads that have not sent a heartbeat in set time. 0 will ignore heartbeats.")
	UnpairCmd.Flags().BoolVar(&includeOnline, "include-online", false, "include workloads that are online. by default only offline workloads that meet criteria will be unpaired.")
	UnpairCmd.Flags().BoolVar(&singleAPI, "single-api", false, "if we need to get vens and workloads from the pce query the vens on at a time vs. getting all vens. useful for very large environments that are only unpairing a small amount.")
	UnpairCmd.Flags().BoolVar(&singleUnpair, "single-unpair", false, "one API call per unpair versus one API call per 1000 workloads. this will be significantly slower but provide more details in the pce's syslog messages.")

	UnpairCmd.Flags().SortFlags = false
}

// UnpairCmd runs the unpair
var UnpairCmd = &cobra.Command{
	Use:   "unpair",
	Short: "Unpair VENs through an input file of labels or ven hrefs.",

	Long: `  
Unpair VENs through an input file of labels or ven hrefs.

To target only workloads with a specific label, use a label-file with the format below. Workloader will generate a CSV to show the workloads that would be unpaired.
First row of the label-file should be label keys. The workload query uses an AND operator for entries on the same row and an OR operator for the separate rows. An example label file is below:
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

Use the --update-pce command to run the unpair with a user prompt confirmation. Use --update-pce and --no-prompt to run unpair with no prompts.`,

	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogErrorf("getting target pce - %s", err)
		}

		// Get persistent flags from Viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		unpair()
	},
}

func unpair() {

	// Check the restore value
	restore = strings.ToLower(restore)
	if restore != "saved" && restore != "default" && restore != "disable" {
		utils.LogError("restore value must be saved, default, or disable.")
	}

	// Check the input files
	if hrefFile == "" && labelFile == "" {
		utils.LogError("must provide either a label file or href file.")
	}
	if hrefFile != "" && labelFile != "" {
		utils.LogError("cannot provide both a label file and href file.")
	}
	if hrefFile == "" && singleAPI {
		utils.LogError("single-api only valid when using an href file.")
	}

	// Load the pce if we are using a label file or need to check hearbeat or online status
	if !singleAPI {
		wkldParams := make(map[string]string)
		wkldParams["managed"] = "true"

		if labelFile != "" {
			api, err := pce.GetLabels(nil)
			utils.LogAPIRespV2("GetLabels", api)
			if err != nil {
				utils.LogErrorf("getting labels - %s", err)
			}

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
			wkldParams["labels"] = labelQuery
		}
		// Get workloads, VENs, and label dimensions
		pce.Load(illumioapi.LoadInput{Workloads: true, VENs: true, LabelDimensions: true, WorkloadsQueryParameters: wkldParams}, utils.UseMulti())
	} else {
		// Get the VENs individually
		pce.VENs = make(map[string]illumioapi.VEN)
		for _, v := range processVENsFile() {
			ven, api, err := pce.GetVenByHref(v.Href)
			utils.LogAPIRespV2("GetVenByHref", api)
			if err != nil {
				utils.LogErrorf("getting ven - %s", err)
			}
			pce.VENsSlice = append(pce.VENsSlice, ven)
			pce.VENs[ven.Href] = ven
			pce.VENs[illumioapi.PtrToVal(ven.Hostname)] = ven
		}
		// Get the workloads individually
		pce.Workloads = make(map[string]illumioapi.Workload)
		for _, v := range pce.VENsSlice {
			for _, w := range illumioapi.PtrToVal(v.Workloads) {
				wkld, api, err := pce.GetWkldByHref(w.Href)
				utils.LogAPIRespV2("GetWkldByHref", api)
				if err != nil {
					utils.LogErrorf("getting workload - %s", err)
				}
				pce.Workloads[wkld.Href] = wkld
				pce.Workloads[illumioapi.PtrToVal(wkld.Hostname)] = wkld
			}
		}
		// Load the label dimensions
		api, err := pce.GetLabelDimensions(nil)
		utils.LogAPIRespV2("GetLabelDimensions", api)
		if err != nil {
			utils.LogErrorf("getting label dimensions - %s", err)
		}
	}

	// Build the href file from the label data export
	if labelFile != "" {
		// Get the workloads
		headers := []string{"ven_href", "hostname"}
		for _, ld := range pce.LabelDimensionsSlice {
			headers = append(headers, ld.Key)
		}
		wkldExport := wkldexport.WkldExport{PCE: &pce, Headers: headers, IncludeLabelSummary: false, IncludeVuln: false, RemoveDescNewLines: false, LabelPrefix: false}
		hrefFile = utils.FileName("")
		wkldExport.WriteToCsv(hrefFile)
	}

	// Run validation
	vensToUnpair := []illumioapi.VEN{}
	for i, v := range processVENsFile() {

		if val, ok := pce.VENs[v.Href]; !ok {
			utils.LogWarningf(true, "csv line %d - %s does not exist in the pce. skipping", i+1, v.Href)
		} else {

			// validate workload assignment
			if (val.Workloads != nil && len(illumioapi.PtrToVal(val.Workloads)) != 1) || val.Workloads == nil {
				utils.LogWarningf(true, "csv line %d - %s has invalid workload assignment. skipping", i+1, val.Href)
			}

			// validate heartbeat
			if hoursSinceLastHB > 0 && val.HoursSinceLastHeartBeat() < float64(hoursSinceLastHB) {
				utils.LogInfof(false, "csv line %d - %s hours since last heartbeat is %d. skipping.", i+1, val.Href, int(val.HoursSinceLastHeartBeat()))
				continue
			}

			// validate online status
			workloads := *val.Workloads
			wkld := pce.Workloads[workloads[0].Href]
			if !includeOnline && *wkld.Online {
				continue
			}

			// Add to the slice
			vensToUnpair = append(vensToUnpair, illumioapi.VEN{Href: val.Href})
		}
	}

	if len(vensToUnpair) == 0 {
		if !includeOnline {
			utils.LogInfo("zero vens identified. The --include-online option was not set so only offline workloads were evaluated.", true)
		} else {
			utils.LogInfo("zero vens identified.", true)
		}
		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d vens requiring unpairing. To do the unpair, run again using --update-pce flag. The --no-prompt flag will bypass the prompt if used with --update-pce.", len(vensToUnpair)), true)
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("%s [PROMPT] - workloader identified %d vens requiring unpairing in %s (%s). Do you want to run the unpair? (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), len(vensToUnpair), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied.", true)
			return
		}
	}

	// Run the single unpair
	if singleUnpair {
		// Create a slice of slices
		singleTargetVENs := [][]illumioapi.VEN{}
		for _, v := range vensToUnpair {
			singleTargetVENs = append(singleTargetVENs, []illumioapi.VEN{v})
		}

		// Iterate through those for unpairing
		for i, v := range singleTargetVENs {
			apiResps, err := pce.VensUnpair(v, restore)
			utils.LogAPIRespV2("unpair workloads", apiResps[0])
			if err != nil {
				utils.LogError(err.Error())
			}
			// Update progress
			utils.LogInfo(fmt.Sprintf("unpaired %d of %d - %s - status code %d", i+1, len(singleTargetVENs), v[0].Href, apiResps[0].StatusCode), true)
		}
	} else {
		// Run the bulk unpair
		apiResps, err := pce.VensUnpair(vensToUnpair, restore)
		for _, a := range apiResps {
			utils.LogAPIRespV2("VensUnpair", a)
		}
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfof(true, "unpaired %d vens", len(vensToUnpair))

	}
}

func processVENsFile() []illumioapi.VEN {
	// Parse the never unpair
	neverUnpairHrefs := make(map[string]bool)
	if neverUnpairFile != "" {
		neverUnpairData, err := utils.ParseCSV(neverUnpairFile)
		if err != nil {
			utils.LogErrorf("parsing never-unpair-file - %s", err)
		}
		for rowNum, row := range neverUnpairData {
			if strings.Contains(row[0], "/workloads/") {
				utils.LogErrorf("csv row %d -  never-unpair-file - %s is a workload href. only VEN hrefs are allowed.", rowNum, row[0])
			}
			if rowNum == 0 && !strings.Contains(row[0], "/vens/") {
				utils.LogInfof(true, "skipping the first row of the never-unpair-file because it's not an href.")
			}
			if rowNum > 0 && !strings.Contains(row[0], "/vens/") {
				utils.LogErrorf("csv row %d - %s is not a valid href.", rowNum, row[0])
			}
			neverUnpairHrefs[row[0]] = true
		}
	}

	hrefFileData, err := utils.ParseCSV(hrefFile)
	if err != nil {
		utils.LogErrorf("parsing csv - %s", err)
	}

	venHrefCol := 0
	vens := []illumioapi.VEN{}
	for rowIndex, row := range hrefFileData {
		if rowIndex == 0 {
			for colIndex, col := range row {
				if col == hrefHeader {
					venHrefCol = colIndex
					continue
				}
			}
		} else {
			// Skip if on the list
			if neverUnpairHrefs[row[venHrefCol]] {
				utils.LogInfof(true, "skipping %s because it is in the always skip list", row[venHrefCol])
			} else {
				vens = append(vens, illumioapi.VEN{Href: row[venHrefCol]})
			}
		}
	}

	return vens
}
