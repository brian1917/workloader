package wkldexport

import (
	"fmt"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
)

// WkldExport is used to export workloads
type WkldExport struct {
	PCE                 *illumioapi.PCE
	IncludeLabelSummary bool
	LabelSummaryKeys    string
	IncludeVuln         bool
	RemoveDescNewLines  bool
	Headers             []string
	LabelPrefix         bool
}

// CsvData returns wkld export in a csv format of slice of slice of strings
func (e *WkldExport) CsvData() (csvData [][]string) {

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

	// Check if need to override the label prefix
	for _, ld := range labelsKeySlice {
		if _, ok := FieldMapping()[ld]; ok {
			utils.LogWarningf(true, "%s label key matches a header or alias used in wkld-export or wkld-import. switching on the add-label-prefix flag to add \"label:\" in front of each label key in the csv headers. this format is compatible in wkld-import.", ld)
			e.LabelPrefix = true
		}
	}

	// Process label summary
	if e.IncludeLabelSummary {
		var uniqueLabelKeysSlice []string
		if e.LabelSummaryKeys == "" {
			uniqueLabelKeysSlice = labelsKeySlice
		} else {
			uniqueLabelKeys = strings.Replace(e.LabelSummaryKeys, ", ", ",", -1)
			uniqueLabelKeysSlice = strings.Split(e.LabelSummaryKeys, ",")
		}
		utils.LogInfof(false, "unique label keys: %s", strings.Join(uniqueLabelKeysSlice, ","))
		includeHeaderRow := append(uniqueLabelKeysSlice, "count")
		includeCsvData := [][]string{includeHeaderRow}
		labelSummaryMap := make(map[string]int)
		for _, w := range e.PCE.WorkloadsSlice {
			// Check the interfaces
			if !validateSubnet(w, subnetInclude) {
				continue
			}
			wkldLabelValueSlice := []string{}
			for _, l := range uniqueLabelKeysSlice {
				label := w.GetLabelByKey(l, e.PCE.Labels)
				if label.Value == "" {
					label.Value = fmt.Sprintf("empty_%s", l)
				}
				wkldLabelValueSlice = append(wkldLabelValueSlice, label.Value)
			}
			labelSummaryMap[strings.Join(wkldLabelValueSlice, ";")] = labelSummaryMap[strings.Join(wkldLabelValueSlice, ";")] + 1
		}
		for uniqueLabels, count := range labelSummaryMap {
			row := append(strings.Split(uniqueLabels, ";"), strconv.Itoa(count))
			includeCsvData = append(includeCsvData, row)
		}
		if len(includeCsvData) > 1 {
			if globalOutputFileName == "" {
				globalOutputFileName = fmt.Sprintf("workloader-wkld-export-unique-labels-%s.csv", time.Now().Format("20060102_150405"))
			} else {
				if strings.HasSuffix(globalOutputFileName, ".csv") {
					globalOutputFileName = fmt.Sprintf("%s-%s.csv", strings.TrimSuffix(globalOutputFileName, ".csv"), "unique-labels")
				} else {
					globalOutputFileName = fmt.Sprintf("%s-unique-labels.csv", globalOutputFileName)
				}
			}
			utils.WriteOutput(includeCsvData, nil, globalOutputFileName)
			utils.LogInfo(fmt.Sprintf("%d unique label combinations exported", len(includeCsvData)-1), true)
		} else {
			// Log command execution for 0 results
			utils.LogInfo("no workloads in PCE.", true)
		}
	}

	// Start the outputdata
	headerRow := []string{}
	// If no user headers provided, get all the headers
	if len(e.Headers) == 0 {
		for _, header := range AllHeaders(e.IncludeVuln, !noHref) {
			headerRow = append(headerRow, header)
			// Insert the labels either after href or hostname
			if (!noHref && header == "href") || (noHref && header == "name") {
				if e.LabelPrefix {
					for _, key := range labelsKeySlice {
						headerRow = append(headerRow, fmt.Sprintf("label:%s", key))
					}
				} else {
					headerRow = append(headerRow, labelsKeySlice...)
				}
			}
		}
		csvData = append(csvData, headerRow)
	} else {
		csvData = append(csvData, e.Headers)
	}

	// Iterate through each workload
	for _, w := range e.PCE.WorkloadsSlice {
		csvRow := make(map[string]string)
		// Skip deleted workloads
		if *w.Deleted {
			continue
		}

		// Check the interfaces
		if !validateSubnet(w, subnetInclude) {
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
			if e.LabelPrefix {
				csvRow[fmt.Sprintf("label:%s", labelKey)] = w.GetLabelByKey(labelKey, e.PCE.Labels).Value
			} else {
				csvRow[labelKey] = w.GetLabelByKey(labelKey, e.PCE.Labels).Value
			}
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
		if w.RiskSummary != nil {
			csvRow[HeaderRansomewareExposure] = w.RiskSummary.Ransomware.WorkloadExposureSeverity
			csvRow[HeaderProtectionCoverageScore] = fmt.Sprintf("%f%%", w.RiskSummary.Ransomware.RansomwareProtectionPercent)
		}

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
		for _, header := range csvData[0] {
			newRow = append(newRow, csvRow[header])
		}
		csvData = append(csvData, newRow)
	}
	return csvData
}

// MapData returns a map where they key is the first header's value and the value is another map for each column.
// For example, if the first header is "hostname" and you need to find the interfaces, use csvDataMap["wkld_host_name"][interfaces]
func (e *WkldExport) MapData() (csvDataMap map[string]map[string]string) {
	csvData := e.CsvData()
	csvDataMap = make(map[string]map[string]string)
	headers := []string{}
	for rowIndex, row := range csvData {
		if rowIndex == 0 {
			// Populate the headers slice
			headers = append(headers, row...)
			continue
		}
		csvDataMap[row[0]] = make(map[string]string)
		for colIndex, col := range row {
			csvDataMap[row[0]][headers[colIndex]] = col
		}
	}
	return csvDataMap
}

// WriteToCsv epxorts a PCE workloads to a CSV
func (e *WkldExport) WriteToCsv(outputFile string) {

	// Get the csvData
	outputData := e.CsvData()

	if len(outputData) > 1 {
		if outputFile == "" {
			outputFile = utils.FileName("")
		}
		utils.WriteOutput(outputData, outputData, outputFile)
		utils.LogInfo(fmt.Sprintf("%d workloads exported", len(outputData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no workloads in PCE.", true)
	}

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

func validateSubnet(w illumioapi.Workload, subnetInclude string) (nicInSubnet bool) {

	if subnetInclude != "" {
		subnets := strings.Split(subnetInclude, ",")

		// Validate the subnet
		for _, s := range subnets {
			_, network, err := net.ParseCIDR(s)
			if err != nil {
				utils.LogErrorf("invalid subnet cidr - %s", s)
			}
			for _, nic := range illumioapi.PtrToVal(w.Interfaces) {
				ip := net.ParseIP(nic.Address)
				if network.Contains(ip) {
					nicInSubnet = true
				}
			}
		}
		return nicInSubnet
	}
	return true

}
