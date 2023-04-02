package venexport

import (
	"fmt"
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
var outputFileName string

// WkldExportCmd runs the workload identifier
var VenExportCmd = &cobra.Command{
	Use:   "ven-export",
	Short: "Create a CSV export of all VENs in the PCE.",
	Long: `
Create a CSV export of all VENs in the PCE. This file can be used in the ven-import command to update VENs.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		exportVens()
	},
}

func exportVens() {

	// Log command execution
	utils.LogStartCommand("ven-export")

	// Get workload export data
	wkldExport := wkldexport.WkldExport{
		PCE:                &pce,
		ManagedOnly:        true,
		IncludeVuln:        false,
		NoHref:             false,
		RemoveDescNewLines: false,
		UnmanagedOnly:      false,
		OnlineOnly:         false,
		WriteCSV:           false,
	}
	utils.LogInfo("getting all workloads...", true)
	wkldExportData := wkldExport.ExportToCsv()

	// Build a map of entries in the CSV data
	headers := []string{}
	wkldMap := make(map[string]map[string]string)
	venCol := 0
	for rowIndex, row := range wkldExportData {
		// Process the headers
		if rowIndex == 0 {
			for colIndex, header := range row {
				headers = append(headers, header)
				if header == wkldexport.HeaderVenHref {
					venCol = colIndex
				}
			}
			continue
		}
		// Process the rows
		wkldMap[row[venCol]] = make(map[string]string)
		for colIndex, entry := range row {
			if colIndex == venCol {
				continue
			}
			wkldMap[row[venCol]][headers[colIndex]] = entry
		}
	}

	// Get the label dimesnions
	api, err := wkldExport.PCE.GetLabelDimensions(nil)
	utils.LogAPIRespV2("GetLabelDimensions", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	labelDimensions := []string{}
	for _, ld := range wkldExport.PCE.LabelDimensionsSlice {
		labelDimensions = append(labelDimensions, ld.Key)
	}

	// Start the data slice with headers
	csvData := [][]string{{HeaderName, HeaderHostname, HeaderDescription, HeaderVenType, HeaderStatus, HeaderHealth, HeaderVersion, HeaderActivationType, HeaderActivePceFqdn, HeaderTargetPceFqdn, HeaderWorkloads, HeaderContainerCluster, HeaderHref, HeaderUID}}
	csvData[0] = append(csvData[0], labelDimensions...)

	// Load the PCE
	utils.LogInfo("getting all vens, conatiner clusters, and container workloads...", true)
	apiResps, err := pce.Load(illumioapi.LoadInput{VENs: true, ContainerClusters: true, ContainerWorkloads: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	utils.LogInfo("processing exports...", true)
	for _, v := range pce.VENsSlice {

		// Get container cluster
		ccName := ""
		if v.ContainerCluster != nil {
			if val, ok := pce.ContainerClusters[v.ContainerCluster.Href]; ok {
				ccName = val.Name
			}
		}

		// Get the VEN health
		health := "healthy"
		healthMessages := []string{}
		if len(illumioapi.PtrToVal(v.Conditions)) > 0 {
			for _, c := range illumioapi.PtrToVal(v.Conditions) {
				healthMessages = append(healthMessages, c.LatestEvent.NotificationType)
			}
			health = strings.Join(healthMessages, "; ")
		}

		row := []string{illumioapi.PtrToVal(v.Name), illumioapi.PtrToVal(v.Hostname), illumioapi.PtrToVal(v.Description), v.VenType, v.Status, health, v.Version, v.ActivationType, v.ActivePceFqdn, illumioapi.PtrToVal(v.TargetPceFqdn), wkldMap[v.Href][wkldexport.HeaderHostname], ccName, v.Href, v.UID}
		for _, ld := range labelDimensions {
			row = append(row, wkldMap[v.Href][ld])
		}

		csvData = append(csvData, row)
	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-ven-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d vens exported", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no vens in PCE.", true)
	}

	utils.LogEndCommand("ven-export")
}
