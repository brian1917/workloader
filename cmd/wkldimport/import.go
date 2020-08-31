package wkldimport

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// FromCSVInput is the data structure the FromCSV function expects
type FromCSVInput struct {
	PCE                                                                     illumioapi.PCE
	ImportFile                                                              string
	MatchCol, HostnameCol, NameCol, RoleCol, AppCol, EnvCol, LocCol, IntCol int
	Umwl, KeepAllPCEInterfaces, FQDNtoHostname, UpdatePCE, NoPrompt         bool
}

// Global variables
var matchCol, roleCol, appCol, envCol, locCol, intCol, hostnameCol, nameCol, createdLabels int
var removeValue, csvFile string
var umwl, keepAllPCEInterfaces, fqdnToHostname, debug, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error
var newLabels []illumioapi.Label

func init() {

	WkldImportCmd.Flags().BoolVar(&umwl, "umwl", false, "Create unmanaged workloads if the host does not exist. Disabled if matching on href.")
	WkldImportCmd.Flags().IntVarP(&matchCol, "match", "m", 1, "Column number with hostname or href to match workloads. If href is used, --umwl is disabled. First column is 1.")
	WkldImportCmd.Flags().IntVarP(&hostnameCol, "hostname", "s", 1, "Column with hostname. Only needs to be set if matching on HREF.")
	WkldImportCmd.Flags().IntVarP(&nameCol, "name", "n", 2, "Column with name. When creating UMWLs, if kept blank (recommended), hostname will be assigned to name field.")
	WkldImportCmd.Flags().IntVarP(&roleCol, "role", "r", 3, "Column number with new role label.")
	WkldImportCmd.Flags().IntVarP(&appCol, "app", "a", 4, "Column number with new app label.")
	WkldImportCmd.Flags().IntVarP(&envCol, "env", "e", 5, "Column number with new env label.")
	WkldImportCmd.Flags().IntVarP(&locCol, "loc", "l", 6, "Column number with new loc label.")
	WkldImportCmd.Flags().IntVarP(&intCol, "ifaces", "i", 7, "Column number with network interfaces for when creating unmanaged workloads. Each interface should be of the like eth1:192.168.200.20. Separate multiple NICs by semicolons.")
	WkldImportCmd.Flags().StringVar(&removeValue, "remove-value", "", "Value in CSV used to remove existing labels. Blank values in the CSV will not change existing. If you want to delete a label do something like --remove-value DELETE and use DELETE in CSV to indicate where to clear existing labels on a workload.")
	WkldImportCmd.Flags().BoolVarP(&keepAllPCEInterfaces, "keep-all-pce-interfaces", "k", false, "Will not delete an interface on an unmanaged workload if it's not in the import. It will only add interfaces to the workload.")

	// Hidden flag for use when called from SNOW command
	WkldImportCmd.Flags().BoolVarP(&fqdnToHostname, "fqdn-to-hostname", "f", false, "Convert FQDN hostnames reported by Illumio VEN to short hostnames by removing everything after first period (e.g., test.domain.com becomes test).")
	WkldImportCmd.Flags().MarkHidden("fqdn-to-hostname")

	WkldImportCmd.Flags().SortFlags = false

}

