package venexport

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var headers, outputFileName string
var exclServer, exclEndpoint, exclContainerized bool

func init() {
	VenExportCmd.Flags().StringVar(&headers, "headers", "", "comma-separated list of headers for export. default is all headers.")
	VenExportCmd.Flags().BoolVar(&exclServer, "excl-server", false, "exclude server vens.")
	VenExportCmd.Flags().BoolVar(&exclEndpoint, "excl-endpoint", false, "exclude server vens.")
	VenExportCmd.Flags().BoolVar(&exclContainerized, "excl-containerized", false, "exclude containerized vens.")
	VenExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	VenExportCmd.Flags().SortFlags = false
}

// WkldExportCmd runs the workload identifier
var VenExportCmd = &cobra.Command{
	Use:   "ven-export",
	Short: "Create a CSV export of all VENs in the PCE.",
	Long: `
Create a CSV export of all VENs in the PCE. This file can be used in the ven-import command to update VENs.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Log command execution
		utils.LogStartCommand("ven-export")

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}
		headersSlice := []string{}
		if headers != "" {
			headers = strings.Replace(headers, ", ", ",", -1)
			headersSlice = strings.Split(headers, ",")
		}

		exportVens(&pce, headersSlice)
	},
}

// WkldExport is used to export workloads
type VenExport struct {
	PCE     *illumioapi.PCE
	Headers []string
}

func (e *VenExport) CsvData() (csvData [][]string) {
	// Get the labels that are in use by the workloads
	labelsKeyMap := make(map[string]bool)
	for _, w := range e.PCE.WorkloadsSlice {
		for _, label := range *w.Labels {
			labelsKeyMap[e.PCE.Labels[label.Href].Key] = true
		}
	}
	labelsKeySlice := []string{}
	for labelKey := range labelsKeyMap {
		labelsKeySlice = append(labelsKeySlice, labelKey)
	}
	// Sort the slice of label keys
	sort.Strings(labelsKeySlice)

	// Get workload export data
	wkldExport := wkldexport.WkldExport{
		PCE:                e.PCE,
		RemoveDescNewLines: false,
		Headers:            append([]string{"ven_href", wkldexport.HeaderHref}, labelsKeySlice...),
	}
	wkldMap := wkldExport.MapData()

	// Start the outputdata
	headerRow := []string{}
	// If no user headers provided, get all the headers
	if len(e.Headers) == 0 {
		for _, header := range AllHeaders() {
			headerRow = append(headerRow, header)
			// Insert the labels either after description
			if header == HeaderDescription {
				headerRow = append(headerRow, labelsKeySlice...)
			}
		}
		csvData = append(csvData, headerRow)
	} else {
		csvData = append(csvData, e.Headers)
	}

	for _, v := range pce.VENsSlice {
		csvRow := make(map[string]string)

		// Get container cluster
		if v.ContainerCluster != nil {
			if val, ok := pce.ContainerClusters[v.ContainerCluster.Href]; ok {
				csvRow[HeaderContainerCluster] = val.Name
			}
		}

		// Get the VEN health
		csvRow[HeaderHealth] = "healthy"
		healthMessages := []string{}
		if len(illumioapi.PtrToVal(v.Conditions)) > 0 {
			for _, c := range illumioapi.PtrToVal(v.Conditions) {
				healthMessages = append(healthMessages, c.LatestEvent.NotificationType)
			}
			csvRow[HeaderHealth] = strings.Join(healthMessages, "; ")
		}

		csvRow[HeaderName] = illumioapi.PtrToVal(v.Name)
		csvRow[HeaderHostname] = illumioapi.PtrToVal(v.Hostname)
		csvRow[HeaderDescription] = illumioapi.PtrToVal(v.Description)
		csvRow[HeaderVenType] = v.VenType
		csvRow[HeaderStatus] = v.Status
		csvRow[HeaderVersion] = v.Version
		csvRow[HeaderActivationType] = v.ActivationType
		csvRow[HeaderActivePceFqdn] = v.ActivePceFqdn
		csvRow[HeaderTargetPceFqdn] = illumioapi.PtrToVal(v.TargetPceFqdn)
		csvRow[HeaderWorkloads] = wkldMap[v.Href][wkldexport.HeaderHostname]
		csvRow[HeaderHref] = v.Href
		csvRow[HeaderUID] = v.UID
		csvRow[HeaderWkldHref] = wkldMap[v.Href][wkldexport.HeaderHref]
		for _, ld := range labelsKeySlice {
			csvRow[ld] = wkldMap[v.Href][ld]

		}
		newRow := []string{}
		for _, header := range csvData[0] {
			newRow = append(newRow, csvRow[header])
		}
		csvData = append(csvData, newRow)
	}
	return csvData
}

func exportVens(pce *illumioapi.PCE, headers []string) {

	// Load the pce
	utils.LogInfo("getting workloads, vens, labels, label dimensions, container clusters, and container workloads...", true)
	apiResps, err := pce.Load(illumioapi.LoadInput{
		Workloads:                true,
		WorkloadsQueryParameters: map[string]string{"managed": "true"},
		Labels:                   true,
		VENs:                     true,
		ContainerClusters:        true,
		ContainerWorkloads:       true,
		LabelDimensions:          true,
	}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Eliminate VEN types
	venSlice := []illumioapi.VEN{}
	for key, ven := range pce.VENs {
		if (exclServer && ven.VenType == "server") || (exclEndpoint && ven.VenType == "endpoint") || (exclContainerized && ven.VenType == "containerized") {
			delete(pce.VENs, key)
		}
	}
	for _, ven := range pce.VENsSlice {
		if (exclServer && ven.VenType == "server") || (exclEndpoint && ven.VenType == "endpoint") || (exclContainerized && ven.VenType == "containerized") {
			continue
		}
		venSlice = append(venSlice, ven)
	}
	pce.VENsSlice = venSlice

	// Get the csvData
	e := VenExport{
		PCE:     pce,
		Headers: headers,
	}

	csvData := e.CsvData()

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-ven-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d vens exported", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no vens in PCE.", true)
	}

	utils.LogEndCommand("ven-export")
}
