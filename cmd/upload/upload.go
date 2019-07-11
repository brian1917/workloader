package upload

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
)

// Global variables
var hostCol, roleCol, appCol, envCol, locCol, intCol int
var removeValue, csvFile string
var umwl bool
var pce illumioapi.PCE
var err error

func init() {
	UploadCmd.Flags().StringVar(&csvFile, "in", "", "Input csv file. The first row (headers) will be skipped.")
	UploadCmd.MarkFlagRequired("in")
	UploadCmd.Flags().StringVar(&removeValue, "removeValue", "", "Value in CSV used to remove existing labels. Blank values in the CSV will not change existing. If you want to delete a label do something like -removeValue delete and use delete in CSV to indicate where to delete.")
	UploadCmd.Flags().BoolVar(&umwl, "umwl", false, "Create unmanaged workloads if the host does not exist")
	UploadCmd.Flags().IntVarP(&hostCol, "hostname", "n", 1, "Column number with hostname. First column is 1.")
	UploadCmd.Flags().IntVarP(&roleCol, "role", "r", 2, "Column number with new role label.")
	UploadCmd.Flags().IntVarP(&appCol, "app", "a", 3, "Column number with new app label.")
	UploadCmd.Flags().IntVarP(&envCol, "env", "e", 4, "Column number with new env label.")
	UploadCmd.Flags().IntVarP(&locCol, "loc", "l", 5, "Column number with new loc label.")
	UploadCmd.Flags().IntVarP(&intCol, "ifaces", "i", 6, "Column number with network interfaces for when creating unmanaged workloads. Each interface should be of the format name:address (e.g., eth1:192.168.200.20). Separate multiple NICs by semicolons.")

	UploadCmd.Flags().SortFlags = false

	// Adjust the columns by one
	hostCol--
	roleCol--
	appCol--
	envCol--
	locCol--
	intCol--

}

// UploadCmd runs the upload command
var UploadCmd = &cobra.Command{
	Use:   "csv",
	Short: "Create and assign labels from a CSV file. Create and label unmanaged workloads from same CSV.",
	Long: `
Create and assign labels from a CSV file. Create and label unmanaged workloads from same CSV.

The default input style is below. Using this command with the --umwl flag will label workloads and create unmanaged workloads.

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

		pce, err = utils.GetPCE("pce.json")
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		processCSV()
	},
}

func checkLabel(label illumioapi.Label, labelMap map[string]illumioapi.Label) (illumioapi.Label, error) {

	// Check if it exists or not
	if _, ok := labelMap[label.Key+label.Value]; ok {
		return labelMap[label.Key+label.Value], nil
	}

	// Create the label if it doesn't exist
	l, _, err := illumioapi.CreateLabel(pce, illumioapi.Label{Key: label.Key, Value: label.Value})
	if err != nil {
		return illumioapi.Label{}, err
	}
	logJSON, _ := json.Marshal(illumioapi.Label{Href: l.Href, Key: label.Key, Value: label.Value})
	utils.Log(0, fmt.Sprintf("created Label - %s", string(logJSON)))

	// Append the label back to the map
	labelMap[l.Key+l.Value] = l

	return l, nil
}

func processCSV() {

	// Log start of the command
	utils.Log(0, "started CSV command")

	// Open CSV File
	file, err := os.Open(csvFile)
	if err != nil {
		utils.Log(1, fmt.Sprintf("opening CSV - %s", err))
	}
	defer file.Close()
	reader := csv.NewReader(bufio.NewReader(file))

	// Get workload hostname map
	wkldMap, err := illumioapi.GetWkldHostMap(pce)
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting workload host map - %s", err))
	}

	// Get label map
	labelMap, err := illumioapi.GetLabelMapKV(pce)
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting label key value map - %s", err))
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

		// Skipe the header row
		if i == 1 {
			continue
		}

		// Check if the workload exists. If exist, we check if UMWL is set and take action.
		if _, ok := wkldMap[line[hostCol]]; !ok {

			if umwl {
				// Create the network interfaces
				netInterfaces := []*illumioapi.Interface{}
				fmt.Println(intCol)
				nic := strings.Split(line[intCol], ";")
				for _, n := range nic {
					x := strings.Split(n, ":")
					if len(x) != 2 {
						utils.Logger.Fatalf("[ERROR] - CSV line %d - Interface not provided in proper format. Example of proper format is eth1:192.168.100.20", i)
					}
					netInterfaces = append(netInterfaces, &illumioapi.Interface{Name: x[0], Address: x[1]})
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
					l, err := checkLabel(illumioapi.Label{Key: keys[i], Value: line[columns[i]]}, labelMap)
					if err != nil {
						utils.Logger.Fatal(err)
					}
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
		labels := []illumioapi.Label{wkld.GetApp(labelMap), wkld.GetRole(labelMap), wkld.GetEnv(labelMap), wkld.GetLoc(labelMap)}
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
				// Change the change flag
				change = true
				// Get the label HREF
				l, err := checkLabel(illumioapi.Label{Key: keys[i], Value: line[columns[i]]}, labelMap)
				if err != nil {
					utils.Log(1, err.Error())
				}
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

	// Bulk update if we have workloads that need updating
	if len(updatedWklds) > 0 {
		api, err := illumioapi.BulkWorkload(pce, updatedWklds, "update")
		if err != nil {
			utils.Log(1, fmt.Sprintf("bulk updating workloads - %s", err))
		}
		for _, a := range api {
			utils.Log(0, a.RespBody)
		}
	}

	// Bulk create if we have new workloads
	if len(newUMWLs) > 0 {
		api, err := illumioapi.BulkWorkload(pce, newUMWLs, "create")
		if err != nil {
			utils.Log(1, fmt.Sprintf("bulk creating workloads - %s", err))
		}
		for _, a := range api {
			utils.Log(0, a.RespBody)
		}
	}

	// Log end
	utils.Log(0, "completed running CSV command.")
}
