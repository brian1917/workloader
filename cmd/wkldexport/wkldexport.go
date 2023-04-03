package wkldexport

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// WkldExport is used to export workloads
type WkldExport struct {
	PCE                                                                                       *illumioapi.PCE
	ManagedOnly, UnmanagedOnly, OnlineOnly, IncludeVuln, NoHref, RemoveDescNewLines, WriteCSV bool
	Headers, OutputFileName                                                                   string
}

// Declare local global variables
var wkldExport WkldExport
var err error

func init() {
	WkldExportCmd.Flags().StringVar(&wkldExport.Headers, "headers", "", "comma-separated list of headers for export. default is all headers.")
	WkldExportCmd.Flags().BoolVarP(&wkldExport.ManagedOnly, "managed-only", "m", false, "only export managed workloads.")
	WkldExportCmd.Flags().BoolVarP(&wkldExport.UnmanagedOnly, "unmanaged-only", "u", false, "only export unmanaged workloads.")
	WkldExportCmd.Flags().BoolVarP(&wkldExport.OnlineOnly, "online-only", "o", false, "only export online workloads.")
	WkldExportCmd.Flags().BoolVarP(&wkldExport.IncludeVuln, "incude-vuln-data", "v", false, "include vulnerability data.")
	WkldExportCmd.Flags().BoolVar(&wkldExport.NoHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	WkldExportCmd.Flags().StringVar(&wkldExport.OutputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	WkldExportCmd.Flags().BoolVar(&wkldExport.RemoveDescNewLines, "remove-desc-newline", false, "will remove new line characters in description field.")

	WkldExportCmd.Flags().SortFlags = false

}

// WkldExportCmd runs the workload identifier
var WkldExportCmd = &cobra.Command{
	Use:   "wkld-export",
	Short: "Create a CSV export of all workloads in the PCE.",
	Long: `
Create a CSV export of all workloads in the PCE.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		wkldExport.PCE = &illumioapi.PCE{}
		*wkldExport.PCE, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		wkldExport.WriteCSV = true
		wkldExport.ExportToCsv()
	},
}

// ExportToCsv epxorts a PCE workloads to a CSV
func (e *WkldExport) ExportToCsv() [][]string {

	// Log command execution
	utils.LogStartCommand("wkld-export")

	// Load the PCE if necessary
	load := illumioapi.LoadInput{}
	if len(e.PCE.WorkloadsSlice) == 0 {
		load.Workloads = true
		load.WorkloadsQueryParameters = make(map[string]string)
		if e.UnmanagedOnly {
			load.WorkloadsQueryParameters["managed"] = "false"
		}
		if e.ManagedOnly {
			load.WorkloadsQueryParameters["managed"] = "true"
		}
		if e.IncludeVuln {
			load.WorkloadsQueryParameters["representation"] = "workload_labels_vulnerabilities"
		}
		if e.OnlineOnly {
			load.WorkloadsQueryParameters["online"] = "true"
		}
	}
	if len(e.PCE.LabelsSlice) == 0 {
		load.Labels = true
	}
	apiResps, err := e.PCE.Load(load, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

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

	// Start the outputdata
	outputData := [][]string{}
	headerRow := []string{}
	// If no user headers provided, get all the headers
	if e.Headers == "" {
		for _, header := range AllHeaders(e.IncludeVuln, !e.NoHref) {
			headerRow = append(headerRow, header)
			// Insert the labels either after href or hostname
			if (!e.NoHref && header == "href") || (e.NoHref && header == "name") {
				headerRow = append(headerRow, labelsKeySlice...)
			}
		}
		outputData = append(outputData, headerRow)
	} else {
		outputData = append(outputData, strings.Split(strings.Replace(e.Headers, " ", "", -1), ","))
	}

	// Iterate through each workload
	for _, w := range e.PCE.WorkloadsSlice {
		csvRow := make(map[string]string)
		// Skip deleted workloads
		if *w.Deleted {
			continue
		}

		// Get interfaces
		csvRow[HeaderInterfaces] = strings.Join(InterfaceToString(w, false), ";")

		// Get Managed Status
		csvRow[HeaderManaged] = "false"
		if (w.Agent != nil && w.Agent.Href != "") || (w.VEN != nil && w.VEN.Href != "") {
			csvRow[HeaderManaged] = "true"
		}

		// Assume the VEN-dependent fields are unmanaged
		csvRow[HeaderSecurityPolicySyncState] = "unmanaged"
		csvRow[HeaderSecurityPolicyAppliedAt] = "unmanaged"
		csvRow[HeaderSecurityPolicyReceivedAt] = "unmanaged"
		csvRow[HeaderSecurityPolicyRefreshAt] = "unmanaged"
		csvRow[HeaderAgentVersion] = "unmanaged"
		csvRow[HeaderLastHeartbeatOn] = "unmanaged"
		csvRow[HeaderHoursSinceLastHeartbeat] = "unmanaged"
		csvRow[HeaderAgentID] = "unmanaged"
		csvRow[HeaderActivePceFqdn] = "unmanaged"
		csvRow[HeaderAgentStatus] = "unmanaged"
		csvRow[HeaderCloudInstanceID] = "unmanaged"
		csvRow[HeaderAgentHealth] = "unmanaged"
		csvRow[HeaderVenHref] = "unmanaged"
		// If it is managed, get that information
		if w.Agent != nil && w.Agent.Href != "" {
			csvRow[HeaderSecurityPolicySyncState] = w.Agent.Status.SecurityPolicySyncState
			csvRow[HeaderSecurityPolicyAppliedAt] = w.Agent.Status.SecurityPolicyAppliedAt
			csvRow[HeaderSecurityPolicyReceivedAt] = w.Agent.Status.SecurityPolicyReceivedAt
			csvRow[HeaderSecurityPolicyRefreshAt] = w.Agent.Status.SecurityPolicyRefreshAt
			csvRow[HeaderAgentVersion] = w.Agent.Status.AgentVersion
			csvRow[HeaderLastHeartbeatOn] = w.Agent.Status.LastHeartbeatOn
			csvRow[HeaderHoursSinceLastHeartbeat] = fmt.Sprintf("%f", w.HoursSinceLastHeartBeat())
			csvRow[HeaderAgentID] = w.Agent.GetID()
			csvRow[HeaderActivePceFqdn] = w.Agent.ActivePceFqdn
			if csvRow[HeaderActivePceFqdn] == "" {
				csvRow[HeaderActivePceFqdn] = e.PCE.FQDN
			}
			csvRow[HeaderAgentStatus] = w.Agent.Status.Status
			csvRow[HeaderCloudInstanceID] = w.Agent.Status.InstanceID
			if csvRow[HeaderCloudInstanceID] == "" {
				csvRow[HeaderCloudInstanceID] = "NA"
			}
			if w.Agent.Status.AgentHealth != nil && len(illumioapi.PtrToVal(w.Agent.Status.AgentHealth)) > 0 {
				healthSlice := []string{}
				for _, a := range illumioapi.PtrToVal(w.Agent.Status.AgentHealth) {
					healthSlice = append(healthSlice, fmt.Sprintf("%s (%s)", a.Type, a.Severity))
				}
				csvRow[HeaderAgentHealth] = strings.Join(healthSlice, "; ")
			} else {
				csvRow[HeaderAgentHealth] = "NA"
			}
		}

		// Start using VEN properties
		if w.VEN != nil {
			csvRow[HeaderVenHref] = w.VEN.Href
		}

		// Remove newlines in description
		if e.RemoveDescNewLines && w.Description != nil {
			*w.Description = utils.ReplaceNewLine(*w.Description)
		}

		// Get the labels
		for _, labelKey := range labelsKeySlice {
			csvRow[labelKey] = w.GetLabelByKey(labelKey, e.PCE.Labels).Value
		}

		// Fill csv row with other data
		csvRow[HeaderHostname] = illumioapi.PtrToVal(w.Hostname)
		csvRow[HeaderName] = illumioapi.PtrToVal(w.Name)
		csvRow[HeaderHref] = w.Href

		csvRow[HeaderPublicIP] = illumioapi.PtrToVal(w.PublicIP)
		csvRow[HeaderDistinguishedName] = illumioapi.PtrToVal(w.DistinguishedName)
		csvRow[HeaderIPWithDefaultGw] = w.GetIPWithDefaultGW()
		csvRow[HeaderNetmaskOfIPWithDefGw] = w.GetNetMaskWithDefaultGW()
		csvRow[HeaderDefaultGw] = w.GetDefaultGW()
		csvRow[HeaderDefaultGwNetwork] = w.GetNetworkWithDefaultGateway()
		csvRow[HeaderSPN] = illumioapi.PtrToVal(w.ServicePrincipalName)
		csvRow[HeaderDescription] = illumioapi.PtrToVal(w.Description)
		csvRow[HeaderEnforcement] = w.GetMode()
		csvRow[HeaderVisibility] = w.GetVisibilityLevel()
		csvRow[HeaderOnline] = strconv.FormatBool(illumioapi.PtrToVal(w.Online))
		csvRow[HeaderCreatedAt] = w.CreatedAt
		csvRow[HeaderOsID] = illumioapi.PtrToVal(w.OsID)
		csvRow[HeaderOsDetail] = illumioapi.PtrToVal(w.OsDetail)
		csvRow[HeaderServiceProvider] = illumioapi.PtrToVal(w.ServiceProvider)
		csvRow[HeaderDataCenter] = illumioapi.PtrToVal(w.DataCenter)
		csvRow[HeaderDataCenterZone] = illumioapi.PtrToVal(w.DataCenterZone)
		csvRow[HeaderExternalDataReference] = illumioapi.PtrToVal(w.ExternalDataReference)
		csvRow[HeaderExternalDataSet] = illumioapi.PtrToVal(w.ExternalDataSet)

		if e.IncludeVuln {
			var maxVulnScore, vulnScore, vulnExposureScore string
			targets := []*string{&maxVulnScore, &vulnScore, &vulnExposureScore}
			if w.VulnerabilitySummary != nil {
				values := []int{w.VulnerabilitySummary.MaxVulnerabilityScore, w.VulnerabilitySummary.VulnerabilityScore, w.VulnerabilitySummary.VulnerabilityExposureScore}
				for i, t := range targets {
					*t = strconv.Itoa(int((math.Round((float64(values[i]) / float64(10))))))
				}

				csvRow[HeaderNumVulns] = strconv.Itoa(w.VulnerabilitySummary.NumVulnerabilities)
				csvRow[HeaderVulnPortExposure] = strconv.Itoa(w.VulnerabilitySummary.VulnerablePortExposure)
				csvRow[HeaderAnyVulnExposure] = strconv.FormatBool(w.VulnerabilitySummary.VulnerablePortWideExposure.Any)
				csvRow[HeaderIpListVulnExposure] = strconv.FormatBool(w.VulnerabilitySummary.VulnerablePortWideExposure.IPList)
				csvRow[HeaderMaxVulnScore] = maxVulnScore
				csvRow[HeaderVulnScore] = vulnScore
				csvRow[HeaderVulnExposureScore] = vulnExposureScore
			}
		}

		newRow := []string{}
		for _, header := range outputData[0] {
			newRow = append(newRow, csvRow[header])
		}
		outputData = append(outputData, newRow)
	}

	if !e.WriteCSV {
		return outputData
	}

	if len(outputData) > 1 {
		if e.OutputFileName == "" {
			e.OutputFileName = fmt.Sprintf("workloader-wkld-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(outputData, outputData, e.OutputFileName)
		utils.LogInfo(fmt.Sprintf("%d workloads exported", len(outputData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no workloads in PCE.", true)
	}

	utils.LogEndCommand("wkld-export")
	return [][]string{}

}

func InterfaceToString(w illumioapi.Workload, replaceDots bool) (interfaces []string) {
	for _, i := range illumioapi.PtrToVal(w.Interfaces) {
		if replaceDots {
			i.Name = strings.Replace(i.Name, ".", "-", -1)
		}
		ipAddress := fmt.Sprintf("%s:%s", i.Name, i.Address)
		if i.CidrBlock != nil && *i.CidrBlock != 0 {
			ipAddress = fmt.Sprintf("%s:%s/%s", i.Name, i.Address, strconv.Itoa(*i.CidrBlock))
		}
		interfaces = append(interfaces, ipAddress)
	}
	return interfaces
}
