package wkldimport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/viper"
)

// createdLabels is a global variable to count created labels
var createdLabels int

// newLabels is a global variable to hold the slice of newly created labels
var newLabels []illumioapi.Label

// checkLabels validates if a label exists.
// If the label exists it returns the label.
// If the label does not exist and updatePCE is set, it creates the label.
// If the label does not exist and updatePCE is not set, it creates a placeholder label in pce map.
func checkLabel(pce illumioapi.PCE, updatePCE bool, label illumioapi.Label, csvLine int) illumioapi.Label {

	// Check if it exists or not
	if _, ok := pce.LabelMapKV[label.Key+label.Value]; ok {
		return pce.LabelMapKV[label.Key+label.Value]
	}

	// Create the label if it doesn't exist
	if updatePCE {
		l, a, err := pce.CreateLabel(illumioapi.Label{Key: label.Key, Value: label.Value})
		utils.LogAPIResp("CreateLabel", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("CSV line - %d - created label - %s (%s) - %s", csvLine, l.Value, l.Key, l.Href), true)

		// Append the label back to the map
		pce.LabelMapKV[l.Key+l.Value] = l
		pce.LabelMapH[l.Href] = l

		// Increment counter
		createdLabels++

		return l
	}

	// If updatePCE is not set, create a placeholder href for provided label, and add it back to the maps
	utils.LogInfo(fmt.Sprintf("Potential New Label - Key: %s, Value: %s", label.Key, label.Value), false)
	label.Href = fmt.Sprintf("place-holder-href-%s-%s", label.Key, label.Value)
	pce.LabelMapKV[label.Key+label.Value] = label
	pce.LabelMapH[label.Href] = label
	newLabels = append(newLabels, illumioapi.Label{Key: label.Key, Value: label.Value})

	return label
}

