package wkldimport

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// FromCSVInput is the data structure the FromCSV function expects
type FromCSVInput struct {
	PCE                                                                                                           illumioapi.PCE
	ImportFile                                                                                                    string
	MatchIndex, HostnameIndex, NameIndex, RoleIndex, AppIndex, EnvIndex, LocIndex, IntIndex, DescIndex, HrefIndex int
	Umwl, KeepAllPCEInterfaces, FQDNtoHostname, UpdatePCE, NoPrompt                                               bool
}

// Global variables
var matchCol, roleCol, appCol, envCol, locCol, intCol, hostnameCol, nameCol, descCol, hrefCol, createdLabels int
var removeValue, csvFile string
var umwl, keepAllPCEInterfaces, fqdnToHostname, debug, updatePCE, noPrompt bool
var err error
var newLabels []illumioapi.Label

func init() {

	WkldImportCmd.Flags().BoolVar(&umwl, "umwl", false, "Create unmanaged workloads if the host does not exist. Disabled if matching on href.")
	WkldImportCmd.Flags().StringVar(&removeValue, "remove-value", "", "Value in CSV used to remove existing labels. Blank values in the CSV will not change existing. If you want to delete a label do something like --remove-value DELETE and use DELETE in CSV to indicate where to clear existing labels on a workload.")
	WkldImportCmd.Flags().IntVarP(&matchCol, "match", "m", 99999, "Column number to override selected column to match workloads.")
	WkldImportCmd.Flags().IntVarP(&hostnameCol, "hostname", "s", 99999, "Column with hostname.")
	WkldImportCmd.Flags().MarkHidden("hostname")
	WkldImportCmd.Flags().IntVarP(&hrefCol, "href", "z", 99999, "Column with href.")
	WkldImportCmd.Flags().MarkHidden("href")
	WkldImportCmd.Flags().IntVarP(&nameCol, "name", "n", 99999, "Column with name. When creating UMWLs, if kept blank (recommended), hostname will be assigned to name field.")
	WkldImportCmd.Flags().MarkHidden("name")
	WkldImportCmd.Flags().IntVarP(&roleCol, "role", "r", 99999, "Column number with new role label.")
	WkldImportCmd.Flags().MarkHidden("role")
	WkldImportCmd.Flags().IntVarP(&appCol, "app", "a", 99999, "Column number with new app label.")
	WkldImportCmd.Flags().MarkHidden("app")
	WkldImportCmd.Flags().IntVarP(&envCol, "env", "e", 99999, "Column number with new env label.")
	WkldImportCmd.Flags().MarkHidden("env")
	WkldImportCmd.Flags().IntVarP(&locCol, "loc", "l", 99999, "Column number with new loc label.")
	WkldImportCmd.Flags().MarkHidden("loc")
	WkldImportCmd.Flags().IntVarP(&intCol, "interfaces", "i", 99999, "Column number with network interfaces for when creating unmanaged workloads. Each interface should be of the like eth1:192.168.200.20. Separate multiple NICs by semicolons.")
	WkldImportCmd.Flags().MarkHidden("interfaces")
	WkldImportCmd.Flags().IntVarP(&descCol, "description", "d", 99999, "Column number with the workload description.")
	WkldImportCmd.Flags().MarkHidden("description")

	// Hidden flag for use when called from SNOW command
	WkldImportCmd.Flags().BoolVarP(&fqdnToHostname, "fqdn-to-hostname", "f", false, "Convert FQDN hostnames reported by Illumio VEN to short hostnames by removing everything after first period (e.g., test.domain.com becomes test).")
	WkldImportCmd.Flags().MarkHidden("fqdn-to-hostname")
	WkldImportCmd.Flags().BoolVarP(&keepAllPCEInterfaces, "keep-all-pce-interfaces", "k", false, "Will not delete an interface on an unmanaged workload if it's not in the import. It will only add interfaces to the workload.")
	WkldImportCmd.Flags().MarkHidden("keep-all-pce-interfaces")

	WkldImportCmd.Flags().SortFlags = false

}

