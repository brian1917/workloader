package wkldexport

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var managedOnly, unmanagedOnly, onlineOnly, includeVuln, noHref, removeDescNewLines bool
var exportHeaders, outputFileName string

func init() {
	WkldExportCmd.Flags().StringVar(&exportHeaders, "headers", "", "comma-separated list of headers for export. default is all headers.")
	WkldExportCmd.Flags().BoolVarP(&managedOnly, "managed-only", "m", false, "only export managed workloads.")
	WkldExportCmd.Flags().BoolVarP(&unmanagedOnly, "unmanaged-only", "u", false, "only export unmanaged workloads.")
	WkldExportCmd.Flags().BoolVarP(&onlineOnly, "online-only", "o", false, "only export online workloads.")
	WkldExportCmd.Flags().BoolVarP(&includeVuln, "incude-vuln-data", "v", false, "include vulnerability data.")
	WkldExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	WkldExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	WkldExportCmd.Flags().BoolVar(&removeDescNewLines, "remove-desc-newline", false, "will remove new line characters in description field.")

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
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		exportWorkloads()
	},
}

func exportWorkloads() {

	// Log command execution
	utils.LogStartCommand("wkld-export")

	// GetAllWorkloads
	qp := make(map[string]string)
	if unmanagedOnly {
		qp["managed"] = "false"
	}
	if managedOnly {
		qp["managed"] = "true"
	}
	if includeVuln {
		qp["representation"] = "workload_labels_vulnerabilities"
	}
	if onlineOnly {
		qp["online"] = "true"
	}
	wklds, a, err := pce.GetWklds(qp)
	utils.LogAPIResp("GetWklds", a)
	if err != nil {
		utils.LogError(fmt.Sprintf("getting all workloads - %s", err))
	}

	// Get the labels that are in use by the workloads
	labelsKeyMap := make(map[string]bool)
	for _, w := range wklds {
		for _, label := range *w.Labels {
			labelsKeyMap[pce.Labels[label.Href].Key] = true
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
	if exportHeaders == "" {
		for _, header := range AllHeaders(includeVuln, !noHref) {
			headerRow = append(headerRow, header)
			// Insert the labels either after href or hostname
			if (!noHref && header == "href") || (noHref && header == "name") {
				headerRow = append(headerRow, labelsKeySlice...)
			}
		}
		outputData = append(outputData, headerRow)
	} else {
		outputData = append(outputData, strings.Split(strings.Replace(exportHeaders, " ", "", -1), ","))
	}

	// Iterate through each workload
	for _, w := range wklds {
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
				csvRow[HeaderActivePceFqdn] = pce.FQDN
			}
			csvRow[HeaderAgentStatus] = w.Agent.Status.Status
			csvRow[HeaderCloudInstanceID] = w.Agent.Status.InstanceID
			if csvRow[HeaderCloudInstanceID] == "" {
				csvRow[HeaderCloudInstanceID] = "NA"
			}
			if w.Agent.Status.AgentHealth != nil && len(w.Agent.Status.AgentHealth) > 0 {
				healthSlice := []string{}
				for _, a := range w.Agent.Status.AgentHealth {
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
		if removeDescNewLines && w.Description != nil {
			*w.Description = utils.ReplaceNewLine(*w.Description)
		}

		// Get the labels
		for _, labelKey := range labelsKeySlice {
			csvRow[labelKey] = w.GetLabelByKey(labelKey, pce.Labels).Value
		}

		// Fill csv row with other data
		csvRow[HeaderHostname] = w.Hostname
		csvRow[HeaderName] = w.Name
		csvRow[HeaderHref] = w.Href

		csvRow[HeaderPublicIP] = w.PublicIP
		csvRow[HeaderDistinguishedName] = utils.PtrToStr(w.DistinguishedName)
		csvRow[HeaderIPWithDefaultGw] = w.GetIPWithDefaultGW()
		csvRow[HeaderNetmaskOfIPWithDefGw] = w.GetNetMaskWithDefaultGW()
		csvRow[HeaderDefaultGw] = w.GetDefaultGW()
		csvRow[HeaderDefaultGwNetwork] = w.GetNetworkWithDefaultGateway()
		csvRow[HeaderSPN] = utils.PtrToStr(w.ServicePrincipalName)
		csvRow[HeaderDescription] = utils.PtrToStr(w.Description)
		csvRow[HeaderEnforcement] = w.GetMode()
		csvRow[HeaderVisibility] = w.GetVisibilityLevel()
		csvRow[HeaderOnline] = strconv.FormatBool(w.Online)
		csvRow[HeaderCreatedAt] = w.CreatedAt
		csvRow[HeaderOsID] = utils.PtrToStr(w.OsID)
		csvRow[HeaderOsDetail] = utils.PtrToStr(w.OsDetail)
		csvRow[HeaderServiceProvider] = w.ServiceProvider
		csvRow[HeaderDataCenter] = utils.PtrToStr(w.DataCenter)
		csvRow[HeaderDataCenterZone] = w.DataCenterZone
		csvRow[HeaderExternalDataReference] = utils.PtrToStr(w.ExternalDataReference)
		csvRow[HeaderExternalDataSet] = utils.PtrToStr(w.ExternalDataSet)

		if includeVuln {
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

	if len(outputData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-wkld-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(outputData, outputData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d workloads exported", len(outputData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no workloads in PCE.", true)
	}

	utils.LogEndCommand("wkld-export")

}

func InterfaceToString(w illumioapi.Workload, replaceDots bool) (interfaces []string) {
	for _, i := range w.Interfaces {
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
