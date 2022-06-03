package iplexport

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/cmd/iplimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var iplName, outputFileName string

func init() {
	IplExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
}

// IplExportCmd runs the workload identifier
var IplExportCmd = &cobra.Command{
	Use:   "ipl-export [optional name of ip list]",
	Short: "Create a CSV export of all IP lists in the PCE.",
	Long: `
Create a CSV export IP Lists in the PCE.

If a name argument is provided the output will be two CSVs: one for IP entries and one for FQDNs. This format can be used as import to the ipl-replace command.

If no arguments are provided, all IP Lists are exported into a single CSV with each IP List on a line and IP entries separated by semi-colons. This format can be used as import to the ipl-import command.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) > 1 {
			fmt.Println("command only accepts 1 or no arguments for the ip list name. See usage help.")
			os.Exit(0)
		}
		if len(args) > 0 {
			iplName = args[0]
		}

		ExportIPL(pce, iplName, outputFileName)
	},
}

func ExportIPL(pce illumioapi.PCE, iplName, outputFileName string) {

	// Log command execution
	utils.LogStartCommand("ipl-export")

	if iplName == "" {

		// Start the data slice with headers
		csvData := [][]string{{iplimport.HeaderName, iplimport.HeaderDescription, iplimport.HeaderInclude, iplimport.HeaderExclude, iplimport.HeaderFqdns, iplimport.HeaderExternalDataSet, iplimport.HeaderExternalDataRef, iplimport.HeaderHref}}

		// Get all IPLists
		ipls, a, err := pce.GetIPLists(nil, "draft")
		utils.LogAPIResp("GetAllDraftIPLists", a)
		if err != nil {
			utils.LogError(err.Error())
		}

		for _, i := range ipls {
			exclude := []string{}
			include := []string{}
			if i.IPRanges != nil {
				for _, r := range *i.IPRanges {
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
			}

			fqdns := []string{}
			for _, f := range *i.FQDNs {
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
		return
	}

	// Get here if we are given a name

	// Get the IP list by name
	ipl, a, err := pce.GetIPListByName(iplName, "draft")
	utils.LogAPIResp("GetIPList", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	if ipl.Href == "" {
		utils.LogError(fmt.Sprintf("%s does not exist as an ip list in the PCE", iplName))
	}
	// Start the data slice with headers
	ipEntrycsvData := [][]string{{"ip", "description"}}

	if ipl.IPRanges != nil {
		for _, r := range *ipl.IPRanges {
			entry := r.FromIP
			if r.ToIP != "" {
				entry = fmt.Sprintf("%s-%s", r.FromIP, r.ToIP)
			}
			if r.Exclusion {
				entry = "!" + entry
			}
			ipEntrycsvData = append(ipEntrycsvData, []string{entry, r.Description})
		}
	}
	if len(ipEntrycsvData) > 1 {
		var iplOutputFileName string
		if outputFileName == "" {
			iplOutputFileName = fmt.Sprintf("workloader-ipl-export-%s-ip_entries-%s.csv", iplName, time.Now().Format("20060102_150405"))
		} else {
			outFileSlice := strings.Split(outputFileName, ".")
			for i, s := range outFileSlice {
				if i == len(outFileSlice)-2 {
					iplOutputFileName = iplOutputFileName + s + "-ip_entries"
				} else {
					iplOutputFileName = iplOutputFileName + "." + s
				}
			}
		}
		utils.WriteOutput(ipEntrycsvData, ipEntrycsvData, iplOutputFileName)
		utils.LogInfo(fmt.Sprintf("%d ip entries exported to %s.", len(ipEntrycsvData)-1, iplOutputFileName), true)
	}

	fqdnCsvData := [][]string{{"fqdn"}}
	if ipl.FQDNs != nil && len(*ipl.FQDNs) != 0 {
		for _, f := range *ipl.FQDNs {
			fqdnCsvData = append(fqdnCsvData, []string{f.FQDN})
		}
	}
	if len(fqdnCsvData) > 1 {
		var fqdnOutputFileName string
		if outputFileName == "" {
			fqdnOutputFileName = fmt.Sprintf("workloader-ipl-export-%s-fqdn_entries-%s.csv", iplName, time.Now().Format("20060102_150405"))
		} else {
			outFileSlice := strings.Split(outputFileName, ".")
			for i, s := range outFileSlice {
				if i == len(outFileSlice)-2 {
					fqdnOutputFileName = fqdnOutputFileName + s + "-fqdn_entries"
				} else {
					fqdnOutputFileName = fqdnOutputFileName + "." + s
				}
			}
		}
		utils.WriteOutput(fqdnCsvData, fqdnCsvData, fqdnOutputFileName)
		utils.LogInfo(fmt.Sprintf("%d fqdn entries exported to %s.", len(fqdnCsvData)-1, fqdnOutputFileName), true)
	}

}