// ImportWkldsFromCSV imports a CSV to label unmanaged workloads and create unmanaged workloads
func ImportWkldsFromCSV(input Input) {

	// Log start of the command
	utils.LogStartCommand("wkld-import")

	// Parse the CSV File
	data, err := utils.ParseCSV(input.ImportFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Process the headers
	input.processHeaders(data[0])

	// Adjust the columns by one
	input.decreaseColBy1()

	// Log our intput
	input.log()

	// Get the workload map by href
	utils.LogInfo("Starting GET all workloads.", true)
	wkldMap, a, err := input.PCE.GetWkldHrefMap()
	utils.LogAPIResp("GetWkldHrefMap", a)
	if err != nil {
		utils.LogError(fmt.Sprintf("getting workload href map - %s", err))
	}
	utils.LogInfo(fmt.Sprintf("GET all workloads complete - %d workloads", len(wkldMap)), true)

	// Get the hostnames
	wkldHostNameMap := make(map[string]illumioapi.Workload)
	for _, w := range wkldMap {
		hostname := w.Hostname
		if input.FQDNtoHostname {
			hostname = strings.Split(w.Hostname, ".")[0]
			w.Hostname = hostname
		}
		wkldHostNameMap[hostname] = w
	}

	// Combine the maps
	for _, w := range wkldHostNameMap {
		wkldMap[w.Hostname] = w
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

		// Check if we are processing description and skip the first row
		if csvLine == 1 {
			continue
		}

		// Check if we are matching on href or hostname
		if csvLine == 2 && strings.Contains(line[input.MatchIndex], "/workloads/") && input.Umwl {
			utils.LogError("cannot match on hrefs and create unmanaged workloads")
		}

		// Check to make sure we have an entry in the match column
		if line[input.MatchIndex] == "" {
			utils.LogWarning(fmt.Sprintf("CSV line %d - the match column cannot be blank - hostname or href required.", csvLine), true)
			continue
		}

		// Check if the workload exists. If it does not exist, check if UMWL is set and take action.
		if _, ok := wkldMap[line[input.MatchIndex]]; !ok {
			var netInterfaces []*illumioapi.Interface
			if input.Umwl {
				// Process if interface is in import and if interface entry has values
				if input.IntIndex != 99998 && len(line[input.IntIndex]) > 0 {
					// Create the network interfaces

					nics := strings.Split(strings.Replace(line[input.IntIndex], " ", "", -1), ";")
					for _, n := range nics {
						ipInterface, err := userInputConvert(n)
						if err != nil {
							utils.LogError(err.Error())
						}
						netInterfaces = append(netInterfaces, &ipInterface)
					}
				} else {
					utils.LogWarning(fmt.Sprintf("CSV line %d - no interface provided for unmanaged workload %s.", csvLine, line[input.MatchIndex]), true)
				}

				// Create the labels slice
				labels := []*illumioapi.Label{}

				// Create the columns and keys slices
				columns := []int{}
				keys := []string{}
				if input.AppIndex != 99998 {
					columns = append(columns, input.AppIndex)
					keys = append(keys, "app")
				}
				if input.RoleIndex != 99998 {
					columns = append(columns, input.RoleIndex)
					keys = append(keys, "role")
				}
				if input.EnvIndex != 99998 {
					columns = append(columns, input.EnvIndex)
					keys = append(keys, "env")
				}
				if input.LocIndex != 99998 {
					columns = append(columns, input.LocIndex)
					keys = append(keys, "loc")
				}

				// Iterate through our labels
				for i := 0; i <= len(columns)-1; i++ {
					if line[columns[i]] == "" {
						continue
					}
					// Get the label HREF
					l := checkLabel(input.PCE, input.UpdatePCE, illumioapi.Label{Key: keys[i], Value: line[columns[i]]}, csvLine)

					// Add that label to the new labels slice
					labels = append(labels, &illumioapi.Label{Href: l.Href})
				}

				// Proces the name
				var name string
				if input.NameIndex != 99998 {
					name = line[input.NameIndex]
					if name == "" {
						name = line[input.MatchIndex]
					}
				}

				// Process the Public IP
				var publicIP string
				if input.PublicIPIndex != 99998 {
					if !publicIPIsValid(line[input.PublicIPIndex]) {
						utils.LogError(fmt.Sprintf("CSV line %d - invalid Public IP address format.", csvLine))
					}
					publicIP = line[input.PublicIPIndex]
				}

				// Process string variables requiring no logic check.
				var desc, extDataRef, extDataSet, osID, osDetail, dataCenter string
				varPtrs := []*string{&desc, &extDataRef, &extDataSet, &osID, &osDetail, &dataCenter}
				inpuIndices := []int{input.DescIndex, input.ExtDataRefIndex, input.ExtDataSetIndex, input.OSIDIndex, input.OSDetailIndex, input.DatacenterIndex}

				for i, varPtr := range varPtrs {
					if inpuIndices[i] == 99998 {
						continue
					}
					*varPtr = line[inpuIndices[i]]
				}

				// Create the unmanaged workload object and add to slice
				w := illumioapi.Workload{
					Hostname:              line[input.MatchIndex],
					Name:                  name,
					Interfaces:            netInterfaces,
					Labels:                labels,
					Description:           desc,
					ExternalDataReference: extDataRef,
					ExternalDataSet:       extDataSet,
					OsID:                  osID,
					OsDetail:              osDetail,
					PublicIP:              publicIP,
					DataCenter:            dataCenter,
				}
				newUMWLs = append(newUMWLs, w)

				// Log the entry
				x := []string{}
				for _, i := range netInterfaces {
					if i.CidrBlock != nil {
						x = append(x, i.Name+":"+i.Address+"/"+strconv.Itoa(*i.CidrBlock))
					} else {
						x = append(x, i.Name+":"+i.Address)
					}
				}
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s to be created - %s (role), %s (app), %s (env), %s(loc) - interfaces: %s", csvLine, w.Hostname, w.GetRole(input.PCE.LabelMapH).Value, w.GetApp(input.PCE.LabelMapH).Value, w.GetEnv(input.PCE.LabelMapH).Value, w.GetLoc(input.PCE.LabelMapH).Value, strings.Join(x, ";")), false)
				continue
			} else {
				// If umwl flag is not set, log the entry
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is not a workload. Include umwl flag to create it. Nothing done.", csvLine, line[input.MatchIndex]), false)
				continue
			}
		}

		// *******************************************
		// *** Get here if the workload does exist ***
		// *******************************************

		// Create a slice told hold new labels if we need to change them
		newWkldLabels := []*illumioapi.Label{}

		// Initialize the change variable
		change := false

		// Create the columns, keys, and labels slices
		columns := []int{}
		keys := []string{}
		labels := []illumioapi.Label{}
		wkld := wkldMap[line[input.MatchIndex]] // Need this since can't perform pointer method on map element
		// Application
		if input.AppIndex != 99998 {
			columns = append(columns, input.AppIndex)
			keys = append(keys, "app")
			labels = append(labels, wkld.GetApp(input.PCE.LabelMapH))
		} else if wkld.GetApp(input.PCE.LabelMapH).Value != "" {
			current := wkld.GetApp(input.PCE.LabelMapH)
			newWkldLabels = append(newWkldLabels, &current)
		}
		// Role
		if input.RoleIndex != 99998 {
			columns = append(columns, input.RoleIndex)
			keys = append(keys, "role")
			labels = append(labels, wkld.GetRole(input.PCE.LabelMapH))
		} else if wkld.GetRole(input.PCE.LabelMapH).Value != "" {
			current := wkld.GetRole(input.PCE.LabelMapH)
			newWkldLabels = append(newWkldLabels, &current)
		}
		// Env
		if input.EnvIndex != 99998 {
			columns = append(columns, input.EnvIndex)
			keys = append(keys, "env")
			labels = append(labels, wkld.GetEnv(input.PCE.LabelMapH))
		} else if wkld.GetEnv(input.PCE.LabelMapH).Value != "" {
			current := wkld.GetEnv(input.PCE.LabelMapH)
			newWkldLabels = append(newWkldLabels, &current)
		}
		// Loc
		if input.LocIndex != 99998 {
			columns = append(columns, input.LocIndex)
			keys = append(keys, "loc")
			labels = append(labels, wkld.GetLoc(input.PCE.LabelMapH))
		} else if wkld.GetLoc(input.PCE.LabelMapH).Value != "" {
			current := wkld.GetLoc(input.PCE.LabelMapH)
			newWkldLabels = append(newWkldLabels, &current)
		}

		// Cycle through each of the four keys
		for i := 0; i <= len(columns)-1; i++ {

			// If the value is blank, skip it
			if line[columns[i]] == "" {
				// Put the old labels back if there is one.
				if labels[i].Href != "" {
					newWkldLabels = append(newWkldLabels, &labels[i])
				}
				continue
			}

			// If the value is the delete value, we turn on the change flag and go to next key
			if line[columns[i]] == input.RemoveValue {
				change = true
				// Log change required
				utils.LogInfo(fmt.Sprintf("%s requiring removal of %s label.", line[input.MatchIndex], keys[i]), false)
				continue
			}

			// If the workload's value does not equal what's in the CSV
			if labels[i].Value != line[columns[i]] {
				// Change the change flag
				change = true
				// Log change required
				utils.LogInfo(fmt.Sprintf("CSV Line - %d - %s requiring %s update from %s to %s.", csvLine, line[input.MatchIndex], keys[i], labels[i].Value, line[columns[i]]), false)
				// Get the label HREF
				l := checkLabel(input.PCE, input.UpdatePCE, illumioapi.Label{Key: keys[i], Value: line[columns[i]]}, csvLine)
				// Add that label to the new labels slice
				newWkldLabels = append(newWkldLabels, &illumioapi.Label{Href: l.Href})
			} else {
				// Keep the existing label if it matches
				newWkldLabels = append(newWkldLabels, &illumioapi.Label{Href: labels[i].Href})
			}
		}

		// We need to check if interfaces have changed
		if wkld.GetMode() == "unmanaged" {
			// If IP field is there and  IP address is provided, check it out
			if input.IntIndex != 99998 && len(line[input.IntIndex]) > 0 {
				// Build out the netInterfaces slice provided by the user
				netInterfaces := []*illumioapi.Interface{}
				nics := strings.Split(strings.Replace(line[input.IntIndex], " ", "", -1), ";")
				for _, n := range nics {
					ipInterface, err := userInputConvert(n)
					if err != nil {
						utils.LogWarning(fmt.Sprintf("CSV Line %d - %s - skipping workload entry.", csvLine, err.Error()), true)
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
						utils.LogInfo(fmt.Sprintf("CSV line %d - Interface not in user provided data - IP: %s, CIDR: %s, Name: %s", csvLine, w.Address, cidrText, w.Name), false)
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
						utils.LogInfo(fmt.Sprintf("CSV line %d - User provided interface not in workload - IP: %s, CIDR: %s, Name: %s", csvLine, u.Address, cidrText, u.Name), false)
					}
				}

				if updateInterfaces {
					wkld.Interfaces = netInterfaces
				}
			}
			// Update the hostname if field provided and matching on Href
			if input.HostnameIndex != 99998 && strings.Contains(line[input.MatchIndex], "/workloads/") {
				if wkld.Hostname != line[input.HostnameIndex] {
					change = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - Hostname to be changed from %s to %s", csvLine, wkld.Hostname, line[input.HostnameIndex]), false)
					wkld.Hostname = line[input.HostnameIndex]
				}
			}
		}

		// Change the name if the name field is provided  it doesn't match unless the name in the CSV is blank and PCE is reporting the name as the hostname
		if input.NameIndex != 99998 && wkld.Name != line[input.NameIndex] && line[input.NameIndex] != "" {
			change = true
			utils.LogInfo(fmt.Sprintf("CSV line %d - Name to be changed from %s to %s", csvLine, wkld.Name, line[input.NameIndex]), false)
			wkld.Name = line[input.NameIndex]
		}

		// Update the Public Ip
		if input.PublicIPIndex != 99998 {
			if line[input.PublicIPIndex] != wkld.PublicIP {
				// Validate it first
				if !publicIPIsValid(line[input.PublicIPIndex]) {
					utils.LogError(fmt.Sprintf("CSV line %d - invalid Public IP address format.", csvLine))
				}
				change = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - Public IP to be changed from %s to %s", csvLine, wkld.PublicIP, line[input.PublicIPIndex]), false)
				wkld.PublicIP = line[input.PublicIPIndex]
			}
		}

		// Update the description column if provided
		if input.DescIndex != 99998 {
			if line[input.DescIndex] != wkld.Description {
				change = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - Desciption to be changed from %s to %s", csvLine, wkld.Description, line[input.DescIndex]), false)
				wkld.Description = line[input.DescIndex]
			}
		}

		// Update the External DataSet
		if input.ExtDataSetIndex != 99998 {
			if line[input.ExtDataSetIndex] != wkld.ExternalDataSet {
				change = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - External Data Set to be changed from %s to %s", csvLine, wkld.ExternalDataSet, line[input.ExtDataSetIndex]), false)
				wkld.ExternalDataSet = line[input.ExtDataSetIndex]
			}
		}

		// Update the External DataRef
		if input.ExtDataRefIndex != 99998 {
			if line[input.ExtDataRefIndex] != wkld.ExternalDataReference {
				change = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - External Data Ref to be changed from %s to %s", csvLine, wkld.ExternalDataReference, line[input.ExtDataRefIndex]), false)
				wkld.ExternalDataReference = line[input.ExtDataRefIndex]
			}
		}

		// Update OS ID
		if input.OSIDIndex != 99998 {
			if line[input.OSIDIndex] != wkld.OsID {
				change = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - OS ID to be changed from %s to %s", csvLine, wkld.OsID, line[input.OSIDIndex]), false)
				wkld.OsID = line[input.OSIDIndex]
			}
		}

		// Update OS Detail
		if input.OSDetailIndex != 99998 {
			if line[input.OSDetailIndex] != wkld.OsDetail {
				change = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - OS Detail to be changed from %s to %s", csvLine, wkld.OsDetail, line[input.OSDetailIndex]), false)
				wkld.OsDetail = line[input.OSDetailIndex]
			}
		}

		// Update Data center
		if input.DatacenterIndex != 99998 {
			if line[input.DatacenterIndex] != wkld.DataCenter {
				change = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - Data Center to be changed from %s to %s", csvLine, wkld.DataCenter, line[input.DatacenterIndex]), false)
				wkld.DataCenter = line[input.DatacenterIndex]
			}
		}

		// If change was flagged, get the workload, update the labels, append to updated slice.
		if change {
			wkld.Labels = newWkldLabels
			updatedWklds = append(updatedWklds, wkld)
		} else {
			unchangedWLs++
		}

	}

	// End run if we have nothing to do
	if len(updatedWklds) == 0 && len(newUMWLs) == 0 {
		utils.LogInfo("nothing to be done", true)
		utils.LogEndCommand("wkld-import")
		return
	}

	// Log findings
	if !input.UpdatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d labels to create.", len(newLabels)), true)
	} else {
		utils.LogInfo(fmt.Sprintf("workloader created %d labels.", createdLabels), true)
	}
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
		fmt.Printf("\r\n%s [PROMPT] - workloader created %d labels in %s (%s) in preparation of updating %d workloads and creating %d unmanaged workloads. Do you want to run the import (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), createdLabels, viper.Get("default_pce_name").(string), viper.Get(viper.Get("default_pce_name").(string)+".fqdn").(string), len(updatedWklds), len(newUMWLs))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("prompt denied to update %d workloads and create %d unmanaged workloads.", len(updatedWklds), len(newUMWLs)), true)
			utils.LogEndCommand("wkld-import")
			return
		}
	}

	// We will only get here if updatePCE and noPrompt is set OR the user accepted the prompt
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
