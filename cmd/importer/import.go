package importer

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Global variables
var hostCol, roleCol, appCol, envCol, locCol, intCol int
var removeValue, csvFile string
var umwl, debug, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error
var labelMapKV, labelMapHref map[string]illumioapi.Label

func init() {
	ImportCmd.Flags().StringVar(&csvFile, "in", "", "Input csv file. The first row (headers) will be skipped.")
	ImportCmd.MarkFlagRequired("in")
	ImportCmd.Flags().StringVar(&removeValue, "removeValue", "", "Value in CSV used to remove existing labels. Blank values in the CSV will not change existing. If you want to delete a label do something like -removeValue delete and use delete in CSV to indicate where to delete.")
	ImportCmd.Flags().BoolVar(&umwl, "umwl", false, "Create unmanaged workloads if the host does not exist")
	ImportCmd.Flags().IntVarP(&hostCol, "match", "n", 1, "Column number with hostname or Href to match workloads. If you have HREF, we recommend using that. First column is 1.")
	ImportCmd.Flags().IntVarP(&roleCol, "role", "r", 2, "Column number with new role label.")
	ImportCmd.Flags().IntVarP(&appCol, "app", "a", 3, "Column number with new app label.")
	ImportCmd.Flags().IntVarP(&envCol, "env", "e", 4, "Column number with new env label.")
	ImportCmd.Flags().IntVarP(&locCol, "loc", "l", 5, "Column number with new loc label.")
	ImportCmd.Flags().IntVarP(&intCol, "ifaces", "i", 6, "Column number with network interfaces for when creating unmanaged workloads. Each interface should be of the format name:address (e.g., eth1:192.168.200.20). Separate multiple NICs by semicolons.")

	ImportCmd.Flags().SortFlags = false

}

// ImportCmd runs the upload command
var ImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Create and assign labels from a CSV file. Create and label unmanaged workloads from same CSV.",
	Long: `
Create and assign labels from a CSV file. Create and label unmanaged workloads from same CSV.

The default input style is below. The input should have a header row. Headers do not matter but the first row will be skipped.

Using this command with the --umwl flag will label workloads and create unmanaged workloads.

The interface column will be ignored for managed workloads.

You can override column numbers with provided flags. The first column is 1.

Additional columns are allowed and will be ignored.

+----------------+------+----------+------+-----+--------------------+
|      host      | role |   app    | env  | loc |     interface      |
+----------------+------+----------+------+-----+--------------------+
| Asset-Mgt-db-1 | DB   | ASSETMGT | PROD | BOS | eth0:192.168.100.2 |
| Asset-Mgt-db-2 | DB   | ASSETMGT | PROD | BOS | eth0:192.168.100.3 |
+----------------+------+----------+------+-----+--------------------+`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetPCE()
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

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

func processCSV() {

	// Log start of the command
	utils.Log(0, "started import command")

	// If debug, log the columns before adjusting by 1
	if debug {
		utils.Log(2, fmt.Sprintf("CSV Columns. Host: %d; Role: %d; App: %d; Env: %d; Loc: %d; Interface: %d", hostCol, roleCol, appCol, envCol, locCol, intCol))
	}

	// Adjust the columns by one
	hostCol--
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

		if line[hostCol] == "" {
			utils.Log(0, fmt.Sprintf("CSV line %d - the match column cannot be blank - hostname or href required.", i))
			fmt.Printf("Skipping CSV line %d - the match column cannot be blank - hostname or href required.\r\n", i)
			continue
		}

		// Check if the workload exists. If exist, we check if UMWL is set and take action.
		if _, ok := wkldMap[line[hostCol]]; !ok {

			if umwl {
				// Create the network interfaces
				netInterfaces := []*illumioapi.Interface{}
				nic := strings.Split(line[intCol], ";")
				for _, n := range nic {
					x := strings.Split(n, ":")
					if len(x) != 2 {
						utils.Log(0, fmt.Sprintf("CSV line %d - Interface not provided in proper format. Example of proper format is eth1:192.168.100.20. Workload created without an interface.", i))
						continue
					}
					skip := false

					for _, n := range netInterfaces {
						// Skip it if it already is in our array. Put in to account for a GAT export bug.
						if n.Name == x[0] && n.Address == x[1] {
							skip = true
						}
					}
					if !skip {
						netInterfaces = append(netInterfaces, &illumioapi.Interface{Name: x[0], Address: x[1]})
					}

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
				newUMWLs = append(newUMWLs, illumioapi.Workload{Hostname: line[hostCol], Interfaces: netInterfaces, Labels: labels})
				continue

				// If umwl flag is not set, log the entry
			} else {
				utils.Log(0, fmt.Sprintf("%s is not a workload. Include umwl flag to create it. Nothing done.", line[hostCol]))
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
		wkld := wkldMap[line[hostCol]] // Need this since can't perform pointer method on map element
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
				continue
			}

			// If the workload's  value does not equal what's in the CSV
			if labels[i].Value != line[columns[i]] {
				fmt.Printf("Value of label[i].Value: %s\r\n", labels[i].Value)
				fmt.Printf("Value of line[columns[i]]: %s\r\n", line[columns[i]])
				// Change the change flag
				change = true
				// Get the label HREF
				l := checkLabel(illumioapi.Label{Key: keys[i], Value: line[columns[i]]})
				// Add that label to the new labels slice
				newLabels = append(newLabels, &illumioapi.Label{Href: l.Href})
			} else {
				// Keep the existing label if it matches
				newLabels = append(newLabels, &illumioapi.Label{Href: labels[i].Href})
			}
		}

		// If change was flagged, get the workload, update the labels, append to updated slice.
		if change {
			w := wkldMap[line[hostCol]]
			w.Labels = newLabels
			updatedWklds = append(updatedWklds, w)
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
