package iplexport

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
var outFormat, outputFileName string

func init() {
	IplExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

}

// IplExportCmd runs the workload identifier
var IplExportCmd = &cobra.Command{
	Use:   "ipl-export",
	Short: "Create a CSV export of all IP lists in the PCE.",
	Long: `
Create a CSV export of all workloads in the PCE. The update-pce and --no-prompt flags are ignored for this command.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the viper values
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		exportIPL()
	},
}

func exportIPL() {

	// Log command execution
	utils.LogStartCommand("ipl-export")

	// Start the data slice with headers
	csvData := [][]string{[]string{"name", "description", "include", "exclude", "fqdns", "external_data_set", "external_data_ref", "href"}}

	// Get all IPLists
	ipls, a, err := pce.GetAllDraftIPLists()
	utils.LogAPIResp("GetAllDraftIPLists", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	for _, i := range ipls {
		exclude := []string{}
		include := []string{}
		for _, r := range i.IPRanges {
			entry := r.FromIP
			if r.ToIP != "" {
				entry = fmt.Sprintf("%s-%s", r.FromIP, r.ToIP)
			}
			if r.Exclusion {
				exclude = append(exclude, entry)
			} else {
				include = append(include, entry)
			}
		}

		fqdns := []string{}
		for _, f := range i.FQDNs {
			fqdns = append(fqdns, f.FQDN)

		}
		csvData = append(csvData, []string{i.Name, i.Description, strings.Join(include, ";"), strings.Join(exclude, ";"), strings.Join(fqdns, ";"), i.ExternalDataSet, i.ExternalDataReference, i.Href})
	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-ipl-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d iplists exported.", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no iplists in PCE.", true)
	}
	utils.LogEndCommand("ipl-export")
}
