package wkldimport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/workloader/cmd/wkldexport"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/viper"
)

// checkLabels validates if a label exists.
// If the label exists it returns the label.
// If the label does not exist it creates a temporary label for later
func checkLabel(pce illumioapi.PCE, label illumioapi.Label, newLabels []illumioapi.Label) (illumioapi.Label, []illumioapi.Label) {

	// Check if it exists or not
	if _, ok := pce.Labels[label.Key+label.Value]; ok {
		return pce.Labels[label.Key+label.Value], newLabels
	}

	// If the label doesn't exist, create a placeholder for it
	label.Href = fmt.Sprintf("wkld-import-temp-%s-%s", label.Key, label.Value)
	newLabels = append(newLabels, label)

	// Append the label back to the map
	pce.Labels[label.Key+label.Value] = label
	pce.Labels[label.Href] = label

	return label, newLabels
}

// ImportWkldsFromCSV imports a CSV to label unmanaged workloads and create unmanaged workloads
func ImportWkldsFromCSV(input Input) {

	// Log start of the command
	utils.LogStartCommand("wkld-import")

	// Create a newLabels slice
	var newLabels []illumioapi.Label

	// Parse the CSV File
	data, err := utils.ParseCSV(input.ImportFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Process the headers
	input.processHeaders(data[0])

	// Log intput
	input.log()

	// Check if we have the workload map populate
	if input.PCE.Workloads == nil || len(input.PCE.WorkloadsSlice) == 0 {
		apiResps, err := input.PCE.Load(illumioapi.LoadInput{Workloads: true})
		utils.LogMultiAPIResp(apiResps)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Check if we have the labels maps
	if input.PCE.Labels == nil || len(input.PCE.Labels) == 0 {
		apiResps, err := input.PCE.Load(illumioapi.LoadInput{Labels: true})
		utils.LogMultiAPIResp(apiResps)
		if err != nil {
			utils.LogError(err.Error())
		}
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
				input.PCE.Workloads[w.Hostname] = w
				input.PCE.Workloads[w.Name] = w
			}
		}
	}

	// Create a map of label keys
	labelKeysMap := make(map[string]bool)
	for _, l := range input.PCE.LabelsSlice {
		labelKeysMap[l.Key] = true
	}

	// Create slices to hold the workloads we will update and create
	updatedWklds := []illumioapi.Workload{}
	newUMWLs := []illumioapi.Workload{}

	// Start the counters
	unchangedWLs := 0

	// Iterate through CSV entries
CSVEntries:
	for i, line := range data {

		// Increment the counter
		csvLine := i + 1

		// Skip the first row
		if csvLine == 1 {
			continue
		}

		// Process the prefixes to labels
		prefixes := []string{input.RolePrefix, input.AppPrefix, input.EnvPrefix, input.LocPrefix}
		for i, header := range []string{wkldexport.HeaderRole, wkldexport.HeaderApp, wkldexport.HeaderEnv, wkldexport.HeaderLoc} {
			if index, ok := input.Headers[header]; ok {
				// If the value is blank, do nothing
				line[index] = prefixes[i] + line[index]
			}
		}

		// Check if we are matching on href or hostname
		if input.MatchString == "href" && input.Umwl {
			utils.LogError("cannot match on hrefs and create unmanaged workloads")
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
			newWorkloads := make(map[string]illumioapi.Workload)
			for k, w := range input.PCE.Workloads {
				newWorkloads[strings.ToLower(k)] = w
			}
			input.PCE.Workloads = newWorkloads
			compareString = strings.ToLower(compareString)
		}

		// If the workloads does not exist and we are set to create unmanaged workloads, create it
		if _, ok := input.PCE.Workloads[compareString]; !ok && input.Umwl {

			// Create the workload
			newWkld := illumioapi.Workload{Hostname: line[input.Headers[wkldexport.HeaderHostname]], Labels: &[]*illumioapi.Label{}}

			// Process if interface is in import and if interface entry has values
			if index, ok := input.Headers[wkldexport.HeaderInterfaces]; ok && len(line[index]) > 0 {
				// Create the network interfaces
				nics := strings.Split(strings.Replace(line[index], " ", "", -1), ";")
				for _, n := range nics {
					ipInterface, err := userInputConvert(n)
					if err != nil {
						utils.LogError(err.Error())
					}
					newWkld.Interfaces = append(newWkld.Interfaces, &ipInterface)
				}
			} else if spnIndex, ok := input.Headers[wkldexport.HeaderSPN]; !ok || line[spnIndex] == "" {
				utils.LogWarning(fmt.Sprintf("csv line %d - no interface and no spn provided for %s.", csvLine, compareString), true)
			}

			// Process the labels
			// Iterate over all headers and process the ones that are label keys
			for headerValue, index := range input.Headers {
				// Check if the header is a label key
				if labelKeysMap[headerValue] {
					// If the value is blank, do nothing
					if line[index] == "" || line[index] == input.RemoveValue {
						continue
					}
					// Add the label to the new labels slice
					if newWkld.Labels == nil {
						newWkld.Labels = &[]*illumioapi.Label{}
					}
					var retrievedLabel illumioapi.Label
					retrievedLabel, newLabels = checkLabel(input.PCE, illumioapi.Label{Key: headerValue, Value: line[index]}, newLabels)
					*newWkld.Labels = append(*newWkld.Labels, &illumioapi.Label{Href: retrievedLabel.Href})
				}
			}

			// Proces the name
			if index, ok := input.Headers[wkldexport.HeaderName]; ok {
				newWkld.Name = line[index]
				if index, ok := input.Headers[wkldexport.HeaderHostname]; ok && newWkld.Name == "" {
					newWkld.Name = line[index]
				}
			}

			// Process the Public IP
			if index, ok := input.Headers[wkldexport.HeaderPublicIP]; ok {
				if !publicIPIsValid(line[index]) {
					utils.LogError(fmt.Sprintf("csv line %d - invalid public ip address format.", csvLine))
				}
				newWkld.PublicIP = line[index]
			}

			// Process the SPN
			if index, ok := input.Headers[wkldexport.HeaderSPN]; ok {
				newWkld.ServicePrincipalName = line[index]
			}

			// Process the enforcement state
			if index, ok := input.Headers[wkldexport.HeaderPolicyState]; ok && strings.ToLower(line[index]) != "unmanaged" {
				m := strings.ToLower(line[index])
				if m != "visibility_only" && m != "full" && m != "selective" && m != "idle" && m != "" {
					utils.LogWarning(fmt.Sprintf("csv line %d - invalid mode state for %s. values must be blank, visibility_only, full, selective, or idle. skipping line.", csvLine, compareString), true)
					continue CSVEntries
				}
				newWkld.EnforcementMode = m
			}

			// Process the visibility state
			if index, ok := input.Headers[wkldexport.HeaderVisibilityState]; ok && strings.ToLower(line[index]) != "unmanaged" {
				v := strings.ToLower(line[index])
				if v != "blocked_allowed" && v != "blocked" && v != "off" && v != "" {
					utils.LogWarning(fmt.Sprintf("csv line %d - invalid visibility state for %s. values must be blank, blocked_allowed, blocked, or off. skipping line.", csvLine, compareString), true)
					continue CSVEntries
				}
				newWkld.VisibilityLevel = line[index]
			}

			// Process string variables requiring no logic check.
			varPtrs := []*string{&newWkld.Description, &newWkld.ExternalDataReference, &newWkld.ExternalDataSet, &newWkld.OsID, &newWkld.OsDetail, &newWkld.DataCenter, &newWkld.DistinguishedName}
			headers := []string{wkldexport.HeaderDescription, wkldexport.HeaderExternalDataReference, wkldexport.HeaderExternalDataSet, wkldexport.HeaderOsID, wkldexport.HeaderOsDetail, wkldexport.HeaderDataCenter, wkldexport.HeaderMachineAuthenticationID}
			for i, varPtr := range varPtrs {
				if _, ok := input.Headers[headers[i]]; !ok {
					continue
				}
				*varPtr = line[input.Headers[headers[i]]]
			}

			// Append the new workload to the newUMWLs slice
			newUMWLs = append(newUMWLs, newWkld)

			// Log the entry
			x := []string{}
			for _, i := range newWkld.Interfaces {
				if i.CidrBlock != nil {
					x = append(x, i.Name+":"+i.Address+"/"+strconv.Itoa(*i.CidrBlock))
				} else {
					x = append(x, i.Name+":"+i.Address)
				}
			}
			utils.LogInfo(fmt.Sprintf("csv line %d - %s to be created - %s (role), %s (app), %s (env), %s(loc) - interfaces: %s", csvLine, newWkld.Hostname, newWkld.GetRole(input.PCE.Labels).Value, newWkld.GetApp(input.PCE.Labels).Value, newWkld.GetEnv(input.PCE.Labels).Value, newWkld.GetLoc(input.PCE.Labels).Value, strings.Join(x, ";")), false)
			continue
		} else if !ok && !input.Umwl {
			// If the workload does not exist and umwl flag is not set, log the entry
			utils.LogInfo(fmt.Sprintf("csv line %d - %s is not a workload. Include umwl flag to create it. Nothing done.", csvLine, compareString), false)
			continue
		}

		// *******************************************
		// *** Get here if the workload does exist ***
		// *******************************************

		if input.UpdateWorkloads {
			// Initialize the change variable
			change := false

			// Need this since can't perform pointer method on map element
			wkld := input.PCE.Workloads[compareString]

			// Make a copy of the original workload for comparing labels and then clear labels for rebuilding
			oldWkld := wkld
			wkld.Labels = nil
			wkld.Labels = &[]*illumioapi.Label{}

			// Process all headers to see if they are labels
			for headerValue, index := range input.Headers {

				// Skip if the header is not a label
				if !labelKeysMap[headerValue] {
					continue
				}

				// Get the current label
				currentLabel := oldWkld.GetLabelByKey(headerValue, input.PCE.Labels)

				// If the value is blank and the current label exists, keep the current label.
				// Or, if the CSV entry equals the current value, keep it
				if (line[index] == "" && currentLabel.Href != "") || (line[index] == currentLabel.Value) {
					*wkld.Labels = append(*wkld.Labels, &illumioapi.Label{Href: currentLabel.Href})
					continue
				}

				// If the value is the delete value, the value is not blank, and the current label is not already blank, log a change without putting any label in.
				if line[index] == input.RemoveValue && line[index] != "" && currentLabel.Href != "" {
					change = true
					utils.LogInfo(fmt.Sprintf("csv line %d - %s requiring removal of %s label.", csvLine, compareString, currentLabel.Key), false)
					continue
				}

				// If the value does not equal the current value and it does not equal the remove value, add the new label.
				if line[index] != currentLabel.Value && line[index] != input.RemoveValue {
					change = true
					// Log change required
					currentlLabelLogValue := currentLabel.Value
					if currentLabel.Value == "" {
						currentlLabelLogValue = "<empty>"
					}
					utils.LogInfo(fmt.Sprintf("csv line %d - %s %s label to be changed from %s to %s.", csvLine, compareString, headerValue, currentlLabelLogValue, line[index]), false)
					// Add that label to the new labels slice]
					var retrievedLabel illumioapi.Label
					retrievedLabel, newLabels = checkLabel(input.PCE, illumioapi.Label{Key: headerValue, Value: line[index]}, newLabels)
					*wkld.Labels = append(*wkld.Labels, &illumioapi.Label{Href: retrievedLabel.Href})
					continue
				}

			}

			// Check interfaces
			if wkld.GetMode() == "unmanaged" {
				// If IP field is there and  IP address is provided, check it out
				if index, ok := input.Headers[wkldexport.HeaderInterfaces]; ok && len(line[index]) > 0 {
					// Build out the netInterfaces slice provided by the user
					netInterfaces := []*illumioapi.Interface{}
					nics := strings.Split(strings.Replace(line[index], " ", "", -1), ";")
					for _, n := range nics {
						ipInterface, err := userInputConvert(n)
						if err != nil {
							utils.LogWarning(fmt.Sprintf("csv line %d - %s - skipping workload entry.", csvLine, err.Error()), true)
							continue CSVEntries
						}
						netInterfaces = append(netInterfaces, &ipInterface)
					}

					// If instructed by flag, make sure we keep all PCE interfaces
					if input.KeepAllPCEInterfaces {
						// Build a map of the interfaces provided by the user with the address as the key
						interfaceMap := make(map[string]illumioapi.Interface)
						for _, i := range netInterfaces {
							interfaceMap[i.Address] = *i
						}
						// For each interface on the PCE, check if the address is in the map
						for _, i := range wkld.Interfaces {
							// If it's not in them map, append it to the user provdided netInterfaces so we keep it
							if _, ok := interfaceMap[i.Address]; !ok {
								netInterfaces = append(netInterfaces, i)
							}
						}
					}

					// Build some maps
					userMap := make(map[string]bool)
					wkldIntMap := make(map[string]bool)
					for _, w := range wkld.Interfaces {
						cidrText := "nil"
						if w.CidrBlock != nil {
							cidrText = strconv.Itoa(*w.CidrBlock)
						}
						wkldIntMap[w.Address+cidrText+w.Name] = true
					}
					for _, u := range netInterfaces {
						cidrText := "nil"
						if u.CidrBlock != nil {
							cidrText = strconv.Itoa(*u.CidrBlock)
						}
						userMap[u.Address+cidrText+u.Name] = true
					}

					updateInterfaces := false
					// Are all workload interfaces in spreadsheet?
					for _, w := range wkld.Interfaces {
						cidrText := "nil"
						if w.CidrBlock != nil && *w.CidrBlock != 0 {
							cidrText = strconv.Itoa(*w.CidrBlock)
						}
						if !userMap[w.Address+cidrText+w.Name] {
							updateInterfaces = true
							change = true
							utils.LogInfo(fmt.Sprintf("csv line %d - interface not in user provided data - ip: %s, cidr: %s, name: %s", csvLine, w.Address, cidrText, w.Name), false)
						}
					}

					// Are all user interfaces on workload?
					for _, u := range netInterfaces {
						cidrText := "nil"
						if u.CidrBlock != nil && *u.CidrBlock != 0 {
							cidrText = strconv.Itoa(*u.CidrBlock)
						}
						if !wkldIntMap[u.Address+cidrText+u.Name] {
							updateInterfaces = true
							change = true
							utils.LogInfo(fmt.Sprintf("csv line %d - user provided interface not in workload - ip: %s, cidr: %s, name: %s", csvLine, u.Address, cidrText, u.Name), false)
						}
					}

					if updateInterfaces {
						wkld.Interfaces = netInterfaces
					}
				}
				// Update the hostname if field provided and matching on Href
				if index, ok := input.Headers[wkldexport.HeaderHostname]; ok && input.MatchString != wkldexport.HeaderHostname {
					if wkld.Hostname != line[index] {
						change = true
						utils.LogInfo(fmt.Sprintf("csv line %d - hostname to be changed from %s to %s", csvLine, wkld.Hostname, line[index]), false)
						wkld.Hostname = line[index]
					}
				}

				// Update the SPN
				if index, ok := input.Headers[wkldexport.HeaderSPN]; ok {
					if wkld.ServicePrincipalName != line[index] {
						change = true
						utils.LogInfo(fmt.Sprintf("csv line %d - spn to be changed from %s to %s", csvLine, wkld.ServicePrincipalName, line[index]), false)
						wkld.ServicePrincipalName = line[index]
					}
				}

			}

			if input.AllowEnforcementChanges {
				// Update the enforcement
				if index, ok := input.Headers[wkldexport.HeaderPolicyState]; ok && strings.ToLower(line[index]) != "unmanaged" && line[index] != "" {
					m := strings.ToLower(line[index])
					if m != "visibility_only" && m != "full" && m != "selective" && m != "idle" && m != "" {
						utils.LogWarning(fmt.Sprintf("csv line %d - invalid mode state for unmanaged workload %s. values must be blank, visibility_only, full, selective, or idle. skipping line.", csvLine, compareString), true)
						continue CSVEntries
					}
					if wkld.EnforcementMode != m {
						change = true
						utils.LogInfo(fmt.Sprintf("csv line %d - %s enforcement to be changed from %s to %s", csvLine, wkld.Hostname, wkld.EnforcementMode, line[index]), false)
						wkld.EnforcementMode = m
					}
				}

				// Update the visibility
				if index, ok := input.Headers[wkldexport.HeaderVisibilityState]; ok && strings.ToLower(line[index]) != "unmanaged" && line[index] != "" {
					v := strings.ToLower(line[index])
					if v != "blocked_allowed" && v != "blocked" && v != "off" && v != "" {
						utils.LogWarning(fmt.Sprintf("csv line %d - invalid visibility state for unmanaged workload %s. values must be blank, blocked_allowed, blocked, or off. skipping line.", csvLine, compareString), true)
						continue CSVEntries
					}
					if wkld.GetVisibilityLevel() != v {
						change = true
						utils.LogInfo(fmt.Sprintf("csv line %d - %s visibility to be changed from %s to %s", csvLine, wkld.Hostname, wkld.VisibilityLevel, line[index]), false)
						wkld.SetVisibilityLevel(v)
					}
				}
			}

			// Change the name if the name field is provided it doesn't match unless the name in the CSV is blank and PCE is reporting the name as the hostname
			if index, ok := input.Headers[wkldexport.HeaderName]; ok && wkld.Name != line[index] && line[index] != "" {
				change = true
				utils.LogInfo(fmt.Sprintf("csv line %d - Name to be changed from %s to %s", csvLine, wkld.Name, line[index]), false)
				wkld.Name = line[index]
			}

			// Update the Public Ip
			if index, ok := input.Headers[wkldexport.HeaderPublicIP]; ok {
				if line[index] != wkld.PublicIP {
					// Validate it first
					if !publicIPIsValid(line[index]) {
						utils.LogError(fmt.Sprintf("csv line %d - invalid Public IP address format.", csvLine))
					}
					change = true
					utils.LogInfo(fmt.Sprintf("csv line %d - Public IP to be changed from %s to %s", csvLine, wkld.PublicIP, line[index]), false)
					wkld.PublicIP = line[index]
				}
			}

			// Update strings that don't need any manipulation
			headerValues := []string{wkldexport.HeaderDescription, wkldexport.HeaderMachineAuthenticationID, wkldexport.HeaderExternalDataSet, wkldexport.HeaderExternalDataReference, wkldexport.HeaderOsID, wkldexport.HeaderOsDetail, wkldexport.HeaderDataCenter}
			targetUpdates := []*string{&wkld.Description, &wkld.DistinguishedName, &wkld.ExternalDataSet, &wkld.ExternalDataReference, &wkld.OsID, &wkld.OsDetail, &wkld.DataCenter}
			for i, header := range headerValues {
				if index, ok := input.Headers[header]; ok {
					if line[index] != *targetUpdates[i] {
						utils.LogInfo(fmt.Sprintf("csv line %d - %s to be changed from %s to %s", csvLine, header, *targetUpdates[i], line[index]), false)
						change = true
						*targetUpdates[i] = line[index]
						*targetUpdates[i] = line[index]
					}
				}
			}

			// If change was flagged, get the workload, update the labels, append to updated slice.
			if change {
				updatedWklds = append(updatedWklds, wkld)
			} else {
				unchangedWLs++
			}

		}
	}

	// End run if we have nothing to do
	if len(updatedWklds) == 0 && len(newUMWLs) == 0 {
		utils.LogInfo("nothing to be done", true)
		utils.LogEndCommand("wkld-import")
		return
	}

	// Log findings
	utils.LogInfo(fmt.Sprintf("workloader identified %d labels to create.", len(newLabels)), true)
	utils.LogInfo(fmt.Sprintf("workloader identified %d workloads requiring updates.", len(updatedWklds)), true)
	utils.LogInfo(fmt.Sprintf("workloader identified %d unmanaged workloads to create.", len(newUMWLs)), true)
	utils.LogInfo(fmt.Sprintf("%d entries in CSV require no changes", unchangedWLs), true)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !input.UpdatePCE {
		utils.LogInfo("See workloader.log for more details. To do the import, run again using --update-pce flag.", true)
		utils.LogEndCommand("wkld-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if input.UpdatePCE && !input.NoPrompt {
		var prompt string
		fmt.Printf("\r\n%s [PROMPT] - Do you want to run the import to %s (%s) (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), input.PCE.FriendlyName, viper.Get(input.PCE.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied", true)
			utils.LogEndCommand("wkld-import")
			return
		}
	}

	// We will only get here if updatePCE and noPrompt is set OR the user accepted the prompt

	// Process the labels first
	labelReplacementMap := make(map[string]string)
	if len(newLabels) > 0 {
		for _, label := range newLabels {
			createdLabel, api, err := input.PCE.CreateLabel(illumioapi.Label{Key: label.Key, Value: label.Value})
			utils.LogAPIResp("CreateLabel", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			labelReplacementMap[label.Href] = createdLabel.Href
			utils.LogInfo(fmt.Sprintf("created new %s label - %s - %d", createdLabel.Key, createdLabel.Value, api.StatusCode), true)
		}
	}

	// Replace the labels that need to
	for i, wkld := range updatedWklds {
		newLabels := []*illumioapi.Label{}
		if wkld.Labels != nil {
			for _, l := range *wkld.Labels {
				if strings.Contains(l.Href, "wkld-import-temp") {
					newLabels = append(newLabels, &illumioapi.Label{Href: labelReplacementMap[l.Href]})
				} else {
					newLabels = append(newLabels, &illumioapi.Label{Href: l.Href})
				}
			}
			wkld.Labels = &newLabels
		}
		updatedWklds[i] = wkld
	}

	for i, wkld := range newUMWLs {
		newLabels := []*illumioapi.Label{}
		for _, l := range *wkld.Labels {
			if strings.Contains(l.Href, "wkld-import-temp") {
				newLabels = append(newLabels, &illumioapi.Label{Href: labelReplacementMap[l.Href]})
			} else {
				newLabels = append(newLabels, &illumioapi.Label{Href: l.Href})
			}
		}
		wkld.Labels = &newLabels
		newUMWLs[i] = wkld
	}

	if len(updatedWklds) > 0 {
		api, err := input.PCE.BulkWorkload(updatedWklds, "update", true)
		for _, a := range api {
			utils.LogAPIResp("BulkWorkloadUpdate", a)
		}
		if err != nil {
			utils.LogError(fmt.Sprintf("bulk updating workloads - %s", err))
		}
		utils.LogInfo(fmt.Sprintf("bulk update workload successful for %d workloads - status code %d", len(updatedWklds), api[0].StatusCode), true)
	}

	// Bulk create if we have new workloads
	if len(newUMWLs) > 0 {
		api, err := input.PCE.BulkWorkload(newUMWLs, "create", true)
		for _, a := range api {
			utils.LogAPIResp("BulkWorkloadCreate", a)

		}
		if err != nil {
			utils.LogError(fmt.Sprintf("bulk creating workloads - %s", err))
		}
		utils.LogInfo(fmt.Sprintf("bulk create workload successful for %d unmanaged workloads - status code %d", len(newUMWLs), api[0].StatusCode), true)
	}

	// Log end
	utils.LogEndCommand("wkld-import")
}
