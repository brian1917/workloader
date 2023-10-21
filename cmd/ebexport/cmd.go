package ebexport

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
	EbExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	EbExportCmd.Flags().BoolVar(&expandServices, "expand-svcs", false, "expand service objects to show ports/protocols (not compatible in eb-import format).")
	EbExportCmd.Flags().StringVar(&exportHeaders, "headers", "", "comma-separated list of headers for export. default is all headers.")
	EbExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	EbExportCmd.Flags().SortFlags = false
}

// EbExportCmd runs the eb-export command
var EbExportCmd = &cobra.Command{
	Use:   "eb-export",
	Short: "Create a CSV export of all enforcement boundaries in the PCE.",
	Long: `
Create a CSV export of all enforcement boundaries in the PCE.

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

		// Consumers
		consumerLabels := []string{}
		if eb.Consumers != nil {
			for _, c := range *eb.Consumers {
				// All workloads
				if ia.PtrToVal(c.Actors) == "ams" {
					csvRow[HeaderConsumerAllWorkloads] = "true"
					continue
				}
				// IP Lists
				if c.IPList != nil {
					if val, ok := csvRow[HeaderConsumerIPLists]; ok {
						csvRow[HeaderConsumerIPLists] = fmt.Sprintf("%s;%s", val, pce.IPLists[c.IPList.Href].Name)
					} else {
						csvRow[HeaderConsumerIPLists] = pce.IPLists[c.IPList.Href].Name
					}
				}
				// Labels
				if c.Label != nil {
					consumerLabels = append(consumerLabels, fmt.Sprintf("%s:%s", pce.Labels[c.Label.Href].Key, pce.Labels[c.Label.Href].Value))
				}
				// Label Groups
				if c.LabelGroup != nil {
					if val, ok := csvRow[HeaderConsumerLabelGroups]; ok {
						csvRow[HeaderConsumerLabelGroups] = fmt.Sprintf("%s;%s", val, pce.LabelGroups[c.LabelGroup.Href].Name)
					} else {
						csvRow[HeaderConsumerLabelGroups] = pce.LabelGroups[c.LabelGroup.Href].Name
					}
				}
			}
		}
		csvRow[HeaderConsumerLabels] = strings.Join(consumerLabels, ";")

		// Providers
		providerLabels := []string{}
		if eb.Providers != nil {
			for _, p := range *eb.Providers {
				// All workloads
				if ia.PtrToVal(p.Actors) == "ams" {
					csvRow[HeaderProviderAllWorkloads] = "true"
					continue
				}
				// IP Lists
				if p.IPList != nil {
					if val, ok := csvRow[HeaderProviderIPLists]; ok {
						csvRow[HeaderProviderIPLists] = fmt.Sprintf("%s;%s", val, pce.IPLists[p.IPList.Href].Name)
					} else {
						csvRow[HeaderProviderIPLists] = pce.IPLists[p.IPList.Href].Name
					}
				}
				// Labels
				if p.Label != nil {
					providerLabels = append(providerLabels, fmt.Sprintf("%s:%s", pce.Labels[p.Label.Href].Key, pce.Labels[p.Label.Href].Value))
				}
				// Label Groups
				if p.LabelGroup != nil {
					if val, ok := csvRow[HeaderProviderLabelGroups]; ok {
						csvRow[HeaderProviderLabelGroups] = fmt.Sprintf("%s;%s", val, pce.LabelGroups[p.LabelGroup.Href].Name)
					} else {
						csvRow[HeaderProviderLabelGroups] = pce.LabelGroups[p.LabelGroup.Href].Name
					}
				}
			}
		}

		csvRow[HeaderProviderLabels] = strings.Join(providerLabels, ";")

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
		if csvRow[HeaderConsumerAllWorkloads] == "" {
			csvRow[HeaderConsumerAllWorkloads] = "false"
		}
		if csvRow[HeaderProviderAllWorkloads] == "" {
			csvRow[HeaderProviderAllWorkloads] = "false"
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
			outputFileName = fmt.Sprintf("workloader-eb-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
	} else {
		utils.LogInfo("no enforcement boundaries in pce", true)
	}

	utils.LogEndCommand("eb-export")

}