// WkldImportCmd runs the upload command
var WkldImportCmd = &cobra.Command{
	Use:   "wkld-import [csv file to import]",
	Short: "Create and assign labels to existing workloads and/or create unmanaged workloads (using --umwl) from a CSV file.",
	Long: `
Create and assign labels to existing workloads and/or create unmanaged workloads (using --umwl) from a CSV file.

The input file requires:
  - Header row (first line is always skipped)
  - At least seven columns (others are ok, but will be ignored)
  - Interfaces should be in the format of "192.168.200.20", "192.168.200.20/24", "eth0:192.168.200.20", or "eth0:192.168.200.20/24". If no interface name is provided with a colon (e.g., "eth0:"), then "umwl:" is used. Multiple interfaces should be separated by a semicolon.
  - Default column order is below. The order matches the first 7 columns of the wkld-export command to easily export workloads, edit, and re-import.
  - Override column numbers for other input formats using flags.

+---------------+------+------+----------+------+-----+---------------------------------------+
|   hostname    | name | role |   app    | env  | loc |              interfaces               |
+---------------+------+------+----------+------+-----+---------------------------------------+
| assetmgtdbp1  |      | DB   | ASSETMGT | PROD | BOS | eth0:192.168.200.15                   |
| assetmgtwebd1 |      | WEB  | ASSETMGT | DEV  | BOS | eth0:192.168.200.15;eth1:10.10.100.22 |
+---------------+------+------+----------+------+-----+---------------------------------------+

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetDefaultPCE(true)
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
			MatchCol:             matchCol,
			HostnameCol:          hostnameCol,
			NameCol:              nameCol,
			RoleCol:              roleCol,
			AppCol:               appCol,
			EnvCol:               envCol,
			LocCol:               locCol,
			IntCol:               intCol,
			FQDNtoHostname:       fqdnToHostname, // This is only used when coming from SNOW when a flag is set.
			KeepAllPCEInterfaces: keepAllPCEInterfaces,
			UpdatePCE:            updatePCE,
			NoPrompt:             noPrompt,
		}

		FromCSV(f)
	},
}

func checkLabel(label illumioapi.Label) illumioapi.Label {

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
		logJSON, _ := json.Marshal(illumioapi.Label{Href: l.Href, Key: label.Key, Value: label.Value})
		utils.LogInfo(fmt.Sprintf("created Label - %s", string(logJSON)), false)

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

	// If debug, log the columns before adjusting by 1
	utils.LogDebug(fmt.Sprintf("CSV Columns. Host: %d; Role: %d; App: %d; Env: %d; Loc: %d; Interface: %d", f.MatchCol, f.RoleCol, f.AppCol, f.EnvCol, f.LocCol, f.IntCol))

	// Adjust the columns by one
	f.MatchCol--
	f.RoleCol--
	f.AppCol--
	f.EnvCol--
	f.LocCol--
	f.IntCol--
	f.HostnameCol--
	f.NameCol--

	// Open CSV File
	file, err := os.Open(f.ImportFile)
	if err != nil {
		utils.LogError(fmt.Sprintf("opening CSV - %s", err))
	}
	defer file.Close()
	reader := csv.NewReader(utils.ClearBOM(bufio.NewReader(file)))

	// Get workload map by hostname
	wkldMap := make(map[string]illumioapi.Workload)
	var a illumioapi.APIResponse
	if !f.FQDNtoHostname {
		wkldMap, a, err = f.PCE.GetWkldHostMap()
		utils.LogAPIResp("GetWkldHostMap", a)
		if err != nil {
			utils.LogError(fmt.Sprintf("getting workload host map - %s", err))
		}
	}

	// If we need to, strip hostnames for after "."
	if f.FQDNtoHostname {
		tempWkldMap, a, err := f.PCE.GetWkldHostMap()
		utils.LogAPIResp("GetWkldHostMap", a)
		if err != nil {
			utils.LogError(fmt.Sprintf("getting workload host map - %s", err))
		}
		for _, w := range tempWkldMap {
			newHost := strings.Split(w.Hostname, ".")[0]
			w.Hostname = newHost
			wkldMap[w.Hostname] = w
		}
	}

	// Get the workload map by href so we can look up either
	wkldHrefMap, a, err := f.PCE.GetWkldHrefMap()
	utils.LogAPIResp("GetWkldHrefMap", a)
	if err != nil {
		utils.LogError(fmt.Sprintf("getting workload href map - %s", err))
	}

	// Combine the two workload maps
	for _, w := range wkldHrefMap {
		wkldMap[w.Href] = w
	}

	// Create slices to hold the workloads we will update and create
	updatedWklds := []illumioapi.Workload{}
	newUMWLs := []illumioapi.Workload{}

	// Start the counters
	csvLine := 0
	unchangedWLs := 0

	// Iterate through CSV entries
CSVEntries:
	for {

		// Increment the counter
		csvLine++

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.LogError(fmt.Sprintf("reading CSV file - %s", err))
		}

		// Skip the header row
		if csvLine == 1 {
			continue
		}

		if csvLine == 2 && strings.Contains(line[f.MatchCol], "/workloads/") {
			f.Umwl = false
		}

		if line[f.MatchCol] == "" {
			utils.LogWarning(fmt.Sprintf("CSV line %d - the match column cannot be blank - hostname or href required.", csvLine), true)
			continue
		}

		// Check if the workload exists. If exist, we check if UMWL is set and take action.
		if _, ok := wkldMap[line[f.MatchCol]]; !ok {
			var netInterfaces []*illumioapi.Interface
			if f.Umwl {
				if len(line[f.IntCol]) > 0 {
					// Create the network interfaces
					nics := strings.Split(strings.ReplaceAll(line[f.IntCol], " ", ""), ";")
					for _, n := range nics {
						ipInterface, err := userInputConvert(n)
						if err != nil {
							utils.LogError(err.Error())
						}
						netInterfaces = append(netInterfaces, &ipInterface)
					}
				} else {
					utils.LogWarning(fmt.Sprintf("CSV line %d - no interface provided for unmanaged workload %s.", csvLine, line[f.MatchCol]), true)
				}

				// Create the labels slice
				labels := []*illumioapi.Label{}
				columns := []int{f.AppCol, f.RoleCol, f.EnvCol, f.LocCol}
				keys := []string{"app", "role", "env", "loc"}
				for i := 0; i <= 3; i++ {
					if line[columns[i]] == "" {
						continue
					}
					// Get the label HREF
					l := checkLabel(illumioapi.Label{Key: keys[i], Value: line[columns[i]]})

					// Add that label to the new labels slice
					labels = append(labels, &illumioapi.Label{Href: l.Href})
				}

				// Add to the slice to process via bulk and go to next CSV entry
				name := line[f.NameCol]
				if name == "" {
					name = line[f.MatchCol]
				}
				w := illumioapi.Workload{Hostname: line[f.MatchCol], Name: name, Interfaces: netInterfaces, Labels: labels}
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
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is not a workload. Include umwl flag to create it. Nothing done.", csvLine, line[f.MatchCol]), false)
				continue
			}
		}

		// Get here if the workload does exist.
		// Create a slice told hold new labels if we need to change them
		newWkldLabels := []*illumioapi.Label{}

		// Initialize the change variable
		change := false

		// Set slices to iterate through the 4 keys
		columns := []int{f.AppCol, f.RoleCol, f.EnvCol, f.LocCol}
		wkld := wkldMap[line[f.MatchCol]] // Need this since can't perform pointer method on map element
		labels := []illumioapi.Label{wkld.GetApp(f.PCE.LabelMapH), wkld.GetRole(f.PCE.LabelMapH), wkld.GetEnv(f.PCE.LabelMapH), wkld.GetLoc(f.PCE.LabelMapH)}
		keys := []string{"app", "role", "env", "loc"}

		// Cycle through each of the four keys
		for i := 0; i <= 3; i++ {

			// If the value is blank, skip it
			if line[columns[i]] == "" {
				// Put the old labels back in case there is a change
				newWkldLabels = append(newWkldLabels, &labels[i])
				continue
			}

			// If the value is the delete value, we turn on the change flag and go to next key
			if line[columns[i]] == removeValue {
				change = true
				// Log change required
				utils.LogInfo(fmt.Sprintf("%s requiring removal of %s label.", line[f.MatchCol], keys[i]), false)
				continue
			}

			// If the workload's  value does not equal what's in the CSV
			if labels[i].Value != line[columns[i]] {
				// Change the change flag
				change = true
				// Log change required
				utils.LogInfo(fmt.Sprintf("%s requiring %s update from %s to %s.", line[f.MatchCol], keys[i], labels[i].Value, line[columns[i]]), false)
				// Get the label HREF
				l := checkLabel(illumioapi.Label{Key: keys[i], Value: line[columns[i]]})
				// Add that label to the new labels slice
				newWkldLabels = append(newWkldLabels, &illumioapi.Label{Href: l.Href})
			} else {
				// Keep the existing label if it matches
				newWkldLabels = append(newWkldLabels, &illumioapi.Label{Href: labels[i].Href})
			}
		}

		// We need to check if interfaces have changed
		if wkld.GetMode() == "unmanaged" {
			// If IP address is provided, check it out
			if len(line[f.IntCol]) > 0 {
				// Build out the netInterfaces slice provided by the user
				netInterfaces := []*illumioapi.Interface{}
				nics := strings.Split(line[f.IntCol], ";")
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
			// Update the hostname if matching on Href
			if strings.Contains(line[f.MatchCol], "/workloads/") {
				if wkld.Hostname != line[f.HostnameCol] {
					change = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - Hostname to be changed from %s to %s", csvLine, wkld.Hostname, line[f.HostnameCol]), false)
					wkld.Hostname = line[f.HostnameCol]
				}
			}
		}

		// Change the name if it doesn't match unless the name in the CSV is blank and PCE is reporting the name as the hostname
		if wkld.Name != line[f.NameCol] && line[f.NameCol] != "" {
			change = true
			utils.LogInfo(fmt.Sprintf("CSV line %d - Name to be changed from %s to %s", csvLine, wkld.Name, line[f.NameCol]), false)
			wkld.Name = line[f.NameCol]
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
		fmt.Printf("\r\n[PROMPT] - workloader created %d labels in %s (%s) in preparation of updating %d workloads and creating %d unmanaged workloads. Do you want to run the import (yes/no)? ", createdLabels, viper.Get("default_pce_name").(string), viper.Get(viper.Get("default_pce_name").(string)+".fqdn").(string), len(updatedWklds), len(newUMWLs))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("prompt denied to update %d workloads and create %d unmanaged workloads.", len(updatedWklds), len(newUMWLs)), true)
			utils.LogEndCommand("wkld-import")
			return
		}
	}

	// We will only get here if updatePCE and noPrompt is set OR the user accepted the prompt
	if len(updatedWklds) > 0 {
		api, err := f.PCE.BulkWorkload(updatedWklds, "update")
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
		api, err := f.PCE.BulkWorkload(newUMWLs, "create")
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
