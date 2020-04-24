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

// Global variables
var matchCol, roleCol, appCol, envCol, locCol, intCol, hostnameCol, nameCol int
var removeValue, csvFile string
var umwl, debug, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error
var newLabels []illumioapi.Label

func init() {

	WkldImportCmd.Flags().BoolVar(&umwl, "umwl", false, "Create unmanaged workloads if the host does not exist. Disabled if matching on href.")
	WkldImportCmd.Flags().IntVarP(&matchCol, "match", "m", 1, "Column number with hostname or href to match workloads. If href is used, --umwl is disabled. First column is 1.")
	WkldImportCmd.Flags().IntVarP(&roleCol, "role", "r", 2, "Column number with new role label.")
	WkldImportCmd.Flags().IntVarP(&appCol, "app", "a", 3, "Column number with new app label.")
	WkldImportCmd.Flags().IntVarP(&envCol, "env", "e", 4, "Column number with new env label.")
	WkldImportCmd.Flags().IntVarP(&locCol, "loc", "l", 5, "Column number with new loc label.")
	WkldImportCmd.Flags().IntVarP(&intCol, "ifaces", "i", 6, "Column number with network interfaces for when creating unmanaged workloads. Each interface should be of the like eth1:192.168.200.20. Separate multiple NICs by semicolons.")
	WkldImportCmd.Flags().IntVarP(&hostnameCol, "hostname", "s", 1, "Column with hostname. Only needs to be set if matching on HREF.")
	WkldImportCmd.Flags().IntVarP(&nameCol, "name", "n", 12, "Column with name.")
	WkldImportCmd.Flags().StringVar(&removeValue, "remove-value", "", "Value in CSV used to remove existing labels. Blank values in the CSV will not change existing. If you want to delete a label do something like --remove-value DELETE and use DELETE in CSV to indicate where to clear existing labels on a workload.")

	WkldImportCmd.Flags().SortFlags = false

}

