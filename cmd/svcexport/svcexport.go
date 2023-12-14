package svcexport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var outputFileName string
var riskData, noHref, compressed bool

func init() {
	SvcExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce. ignored with compressed flag.")
	SvcExportCmd.Flags().BoolVar(&compressed, "compressed", false, "compress the output to one service per line. this output is not compatible with the svc-import command.")
	SvcExportCmd.Flags().BoolVar(&riskData, "risk", false, "include risk info.")
	SvcExportCmd.Flags().MarkHidden("risk")
	SvcExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
}

// SvcExportCmd runs the workload identifier
var SvcExportCmd = &cobra.Command{
	Use:   "svc-export",
	Short: "Create a CSV export of all services in the PCE.",
	Long: `
Create a CSV export of all services in the PCE.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		ExportServices(pce, noHref, outputFileName, []string{})
	},
}

// ExportServices exports the services in the Illumio PCE to a CSV file.
// If hrefs is an empty slice, all services are exported. If there are entries in the hrefs slice, only those services will be exported
func ExportServices(pce illumioapi.PCE, templateFormat bool, outputFileName string, hrefs []string) {

	// GetAllServices
	a, err := pce.GetServices(nil, "draft")
	utils.LogAPIRespV2("GetAllSvcs", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	allSvcs := pce.ServicesSlice

	// Create a map of the provided hrefs
	providedHrefs := make(map[string]bool)
	for _, h := range hrefs {
		providedHrefs[h] = true
	}

	// Create the targetServices
	targetSvcs := []illumioapi.Service{}
	if len(hrefs) > 0 {
		for _, s := range allSvcs {
			if providedHrefs[s.Href] {
				targetSvcs = append(targetSvcs, s)
			}
		}
	} else {
		targetSvcs = allSvcs
	}

	csvData := [][]string{}

	if compressed {

		// Start the data slice with headers
		csvData = [][]string{{"name", "description", "service_ports", "window_services", "href"}}
		if riskData {
			csvData[0] = append(csvData[0], "ransomware_category", "ransomware_severity", "ransomware_os_platform")
		}

		for _, s := range targetSvcs {

			// Parse the services
			windowsServices, servicePorts := s.ParseService()

			// Add to the CSV data
			row := []string{s.Name, s.Description, strings.Join(servicePorts, ";"), strings.Join(windowsServices, ";"), s.Href}
			if riskData && s.RiskDetails.Ransomware != nil {
				row = append(row, s.RiskDetails.Ransomware.Category, s.RiskDetails.Ransomware.Severity, strings.Join(s.RiskDetails.Ransomware.OsPlatforms, ";"))
			}
			csvData = append(csvData, row)
		}

	}

	if !compressed {

		// Start the data slice with headers
		headers := []string{HeaderName, HeaderDescription, HeaderWinService, HeaderPort, HeaderProto, HeaderProcess, HeaderService, HeaderICMPCode, HeaderICMPType}
		if !templateFormat {
			headers = append(headers, HeaderHref)
		}
		if riskData {
			headers = append(headers, "ransomware_category", "ransomware_severity", "ransomware_os_platform")
		}
		csvData = [][]string{headers}

		for _, s := range targetSvcs {
			var isWinSvc bool
			if len(illumioapi.PtrToVal(s.WindowsServices)) > 0 {
				isWinSvc = true
			}

			var port, proto string
			for _, p := range illumioapi.PtrToVal(s.ServicePorts) {
				if p.ToPort != 0 {
					port = fmt.Sprintf("%d-%d", illumioapi.PtrToVal(p.Port), p.ToPort)
				} else if p.Protocol == 6 || p.Protocol == 17 {
					port = strconv.Itoa(illumioapi.PtrToVal(p.Port))
				}
				if p.Protocol == 6 {
					proto = "tcp"
				} else if p.Protocol == 17 {
					proto = "udp"
				} else {
					proto = strconv.Itoa(p.Protocol)
				}
				entry := []string{s.Name, s.Description, strconv.FormatBool(isWinSvc), port, proto, "", "", strconv.Itoa(p.IcmpCode), strconv.Itoa(p.IcmpType)}
				if !templateFormat {
					entry = append(entry, s.Href)
				}
				if riskData && s.RiskDetails != nil {
					entry = append(entry, s.RiskDetails.Ransomware.Category, s.RiskDetails.Ransomware.Severity, strings.Join(s.RiskDetails.Ransomware.OsPlatforms, ";"))
				}
				// Append to the CSV
				csvData = append(csvData, entry)
			}

			for _, p := range illumioapi.PtrToVal(s.WindowsServices) {
				if p.ToPort != 0 {
					port = fmt.Sprintf("%d-%d", illumioapi.PtrToVal(p.Port), p.ToPort)
				} else if p.Protocol == 6 || p.Protocol == 17 {
					port = strconv.Itoa(illumioapi.PtrToVal(p.Port))
				}
				if p.Protocol == 6 {
					proto = "tcp"
				} else if p.Protocol == 17 {
					proto = "udp"
				} else {
					proto = strconv.Itoa(p.Protocol)
				}
				entry := []string{s.Name, s.Description, strconv.FormatBool(isWinSvc), port, proto, p.ProcessName, p.ServiceName, strconv.Itoa(p.IcmpCode), strconv.Itoa(p.IcmpType)}
				if !templateFormat {
					entry = append(entry, s.Href)
				}

				if riskData && s.RiskDetails != nil {
					entry = append(entry, s.RiskDetails.Ransomware.Category, s.RiskDetails.Ransomware.Severity, strings.Join(s.RiskDetails.Ransomware.OsPlatforms, ";"))
				}

				// Append to the CSV
				csvData = append(csvData, entry)
			}

		}

	}

	// Output the CSV Data
	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-svc-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d services exported", len(targetSvcs)), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no services in PCE.", true)
	}

}