// WkldImportCmd runs the upload command
var WkldImportCmd = &cobra.Command{
	Use:   "wkld-import [csv file to import]",
	Short: "Create and assign labels to existing workloads and/or create unmanaged workloads (using --umwl) from a CSV file.",
	Long: `
Create and assign labels to existing workloads and/or create unmanaged workloads (using --umwl) from a CSV file.

The input file requires headers and matches fields to header values. The following headers can be used:
- hostname
- name
- role
- app
- env
- loc
- interfaces
- description
- href

Besides either href or hostname for matching, no field is required.
For example, to only update the location field you can provide just two columns: href and loc (or hostname and loc). All other workload properties will be preserved.
Similarily, if to only update labels, you do not need to include an interface, name, description, etc.

If you need to override the header to to field matching you can specify the column number with any flag.
For example --name 2 will force workloader to use the second column in the CSV as the name field, regardless of what the header value is.

Other columns are alloewd but will be ignored.

Interfaces should be in the format of "192.168.200.20", "192.168.200.20/24", "eth0:192.168.200.20", or "eth0:192.168.200.20/24".
If no interface name is provided with a colon (e.g., "eth0:"), then "umwl:" is used. Multiple interfaces should be separated by a semicolon.

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err := utils.GetTargetPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		f := FromCSVInput{
			PCE:                  pce,
			ImportFile:           csvFile,
			Umwl:                 umwl,
			MatchIndex:           matchCol,
			HostnameIndex:        hostnameCol,
			NameIndex:            nameCol,
			HrefIndex:            hrefCol,
			RoleIndex:            roleCol,
			AppIndex:             appCol,
			EnvIndex:             envCol,
			LocIndex:             locCol,
			IntIndex:             intCol,
			DescIndex:            descCol,
			FQDNtoHostname:       fqdnToHostname, // This is only used when coming from SNOW when a flag is set.
			KeepAllPCEInterfaces: keepAllPCEInterfaces,
			UpdatePCE:            updatePCE,
			NoPrompt:             noPrompt,
		}

		FromCSV(f)
	},
}

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

// FromCSV imports a CSV to label unmanaged workloads and create unmanaged workloads
func FromCSV(f FromCSVInput) {

	// Log start of the command
	utils.LogStartCommand("wkld-import")

	// Parse the CSV File
	data, err := utils.ParseCSV(f.ImportFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Process the headers
	f.processHeaders(data[0])

	// Adjust the columns by one
	f.decreaseColBy1()

	// Log our intput
	f.log()

	// Get the workload map by href
	utils.LogInfo("Starting GET all workloads.", true)
	wkldMap, a, err := f.PCE.GetWkldHrefMap()
	utils.LogAPIResp("GetWkldHrefMap", a)
	if err != nil {
		utils.LogError(fmt.Sprintf("getting workload href map - %s", err))
	}
	utils.LogInfo(fmt.Sprintf("GET all workloads complete - %d workloads", len(wkldMap)), true)

	// Get the hostnames
	wkldHostNameMap := make(map[string]illumioapi.Workload)
	for _, w := range wkldMap {
		hostname := w.Hostname
		if f.FQDNtoHostname {
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
		if csvLine == 2 && strings.Contains(line[f.MatchIndex], "/workloads/") && f.Umwl {
			utils.LogError("cannot match on hrefs and create unmanaged workloads")
		}

		// Check to make sure we have an entry in the match column
		if line[f.MatchIndex] == "" {
			utils.LogWarning(fmt.Sprintf("CSV line %d - the match column cannot be blank - hostname or href required.", csvLine), true)
			continue
		}

		// Check if the workload exists. If exist, we check if UMWL is set and take action.
		if _, ok := wkldMap[line[f.MatchIndex]]; !ok {
			var netInterfaces []*illumioapi.Interface
			if f.Umwl {
				// Process if interface is in import and if interface entry has values
				if f.IntIndex != 99998 && len(line[f.IntIndex]) > 0 {
					// Create the network interfaces

					nics := strings.Split(strings.ReplaceAll(line[f.IntIndex], " ", ""), ";")
					for _, n := range nics {
						ipInterface, err := userInputConvert(n)
						if err != nil {
							utils.LogError(err.Error())
						}
						netInterfaces = append(netInterfaces, &ipInterface)
					}
				} else {
					utils.LogWarning(fmt.Sprintf("CSV line %d - no interface provided for unmanaged workload %s.", csvLine, line[f.MatchIndex]), true)
				}

				// Create the labels slice
				labels := []*illumioapi.Label{}

				// Create the columns and keys slices
				columns := []int{}
				keys := []string{}
				if f.AppIndex != 99998 {
					columns = append(columns, f.AppIndex)
					keys = append(keys, "app")
				}
				if f.RoleIndex != 99998 {
					columns = append(columns, f.RoleIndex)
					keys = append(keys, "role")
				}
				if f.EnvIndex != 99998 {
					columns = append(columns, f.EnvIndex)
					keys = append(keys, "env")
				}
				if f.LocIndex != 99998 {
					columns = append(columns, f.LocIndex)
					keys = append(keys, "loc")
				}

				// Iterate through our labels
				for i := 0; i <= len(columns)-1; i++ {
					if line[columns[i]] == "" {
						continue
					}
					// Get the label HREF
					l := checkLabel(f.PCE, f.UpdatePCE, illumioapi.Label{Key: keys[i], Value: line[columns[i]]}, csvLine)

					// Add that label to the new labels slice
					labels = append(labels, &illumioapi.Label{Href: l.Href})
				}

				// Proces the name
				var name string
				if f.NameIndex != 99998 {
					name := line[f.NameIndex]
					if name == "" {
						name = line[f.MatchIndex]
					}
				}

				// Create the unmanaged workload object and add to slice
				w := illumioapi.Workload{Hostname: line[f.MatchIndex], Name: name, Interfaces: netInterfaces, Labels: labels}
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
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s to be created - %s (role), %s (app), %s (env), %s(loc) - interfaces: %s", csvLine, w.Hostname, w.GetRole(f.PCE.LabelMapH).Value, w.GetApp(f.PCE.LabelMapH).Value, w.GetEnv(f.PCE.LabelMapH).Value, w.GetLoc(f.PCE.LabelMapH).Value, strings.Join(x, ";")), false)
				continue
			} else {
				// If umwl flag is not set, log the entry
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is not a workload. Include umwl flag to create it. Nothing done.", csvLine, line[f.MatchIndex]), false)
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
		wkld := wkldMap[line[f.MatchIndex]] // Need this since can't perform pointer method on map element
		// Application
		if f.AppIndex != 99998 {
			columns = append(columns, f.AppIndex)
			keys = append(keys, "app")
			labels = append(labels, wkld.GetApp(f.PCE.LabelMapH))
		} else if wkld.GetApp(f.PCE.LabelMapH).Value != "" {
			current := wkld.GetApp(f.PCE.LabelMapH)
			newWkldLabels = append(newWkldLabels, &current)
		}
		// Role
		if f.RoleIndex != 99998 {
			columns = append(columns, f.RoleIndex)
			keys = append(keys, "role")
			labels = append(labels, wkld.GetRole(f.PCE.LabelMapH))
		} else if wkld.GetRole(f.PCE.LabelMapH).Value != "" {
			current := wkld.GetRole(f.PCE.LabelMapH)
			newWkldLabels = append(newWkldLabels, &current)
		}
		// Env
		if f.EnvIndex != 99998 {
			columns = append(columns, f.EnvIndex)
			keys = append(keys, "env")
			labels = append(labels, wkld.GetEnv(f.PCE.LabelMapH))
		} else if wkld.GetEnv(f.PCE.LabelMapH).Value != "" {
			current := wkld.GetEnv(f.PCE.LabelMapH)
			newWkldLabels = append(newWkldLabels, &current)
		}
		// Loc
		if f.LocIndex != 99998 {
			columns = append(columns, f.LocIndex)
			keys = append(keys, "loc")
			labels = append(labels, wkld.GetLoc(f.PCE.LabelMapH))
		} else if wkld.GetLoc(f.PCE.LabelMapH).Value != "" {
			current := wkld.GetLoc(f.PCE.LabelMapH)
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
			if line[columns[i]] == removeValue {
				change = true
				// Log change required
				utils.LogInfo(fmt.Sprintf("%s requiring removal of %s label.", line[f.MatchIndex], keys[i]), false)
				continue
			}

			// If the workload's value does not equal what's in the CSV
			if labels[i].Value != line[columns[i]] {
				// Change the change flag
				change = true
				// Log change required
				utils.LogInfo(fmt.Sprintf("CSV Line - %d - %s requiring %s update from %s to %s.", csvLine, line[f.MatchIndex], keys[i], labels[i].Value, line[columns[i]]), false)
				// Get the label HREF
				l := checkLabel(f.PCE, f.UpdatePCE, illumioapi.Label{Key: keys[i], Value: line[columns[i]]}, csvLine)
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
			if f.IntIndex != 99998 && len(line[f.IntIndex]) > 0 {
				// Build out the netInterfaces slice provided by the user
				netInterfaces := []*illumioapi.Interface{}
				nics := strings.Split(strings.ReplaceAll(line[f.IntIndex], " ", ""), ";")
				for _, n := range nics {
					ipInterface, err := userInputConvert(n)
					if err != nil {
						utils.LogWarning(fmt.Sprintf("CSV Line %d - %s - skipping workload entry.", csvLine, err.Error()), true)
						continue CSVEntries

					}
					netInterfaces = append(netInterfaces, &ipInterface)
				}

				// If instructed by flag, make sure we keep all PCE interfaces
				if f.KeepAllPCEInterfaces {
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
			if f.HostnameIndex != 99998 && strings.Contains(line[f.MatchIndex], "/workloads/") {
				if wkld.Hostname != line[f.HostnameIndex] {
					change = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - Hostname to be changed from %s to %s", csvLine, wkld.Hostname, line[f.HostnameIndex]), false)
					wkld.Hostname = line[f.HostnameIndex]
				}
			}
		}

		// Change the name if the name field is provided  it doesn't match unless the name in the CSV is blank and PCE is reporting the name as the hostname
		if f.NameIndex != 99998 && wkld.Name != line[f.NameIndex] && line[f.NameIndex] != "" {
			change = true
			utils.LogInfo(fmt.Sprintf("CSV line %d - Name to be changed from %s to %s", csvLine, wkld.Name, line[f.NameIndex]), false)
			wkld.Name = line[f.NameIndex]
		}

		// Update the description column if provided
		if f.DescIndex != 99998 {
			if line[f.DescIndex] != wkld.Description {
				change = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - Desciption to be changed from %s to %s", csvLine, wkld.Description, line[f.DescIndex]), false)
				wkld.Description = line[f.DescIndex]
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
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d labels to create.", len(newLabels)), true)
	} else {
		utils.LogInfo(fmt.Sprintf("workloader created %d labels.", createdLabels), true)
	}
	utils.LogInfo(fmt.Sprintf("workloader identified %d workloads requiring updates.", len(updatedWklds)), true)
	utils.LogInfo(fmt.Sprintf("workloader identified %d unmanaged workloads to create.", len(newUMWLs)), true)
	utils.LogInfo(fmt.Sprintf("%d entries in CSV require no changes", unchangedWLs), true)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !f.UpdatePCE {
		utils.LogInfo("See workloader.log for more details. To do the import, run again using --update-pce flag.", true)
		utils.LogEndCommand("wkld-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if f.UpdatePCE && !f.NoPrompt {
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
		api, err := f.PCE.BulkWorkload(updatedWklds, "update", true)
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
		api, err := f.PCE.BulkWorkload(newUMWLs, "create", true)
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
