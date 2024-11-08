package wkldimport

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/workloader/cmd/wkldexport"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/viper"
)

// ImportWkldsFromCSV imports a CSV to label unmanaged workloads and create unmanaged workloads
func ImportWkldsFromCSV(input Input) {

	// Create a newLabels slice
	var newLabels []illumioapi.Label

	// Parse the CSV File
	data := input.ImportData
	var err error
	if len(data) == 0 {
		data, err = utils.ParseCSV(input.ImportFile)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Process the headers and log in the input
	input.ProcessHeaders(data[0])
	input.log()

	// Get the PCE version
	version, api, err := input.PCE.GetVersion()
	utils.LogAPIRespV2("GetVersion", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Check if need workloads, labels, and label dimensions
	var needWklds, needLabels, needLabelDimensions bool
	if input.PCE.Workloads == nil || len(input.PCE.WorkloadsSlice) == 0 {
		needWklds = true
	}
	if input.PCE.Labels == nil || len(input.PCE.Labels) == 0 {
		needLabels = true
	}
	if (version.Major > 22 || (version.Major == 22 && version.Minor >= 5)) && len(input.PCE.LabelDimensionsSlice) == 0 {
		needLabelDimensions = true
	}

	apiResps, err := input.PCE.Load(illumioapi.LoadInput{Workloads: needWklds, Labels: needLabels, LabelDimensions: needLabelDimensions}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Check for invalid flag combinations
	if input.Umwl && (input.ManagedOnly || input.UnmanagedOnly) {
		utils.LogError("--umwl cannot be used with --managed-only or --unmanaged-ony")
	}

	// If we only want to look at unmanaged or managed rebuild our workload map.
	if input.UnmanagedOnly || input.ManagedOnly {
		input.PCE.Workloads = nil
		input.PCE.Workloads = make(map[string]illumioapi.Workload)
		for _, w := range input.PCE.WorkloadsSlice {
			if (w.GetMode() == "unmanaged" && input.UnmanagedOnly) || (w.GetMode() != "managed" && input.ManagedOnly) {
				input.PCE.Workloads[w.Href] = w
				input.PCE.Workloads[illumioapi.PtrToVal(w.Hostname)] = w
				input.PCE.Workloads[illumioapi.PtrToVal(w.Name)] = w
			}
		}
	}

	// Update workload hostnames with short name if needed
	if input.FqdnToShort {
		newWkldSlice := []illumioapi.Workload{}
		input.PCE.Workloads = nil
		input.PCE.Workloads = make(map[string]illumioapi.Workload)
		for _, w := range input.PCE.WorkloadsSlice {
			w.Hostname = illumioapi.Ptr(strings.Split(illumioapi.PtrToVal(w.Hostname), ".")[0])
			newWkldSlice = append(newWkldSlice, w)
			input.PCE.Workloads[w.Href] = w
			if illumioapi.PtrToVal(w.Hostname) != "" {
				input.PCE.Workloads[*w.Hostname] = w
			}
			if illumioapi.PtrToVal(w.Name) != "" {
				input.PCE.Workloads[*w.Name] = w
			}
			if illumioapi.PtrToVal(w.ExternalDataReference) != "" && illumioapi.PtrToVal(w.ExternalDataSet) != "" {
				input.PCE.Workloads[*w.ExternalDataSet+*w.ExternalDataReference] = w
			}
		}
		input.PCE.WorkloadsSlice = newWkldSlice
	}

	// Create a map of label keys and depending on version either populate with API or with role, app, env, and loc.
	labelKeysMap := make(map[string]bool)
	if version.Major > 22 || (version.Major == 22 && version.Minor >= 5) {
		for _, l := range input.PCE.LabelDimensionsSlice {
			labelKeysMap[l.Key] = true
		}
	} else {
		labelKeysMap["role"] = true
		labelKeysMap["app"] = true
		labelKeysMap["env"] = true
		labelKeysMap["loc"] = true
	}
	utils.LogInfo(fmt.Sprintf("label keys map: %v", labelKeysMap), false)

	// Create slices to hold the workloads we will update and create
	updatedWklds := []illumioapi.Workload{}
	newUMWLs := []illumioapi.Workload{}

	// Check if we are matching on href or hostname
	if input.MatchString == "href" && input.Umwl {
		utils.LogError("cannot match on hrefs and create unmanaged workloads")
	}

	// Case sensitivity

	if input.IgnoreCase {
		newWorkloads := make(map[string]illumioapi.Workload)
		for k, w := range input.PCE.Workloads {
			newWorkloads[strings.ToLower(k)] = w
		}
		input.PCE.Workloads = nil
		input.PCE.Workloads = newWorkloads
	}

	// Iterate through CSV entries
	for i, line := range data {

		// Increment the counter
		csvLine := i + 1

		// Skip the first row
		if csvLine == 1 {
			continue
		}

		// SHOULD BE REMOVED WHEN PREFIX FLAGS ARE REMOVED - Process the prefixes to labels
		prefixes := []string{input.RolePrefix, input.AppPrefix, input.EnvPrefix, input.LocPrefix}
		for i, header := range []string{"role", "app", "env", "loc"} {
			if index, ok := input.Headers[header]; ok {
				// If the value is blank, do nothing
				line[index] = prefixes[i] + line[index]
			}
		}

		// Check to make sure we have an entry in the match column
		if line[input.Headers[input.MatchString]] == "" {
			utils.LogWarning(fmt.Sprintf("csv line %d - the match column cannot be blank.", csvLine), true)
			continue
		}

		// Set the compare string
		compareString := line[input.Headers[input.MatchString]]
		if input.MatchString == "external_data" {
			compareString = line[input.Headers[wkldexport.HeaderExternalDataSet]] + line[input.Headers[wkldexport.HeaderExternalDataReference]]
		}

		// Case sensitity
		if input.IgnoreCase {
			compareString = strings.ToLower(compareString)
		}

		// Create the target
		w := importWkld{
			compareString: compareString,
			csvLine:       line,
			csvLineNum:    csvLine,
		}

		// Check if the workload exists. If not, check if unmanaged workload is enabled
		if val, ok := input.PCE.Workloads[compareString]; !ok {
			if !input.Umwl {
				// If unmanaged workload is not enabled, log
				if !input.DoNotLogEachCSVRow {
					utils.LogInfo(fmt.Sprintf("csv line %d - %s is not a workload. include umwl flag to create it. nothing done.", csvLine, compareString), false)
				}
				continue
			} else {
				// If unmanaged workload is enabled, populate the workload with a blank workload
				w.wkld = &illumioapi.Workload{}
			}
		} else {
			w.wkld = &val
		}

		// Process fields that require logic
		w.hostname(input)
		w.name(input)
		w.interfaces(input)
		w.publcIP(input)
		w.enforcement(input)
		w.visibility(input)
		newLabels = w.labels(input, newLabels, labelKeysMap)

		// Process fields that don't require logic
		headerValues := []string{wkldexport.HeaderDescription, wkldexport.HeaderDistinguishedName, wkldexport.HeaderSPN, wkldexport.HeaderExternalDataSet, wkldexport.HeaderExternalDataReference, wkldexport.HeaderOsID, wkldexport.HeaderOsDetail, wkldexport.HeaderDataCenter}
		targetUpdates := []**string{&w.wkld.Description, &w.wkld.DistinguishedName, &w.wkld.ServicePrincipalName, &w.wkld.ExternalDataSet, &w.wkld.ExternalDataReference, &w.wkld.OsID, &w.wkld.OsDetail, &w.wkld.DataCenter}

		for i, header := range headerValues {
			if index, ok := input.Headers[header]; ok {
				if w.csvLine[index] == input.RemoveValue && targetUpdates[i] != nil && illumioapi.PtrToVal(*targetUpdates[i]) != "" {
					if w.wkld.Href != "" {
						utils.LogInfo(fmt.Sprintf("csv line %d - %s - %s to be removed", w.csvLineNum, w.compareString, header), false)
						w.change = true
					}
					**targetUpdates[i] = ""
				} else if w.csvLine[index] != illumioapi.PtrToVal(*targetUpdates[i]) && w.csvLine[index] != "" {
					// The values don't equal each other and not using the remove value
					if w.wkld.Href != "" {
						logValue := illumioapi.PtrToVal(*targetUpdates[i])
						if logValue == "" {
							logValue = "<empty>"
						}
						utils.LogInfo(fmt.Sprintf("csv line %d - %s - %s - %s to be changed from \"%s\" to \"%s\"", w.csvLineNum, illumioapi.PtrToVal(w.wkld.Hostname), w.wkld.Href, header, logValue, w.csvLine[index]), false)
						w.change = true
					}
					*targetUpdates[i] = &w.csvLine[index]
				}

			}
		}

		// Put into right slices
		if w.wkld.Href == "" && input.Umwl {
			newUMWLs = append(newUMWLs, *w.wkld)
			utils.LogInfo(fmt.Sprintf("csv line %d - %s to be created", w.csvLineNum, w.compareString), false)
		}
		if w.wkld.Href != "" && w.change && input.UpdateWorkloads {
			updatedWklds = append(updatedWklds, *w.wkld)
		}
	}

	// End run if we have nothing to do
	if len(updatedWklds) == 0 && len(newUMWLs) == 0 {
		utils.LogInfo("nothing to be done", true)

		return
	}

	// Log findings
	utils.LogInfo(fmt.Sprintf("workloader identified %d labels to create.", len(newLabels)), true)
	utils.LogInfo(fmt.Sprintf("workloader identified %d workloads requiring updates.", len(updatedWklds)), true)
	utils.LogInfo(fmt.Sprintf("workloader identified %d unmanaged workloads to create.", len(newUMWLs)), true)
	utils.LogInfo(fmt.Sprintf("%d entries in CSV require no changes", len(data)-1-len(updatedWklds)-len(newUMWLs)), true)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !input.UpdatePCE {
		utils.LogInfo("See workloader.log for more details. To do the import, run again using --update-pce flag.", true)

		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if input.UpdatePCE && !input.NoPrompt {
		var prompt string
		fmt.Printf("\r\n%s [PROMPT] - Do you want to run the import to %s (%s) (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), input.PCE.FriendlyName, viper.Get(input.PCE.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied", true)

			return
		}
	}

	// We will only get here if updatePCE and noPrompt is set OR the user accepted the prompt

	// Process the labels first
	labelReplacementMap := make(map[string]string)
	if len(newLabels) > 0 {
		for _, label := range newLabels {
			createdLabel, api, err := input.PCE.CreateLabel(illumioapi.Label{Key: label.Key, Value: label.Value})
			utils.LogAPIRespV2("CreateLabel", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			labelReplacementMap[label.Href] = createdLabel.Href
			utils.LogInfo(fmt.Sprintf("created new %s label - %s - %d", createdLabel.Key, createdLabel.Value, api.StatusCode), true)
		}
	}

	// Replace the labels that need to
	for i, wkld := range updatedWklds {
		newLabels := []illumioapi.Label{}
		if wkld.Labels != nil {
			for _, l := range *wkld.Labels {
				if strings.Contains(l.Href, "wkld-import-temp") {
					newLabels = append(newLabels, illumioapi.Label{Href: labelReplacementMap[l.Href]})
				} else {
					newLabels = append(newLabels, illumioapi.Label{Href: l.Href})
				}
			}
			wkld.Labels = &newLabels
		}
		updatedWklds[i] = wkld
	}

	for i, wkld := range newUMWLs {
		newLabels := []illumioapi.Label{}
		for _, l := range illumioapi.PtrToVal(wkld.Labels) {
			if strings.Contains(l.Href, "wkld-import-temp") {
				newLabels = append(newLabels, illumioapi.Label{Href: labelReplacementMap[l.Href]})
			} else {
				newLabels = append(newLabels, illumioapi.Label{Href: l.Href})
			}
		}
		wkld.Labels = &newLabels
		newUMWLs[i] = wkld
	}

	if len(updatedWklds) > 0 {
		// Check the maximum allowed updates
		if input.MaxUpdate != -1 && len(updatedWklds) > input.MaxUpdate {
			utils.LogErrorfCode(2, "update count for %s of %d exceeds maximum of %d. terminating run with exit code 2.", input.PCE.FQDN, len(updatedWklds), input.MaxUpdate)
		} else {
			api, err := input.PCE.BulkWorkload(updatedWklds, "update", true)
			for _, a := range api {
				utils.LogAPIRespV2("BulkWorkloadUpdate", a)
			}
			if err != nil {
				utils.LogError(fmt.Sprintf("bulk updating workloads - %s", err))
			}
			utils.LogInfo(fmt.Sprintf("bulk update workload successful for %d workloads - status code %d", len(updatedWklds), api[0].StatusCode), true)
		}
	}

	// Bulk create if we have new workloads
	if len(newUMWLs) > 0 {
		// Check the maximum allowed updates
		if input.MaxCreate != -1 && len(newUMWLs) > input.MaxCreate {
			utils.LogErrorfCode(2, "create count for %s of %d exceeds maximum of %d. terminating run with exit code 2.", input.PCE.FQDN, len(newUMWLs), input.MaxCreate)
		} else {
			api, err := input.PCE.BulkWorkload(newUMWLs, "create", true)
			for _, a := range api {
				utils.LogAPIRespV2("BulkWorkloadCreate", a)

			}
			if err != nil {
				utils.LogError(fmt.Sprintf("bulk creating workloads - %s", err))
			}
			utils.LogInfo(fmt.Sprintf("bulk create workload successful for %d unmanaged workloads - status code %d", len(newUMWLs), api[0].StatusCode), true)
		}
	}

}