// WkldImportCmd runs the upload command
var WkldImportCmd = &cobra.Command{
	Use:   "wkld-import [csv file to import]",
	Short: "Create and assign labels to existing workloads and (optionally using the --umwl flag) create unmanaged workloads from a CSV file.",
	Long: `
Create and assign labels to existing workloads and (optionally using the --umwl flag) create unmanaged workloads from a CSV file.

The input should have a header row as the first row will be skipped. Interfaces should be in the format of "eth0:192.168.200.20" and multiple interfaces should be separated by a semicolon with no spaces. Additional columns are allowed and will be ignored.

The default import format is below. It matches the first 6 columns of the workloader export command so you can easily export workloads, edit, and reimport.

+-------------------+------+----------+------+-----+---------------------------------------+
|       host        | role |   app    | env  | loc |              interfaces               |
+-------------------+------+----------+------+-----+---------------------------------------+
| AssetMgt.db.prod  | DB   | ASSETMGT | PROD | BOS | eth0:192.168.200.15                   |
| AssetMgt.web.prod | WEB  | ASSETMGT | PROD | BOS | eth0:192.168.200.15;eth1:10.10.100.22 |
+-------------------+------+----------+------+-----+---------------------------------------+

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

		processCSV()
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
		if debug {
			utils.LogAPIResp("CreateLabel", a)
		}
		if err != nil {
			utils.Log(1, err.Error())
		}
		logJSON, _ := json.Marshal(illumioapi.Label{Href: l.Href, Key: label.Key, Value: label.Value})
		utils.Log(0, fmt.Sprintf("created Label - %s", string(logJSON)))

		// Append the label back to the map
		pce.LabelMapKV[l.Key+l.Value] = l
		pce.LabelMapH[l.Href] = l

		return l
	}

	// If updatePCE is not set, we are create a placeholder href for provided label, and add it back to the maps
	utils.Log(0, fmt.Sprintf("Potential New Label - Key: %s, Value: %s", label.Key, label.Value))
	label.Href = fmt.Sprintf("place-holder-href-%s-%s", label.Key, label.Value)
	pce.LabelMapKV[label.Key+label.Value] = label
	pce.LabelMapH[label.Href] = label
	newLabels = append(newLabels, illumioapi.Label{Key: label.Key, Value: label.Value})

	return label
}

func processCSV() {

	// Log start of the command
	utils.Log(0, "started import command")

	// If debug, log the columns before adjusting by 1
	if debug {
		utils.Log(2, fmt.Sprintf("CSV Columns. Host: %d; Role: %d; App: %d; Env: %d; Loc: %d; Interface: %d", matchCol, roleCol, appCol, envCol, locCol, intCol))
	}

	// Adjust the columns by one
	matchCol--
	roleCol--
	appCol--
	envCol--
	locCol--
	intCol--
	hostnameCol--
	nameCol--

	// Open CSV File
	file, err := os.Open(csvFile)
	if err != nil {
		utils.Log(1, fmt.Sprintf("opening CSV - %s", err))
	}
	defer file.Close()
	reader := csv.NewReader(utils.ClearBOM(bufio.NewReader(file)))

	// Get workload map by hostname
	wkldMap, a, err := pce.GetWkldHostMap()
	utils.LogAPIResp("GetWkldHostMap", a)
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting workload host map - %s", err))
	}

	// Get the workload map by href so we can look up either
	wkldHrefMap, a, err := pce.GetWkldHrefMap()
	utils.LogAPIResp("GetWkldHrefMap", a)
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting workload href map - %s", err))
	}

	// Combine the two workload maps
	for _, w := range wkldHrefMap {
		wkldMap[w.Href] = w
	}

	// Create slices to hold the workloads we will update and create
	updatedWklds := []illumioapi.Workload{}
	newUMWLs := []illumioapi.Workload{}

	// Start the counters
	i := 0

	// Iterate through CSV entries
	for {

		// Increment the counter
		i++

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("reading CSV file - %s", err))
		}

		// Skip the header row
		if i == 1 {
			continue
		}

		if i == 2 && strings.Contains(line[matchCol], "/workloads/") {
			umwl = false
		}

		if line[matchCol] == "" {
			utils.Log(0, fmt.Sprintf("CSV line %d - the match column cannot be blank - hostname or href required.", i))
			fmt.Printf("Skipping CSV line %d - the match column cannot be blank - hostname or href required.\r\n", i)
			continue
		}

		// Check if the workload exists. If exist, we check if UMWL is set and take action.
		if _, ok := wkldMap[line[matchCol]]; !ok {
			var netInterfaces []*illumioapi.Interface
			if umwl {
				if len(line[intCol]) > 0 {
					// Create the network interfaces
					nics := strings.Split(line[intCol], ";")
					for _, n := range nics {
						ipInterface, err := userInputConvert(n)
						if err != nil {
							utils.Log(1, err.Error())
						}
						netInterfaces = append(netInterfaces, &ipInterface)
					}
				} else {
					utils.Log(0, fmt.Sprintf("CSV line - %d - no interface provided for unmanaged workload.", i))
				}

				// Create the labels slice
				labels := []*illumioapi.Label{}
				columns := []int{appCol, roleCol, envCol, locCol}
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
				w := illumioapi.Workload{Hostname: line[matchCol], Interfaces: netInterfaces, Labels: labels}
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
				utils.Log(0, fmt.Sprintf("CSV line %d - %s to be created - %s (role), %s (app), %s (env), %s(loc) - interfaces: %s", i, w.Hostname, w.GetRole(pce.LabelMapH).Value, w.GetApp(pce.LabelMapH).Value, w.GetEnv(pce.LabelMapH).Value, w.GetLoc(pce.LabelMapH).Value, strings.Join(x, ";")))
				continue
			} else {
				// If umwl flag is not set, log the entry
				utils.Log(0, fmt.Sprintf("CSV line %d - %s is not a workload. Include umwl flag to create it. Nothing done.", i, line[matchCol]))
				continue
			}
		}

		// Get here if the workload does exist.
		// Create a slice told hold new labels if we need to change them
		newLabels := []*illumioapi.Label{}

		// Initialize the change variable
		change := false

		// Set slices to iterate through the 4 keys
		columns := []int{appCol, roleCol, envCol, locCol}
		wkld := wkldMap[line[matchCol]] // Need this since can't perform pointer method on map element
		labels := []illumioapi.Label{wkld.GetApp(pce.LabelMapH), wkld.GetRole(pce.LabelMapH), wkld.GetEnv(pce.LabelMapH), wkld.GetLoc(pce.LabelMapH)}
		keys := []string{"app", "role", "env", "loc"}

		// Cycle through each of the four keys
		for i := 0; i <= 3; i++ {

			// If the value is blank, skip it
			if line[columns[i]] == "" {
				continue
			}

			// If the value is the delete value, we turn on the change flag and go to next key
			if line[columns[i]] == removeValue {
				change = true
				// Log change required
				utils.Log(0, fmt.Sprintf("%s requiring removal of %s label.", line[matchCol], keys[i]))
				continue
			}

			// If the workload's  value does not equal what's in the CSV
			if labels[i].Value != line[columns[i]] {
				// Change the change flag
				change = true
				// Log change required
				utils.Log(0, fmt.Sprintf("%s requiring %s update from %s to %s.", line[matchCol], keys[i], labels[i].Value, line[columns[i]]))
				// Get the label HREF
				l := checkLabel(illumioapi.Label{Key: keys[i], Value: line[columns[i]]})
				// Add that label to the new labels slice
				newLabels = append(newLabels, &illumioapi.Label{Href: l.Href})
			} else {
				// Keep the existing label if it matches
				newLabels = append(newLabels, &illumioapi.Label{Href: labels[i].Href})
			}
		}

		// We need to check if interfaces have changed
		if wkld.GetMode() == "unmanaged" {
			// If IP address is provided, check it out
			if len(line[intCol]) > 0 {
				// Build out the netInterfaces slice provided by the user
				netInterfaces := []*illumioapi.Interface{}
				nics := strings.Split(line[intCol], ";")
				for _, n := range nics {
					ipInterface, err := userInputConvert(n)
					if err != nil {
						utils.Log(1, err.Error())
					}
					netInterfaces = append(netInterfaces, &ipInterface)
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
						utils.Log(0, fmt.Sprintf("CSV line %d - Interface not in user provided data - IP: %s, CIDR: %s, Name: %s", i, w.Address, cidrText, w.Name))
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
						utils.Log(0, fmt.Sprintf("CSV line %d - User provided interface not in workload - IP: %s, CIDR: %s, Name: %s", i, u.Address, cidrText, u.Name))
					}
				}

				if updateInterfaces {
					wkld.Interfaces = netInterfaces
				}
			}
			// Update the hostname if matching on Href
			if strings.Contains(line[matchCol], "/workloads/") {
				if wkld.Hostname != line[hostnameCol] {
					change = true
					utils.Log(0, fmt.Sprintf("CSV line %d - Hostname to be changed from %s to %s", i, wkld.Hostname, line[hostnameCol]))
					wkld.Hostname = line[hostnameCol]
				}
			}
		}

		// Check on the name
		if wkld.Name != line[nameCol] {
			change = true
			utils.Log(0, fmt.Sprintf("CSV line %d - Name to be changed from %s to %s", i, wkld.Name, line[nameCol]))
			wkld.Name = line[nameCol]
		}

		// If change was flagged, get the workload, update the labels, append to updated slice.
		if change {
			wkld.Labels = newLabels
			updatedWklds = append(updatedWklds, wkld)
		}

	}

	// End run if we have nothing to do
	if len(updatedWklds) == 0 && len(newUMWLs) == 0 {
		fmt.Println("Nothing to be done.")
		utils.Log(0, "nothing to be done. completed running import command.")
		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.Log(0, fmt.Sprintf("import identified %d workloads requiring update, %d unmanaged workloads to be created, and %d labels to be created.", len(updatedWklds), len(newUMWLs), len(newLabels)))
		fmt.Printf("Import identified:\r\n%d workloads requiring update\r\n%d unmanaged workloads to be created\r\n%d labels to be created.\r\n\r\nSee workloader.log for all identified changes. To do the import, run again using --update-pce flag.\r\n\r\n", len(updatedWklds), len(newUMWLs), len(newLabels))
		utils.Log(0, "completed running import command")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("Import will update %d workloads and create %d unmanaged workloads. Do you want to run the import (yes/no)? ", len(updatedWklds), len(newUMWLs))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.Log(0, fmt.Sprintf("import identified %d workloads requiring update and %d unmanaged workloads to be created. user denied prompt", len(updatedWklds), len(newUMWLs)))
			fmt.Println("Prompt denied.")
			utils.Log(0, "completed running import command")
			return
		}
	}

	// We will only get here if updatePCE and noPrompt is set OR the user accepted the prompt
	if len(updatedWklds) > 0 {
		api, err := pce.BulkWorkload(updatedWklds, "update")
		if debug {
			for _, a := range api {
				utils.LogAPIResp("BulkWorkloadUpdate", a)
			}
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("bulk updating workloads - %s", err))
		}
		utils.Log(0, fmt.Sprintf("bulk update workload successful for %d workloads", len(updatedWklds)))
	}

	// Bulk create if we have new workloads
	if len(newUMWLs) > 0 {
		api, err := pce.BulkWorkload(newUMWLs, "create")
		if debug {
			for _, a := range api {
				utils.LogAPIResp("BulkWorkloadCreate", a)
			}
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("bulk creating workloads - %s", err))
		}
		utils.Log(0, fmt.Sprintf("bulk create workload successful for %d unmanaged workloads", len(newUMWLs)))
	}

	// Log end
	utils.Log(0, "completed running import command")
}