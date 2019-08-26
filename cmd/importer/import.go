package importer

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
var matchCol, roleCol, appCol, envCol, locCol, intCol int
var removeValue, csvFile string
var umwl, debug, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error
var labelMapKV, labelMapHref map[string]illumioapi.Label

func init() {

	ImportCmd.Flags().BoolVar(&umwl, "umwl", false, "Create unmanaged workloads if the host does not exist. Auto-disabled if matching on HREF.")
	ImportCmd.Flags().IntVarP(&matchCol, "match", "m", 2, "Column number with hostname or Href to match workloads. First column is 1.")
	ImportCmd.Flags().IntVarP(&roleCol, "role", "r", 3, "Column number with new role label.")
	ImportCmd.Flags().IntVarP(&appCol, "app", "a", 4, "Column number with new app label.")
	ImportCmd.Flags().IntVarP(&envCol, "env", "e", 5, "Column number with new env label.")
	ImportCmd.Flags().IntVarP(&locCol, "loc", "l", 6, "Column number with new loc label.")
	ImportCmd.Flags().IntVarP(&intCol, "ifaces", "i", 7, "Column number with network interfaces for when creating unmanaged workloads. Each interface should be of the like eth1:192.168.200.20. Separate multiple NICs by semicolons.")
	ImportCmd.Flags().StringVar(&removeValue, "remove-value", "", "Value in CSV used to remove existing labels. Blank values in the CSV will not change existing. If you want to delete a label do something like --remove-value DELETE and use DELETE in CSV to indicate where to clear existing labels on a workload.")

	ImportCmd.Flags().SortFlags = false

}

// ImportCmd runs the upload command
var ImportCmd = &cobra.Command{
	Use:   "import [csv file to import]",
	Short: "Create and assign labels to a workload from a CSV file. Use the --umwl flag to create and label unmanaged workloads from the same CSV.",
	Long: `
Create and assign labels to a workload from a CSV file.

Use the --umwl flag to create and label unmanaged workloads from the same CSV, if matching on hostnames. If a hostname is not found with --umwl, import will create it.

The input should have a header row as the first row will be skipped. Interfaces should be in the format of "eth0:192.168.200.20" and multiple interfaces should be separated by a semicolon with no spaces. Additional columns are allowed and will be ignored.

The match can be either hostname or href. If matching on href, the --umwl flag will automatically be disabled.

The default import format is below. It matches the first 6 columns of the workloader export command so you can easily export workloads, edit, and reimport.

+-------------------+--------------------------------------------------------+------+----------+------+-----+---------------------------------------+
|       host        |                          href                          | role |   app    | env  | loc |              interfaces               |
+-------------------+--------------------------------------------------------+------+----------+------+-----+---------------------------------------+
| AssetMgt.db.prod  | /orgs/1/workloads/17589443-7731-488f-b57a-f26c9d9e9eff | DB   | ASSETMGT | PROD | BOS | eth0:192.168.200.15                   |
| AssetMgt.web.prod | /orgs/1/workloads/12384475-7491-428e-b47c-f36c5d8e9eff | WEB  | ASSETMGT | PROD | BOS | eth0:192.168.200.15;eth1:10.10.100.22 |
+-------------------+--------------------------------------------------------+------+----------+------+-----+---------------------------------------+

Import will create labels even without --update-pce. Workloads will not be created/updated without --update-pce.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetPCE()
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
	if _, ok := labelMapKV[label.Key+label.Value]; ok {
		return labelMapKV[label.Key+label.Value]
	}

	// Create the label if it doesn't exist
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
	labelMapKV[l.Key+l.Value] = l
	labelMapHref[l.Href] = l

	return l
}

// ContainsStr hecks if an integer is in a slice
func containsStr(strSlice []string, searchStr string) bool {
	for _, value := range strSlice {
		if value == searchStr {
			return true
		}
	}
	return false
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

	// Open CSV File
	file, err := os.Open(csvFile)
	if err != nil {
		utils.Log(1, fmt.Sprintf("opening CSV - %s", err))
	}
	defer file.Close()
	reader := csv.NewReader(utils.ClearBOM(bufio.NewReader(file)))

	// Get workload map. Build based off hostname and and Href so we can look up either.
	wkldMap, a, err := pce.GetWkldHostMap()
	if debug {
		utils.LogAPIResp("GetWkldHostMap", a)
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting workload host map - %s", err))
	}
	for _, w := range wkldMap {
		wkldMap[w.Href] = w
	}

	// Get label map
	labelMapKV, a, err = pce.GetLabelMapKV()
	if debug {
		utils.LogAPIResp("GetLabelMapKV", a)
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting label key value map - %s", err))
	}
	labelMapHref, a, err = pce.GetLabelMapH()
	if debug {
		utils.LogAPIResp("GetLabelMapH", a)
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting label href map - %s", err))
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
				utils.Log(0, fmt.Sprintf("CSV line %d - %s to be created - %s (role), %s (app), %s (env), %s(loc) - interfaces: %s", i, w.Hostname, w.GetRole(labelMapHref).Value, w.GetApp(labelMapHref).Value, w.GetEnv(labelMapHref).Value, w.GetLoc(labelMapHref).Value, strings.Join(x, ";")))
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
		labels := []illumioapi.Label{wkld.GetApp(labelMapHref), wkld.GetRole(labelMapHref), wkld.GetEnv(labelMapHref), wkld.GetLoc(labelMapHref)}
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
					if w.CidrBlock != nil {
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
					if u.CidrBlock != nil {
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
		utils.Log(0, fmt.Sprintf("import identified %d workloads requiring update and %d unmanaged workloads to be created", len(updatedWklds), len(newUMWLs)))
		fmt.Printf("Import identified %d workloads requiring update and %d unmanaged workloads to be created. To do the import, run again using --update-pce flag. The --auto flag will bypass the prompt if used with --update-pce.\r\n", len(updatedWklds), len(newUMWLs))
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
