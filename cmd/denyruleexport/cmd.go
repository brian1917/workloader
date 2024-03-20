package denyruleexport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	ia "github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare global variables for flags
var noHref, expandServices bool
var exportHeaders, outputFileName string

func init() {
	DenyRuleExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	DenyRuleExportCmd.Flags().BoolVar(&expandServices, "expand-svcs", false, "expand service objects to show ports/protocols (not compatible in deny-rule-import format).")
	DenyRuleExportCmd.Flags().StringVar(&exportHeaders, "headers", "", "comma-separated list of headers for export. default is all headers.")
	DenyRuleExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	DenyRuleExportCmd.Flags().SortFlags = false
}

// DenyRuleExportCmd runs the deny-rule-export command
var DenyRuleExportCmd = &cobra.Command{
	Use:   "deny-rule-export",
	Short: "Create a CSV export of all deny rules in the PCE.",
	Long: `
Create a CSV export of all deny rules in the PCE.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Call the export function
		ExportEBs(pce, outputFileName, noHref)
	},
}

func ExportEBs(pce ia.PCE, outputFileName string, noHref bool) {

	// Get needed obects
	utils.LogInfo("getting boundaries, labels, label groups, iplists, and services...", true)
	apiResps, err := pce.Load(ia.LoadInput{
		EnforcementBoundaries: true,
		Labels:                true,
		LabelGroups:           true,
		IPLists:               true,
		Services:              true,
	}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Start the CSV data
	csvData := [][]string{}
	if exportHeaders != "" {
		exportHeaders = strings.Replace(exportHeaders, ", ", ",", -1)
		csvData = append(csvData, strings.Split(exportHeaders, ","))

	} else {
		csvData = append(csvData, AllHeaders(noHref))
	}

	// Iterate through each boundary
	for _, eb := range pce.EnforcementBoundariesSlice {
		// Create the csv row and start with the strings
		csvRow := map[string]string{
			HeaderName:        eb.Name,
			HeaderHref:        eb.Href,
			HeaderCreatedAt:   eb.CreatedAt,
			HeaderUpdateType:  eb.UpdateType,
			HeaderUpdatedAt:   eb.UpdatedAt,
			HeaderNetworkType: eb.NetworkType,
		}
		// Process enabled
		if eb.Enabled != nil {
			csvRow[HeaderEnabled] = strconv.FormatBool(*eb.Enabled)
		}

		// Sources
		srcLabels := []string{}
		if eb.Consumers != nil {
			for _, c := range *eb.Consumers {
				// All workloads
				if ia.PtrToVal(c.Actors) == "ams" {
					csvRow[HeaderSrcAllWorkloads] = "true"
					continue
				}
				// IP Lists
				if c.IPList != nil {
					if val, ok := csvRow[HeaderSrcIPLists]; ok {
						csvRow[HeaderSrcIPLists] = fmt.Sprintf("%s;%s", val, pce.IPLists[c.IPList.Href].Name)
					} else {
						csvRow[HeaderSrcIPLists] = pce.IPLists[c.IPList.Href].Name
					}
				}
				// Labels
				if c.Label != nil {
					srcLabels = append(srcLabels, fmt.Sprintf("%s:%s", pce.Labels[c.Label.Href].Key, pce.Labels[c.Label.Href].Value))
				}
				// Label Groups
				if c.LabelGroup != nil {
					if val, ok := csvRow[HeaderSrcLabelGroups]; ok {
						csvRow[HeaderSrcLabelGroups] = fmt.Sprintf("%s;%s", val, pce.LabelGroups[c.LabelGroup.Href].Name)
					} else {
						csvRow[HeaderSrcLabelGroups] = pce.LabelGroups[c.LabelGroup.Href].Name
					}
				}
			}
		}
		csvRow[HeaderSrcLabels] = strings.Join(srcLabels, ";")

		// Destinations
		dstLabels := []string{}
		if eb.Providers != nil {
			for _, p := range *eb.Providers {
				// All workloads
				if ia.PtrToVal(p.Actors) == "ams" {
					csvRow[HeaderDstAllWorkloads] = "true"
					continue
				}
				// IP Lists
				if p.IPList != nil {
					if val, ok := csvRow[HeaderDstIPLists]; ok {
						csvRow[HeaderDstIPLists] = fmt.Sprintf("%s;%s", val, pce.IPLists[p.IPList.Href].Name)
					} else {
						csvRow[HeaderDstIPLists] = pce.IPLists[p.IPList.Href].Name
					}
				}
				// Labels
				if p.Label != nil {
					dstLabels = append(dstLabels, fmt.Sprintf("%s:%s", pce.Labels[p.Label.Href].Key, pce.Labels[p.Label.Href].Value))
				}
				// Label Groups
				if p.LabelGroup != nil {
					if val, ok := csvRow[HeaderDstLabelGroups]; ok {
						csvRow[HeaderDstLabelGroups] = fmt.Sprintf("%s;%s", val, pce.LabelGroups[p.LabelGroup.Href].Name)
					} else {
						csvRow[HeaderDstLabelGroups] = pce.LabelGroups[p.LabelGroup.Href].Name
					}
				}
			}
		}

		csvRow[HeaderDstLabels] = strings.Join(dstLabels, ";")

		// Services
		services := []string{}

		// Iterate through ingress service
		if eb.IngressServices != nil {
			for _, s := range *eb.IngressServices {
				// Windows Services
				if pce.Services[s.Href].WindowsServices != nil {
					a := pce.Services[s.Href]
					b, _ := a.ParseService()
					if !expandServices {
						services = append(services, pce.Services[s.Href].Name)
					} else {
						services = append(services, fmt.Sprintf("%s (%s)", pce.Services[s.Href].Name, strings.Join(b, ";")))
					}
					// Skip processing the services
					continue
				}
				// Port/Proto Services
				if pce.Services[s.Href].ServicePorts != nil {
					a := pce.Services[s.Href]
					_, b := a.ParseService()
					if pce.Services[s.Href].Name == "All Services" {
						services = append(services, "All Services")
					} else {
						if !expandServices {
							services = append(services, pce.Services[s.Href].Name)
						} else {
							services = append(services, fmt.Sprintf("%s (%s)", pce.Services[s.Href].Name, strings.Join(b, ";")))
						}
					}
					// Skip processing the services
					continue
				}

				// Port or port ranges
				protocol := ia.ProtocolList()[ia.PtrToVal(s.Protocol)]
				if ia.PtrToVal(s.Protocol) == 0 {
					protocol = ""
				}
				if ia.PtrToVal(s.ToPort) == 0 {
					services = append(services, fmt.Sprintf("%d %s", ia.PtrToVal(s.Port), protocol))
				} else {
					services = append(services, fmt.Sprintf("%d-%d %s", ia.PtrToVal(s.Port), ia.PtrToVal(s.ToPort), protocol))
				}
			}
		}
		csvRow[HeaderServices] = strings.Join(services, ";")

		// Adjust some blanks
		if csvRow[HeaderSrcAllWorkloads] == "" {
			csvRow[HeaderSrcAllWorkloads] = "false"
		}
		if csvRow[HeaderDstAllWorkloads] == "" {
			csvRow[HeaderDstAllWorkloads] = "false"
		}

		// Add the CSV file
		newRow := []string{}
		for _, header := range csvData[0] {
			newRow = append(newRow, csvRow[header])
		}
		csvData = append(csvData, newRow)
	}

	// Write the output file
	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-deny-rule-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
	} else {
		utils.LogInfo("no deny rules in pce", true)
	}

}
