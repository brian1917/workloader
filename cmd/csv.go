package cmd

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

func checkLabel(label illumioapi.Label, labelMap map[string]illumioapi.Label) (illumioapi.Label, error) {

	// Check if it exists or not
	if _, ok := labelMap[label.Key+label.Value]; ok {
		return labelMap[label.Key+label.Value], nil
	}

	// Get PCE
	pce, err := utils.GetPCE("pce.json")
	if err != nil {
		return illumioapi.Label{}, err
	}

	// Create the label
	l, _, err := illumioapi.CreateLabel(pce, illumioapi.Label{Key: label.Key, Value: label.Value})
	if err != nil {
		return illumioapi.Label{}, err
	}
	logJSON, _ := json.Marshal(illumioapi.Label{Href: l.Href, Key: label.Key, Value: label.Value})
	log.Printf("INFO - Created Label - %s", string(logJSON))

	// Append the label back to the map
	labelMap[l.Key+l.Value] = l

	return l, nil
}

func init() {
	csvCmd.Flags().String("in", "", "Input csv file. The first row (headers) will be skipped.")
	csvCmd.MarkFlagRequired("in")
	csvCmd.Flags().String("removeValue", "", "Value in CSV used to remove existing labels. Blank values in the CSV will not change existing. If you want to delete a label do something like -removeValue delete and use delete in CSV to indicate where to delete.")
	csvCmd.Flags().Bool("umwl", false, "Create unmanaged workloads if the host does not exist")
	csvCmd.Flags().IntP("hostname", "n", 1, "Column number with hostname. First column is 1.")
	csvCmd.Flags().IntP("role", "r", 2, "Column number with new role label.")
	csvCmd.Flags().IntP("app", "a", 3, "Column number with new app label.")
	csvCmd.Flags().IntP("env", "e", 4, "Column number with new env label.")
	csvCmd.Flags().IntP("loc", "l", 5, "Column number with new loc label.")
	csvCmd.Flags().IntP("ifaces", "i", 6, "Column number with network interfaces for when creating unmanaged workloads. Each interface should be of the format name:address (e.g., eth1:192.168.200.20). Separate multiple NICs by semicolons.")

	csvCmd.Flags().SortFlags = false

}

// TrafficCmd runs the workload identifier
var csvCmd = &cobra.Command{
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

		csvFile, _ := cmd.Flags().GetString("in")
		umwl, _ := cmd.Flags().GetBool("umwl")
		hostCol, _ := cmd.Flags().GetInt("hostname")
		removeValue, _ := cmd.Flags().GetString("removeValue")
		roleCol, _ := cmd.Flags().GetInt("role")
		appCol, _ := cmd.Flags().GetInt("app")
		envCol, _ := cmd.Flags().GetInt("env")
		locCol, _ := cmd.Flags().GetInt("loc")
		intCol, _ := cmd.Flags().GetInt("ifaces")

		pce, err := utils.GetPCE("pce.json")
		if err != nil {
			log.Fatalf("Error getting PCE for traffic command - %s", err)
		}

		processCSV(pce, csvFile, hostCol-1, roleCol-1, appCol-1, envCol-1, locCol-1, intCol-1, removeValue, umwl)
	},
}

func processCSV(pce illumioapi.PCE, csvFile string, hostCol, roleCol, appCol, envCol, locCol, intCol int, removeValue string, umwl bool) {

	// Open CSV File
	file, err := os.Open(csvFile)
	if err != nil {
		log.Fatalf("Error opening CSV - %s", err)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))

	// GetAllWorkloads
	wklds, _, err := illumioapi.GetAllWorkloads(pce)
	if err != nil {
		log.Fatalf("Error getting all workloads - %s", err)
	}
	wkldMap := make(map[string]illumioapi.Workload)
	for _, w := range wklds {
		wkldMap[w.Hostname] = w
	}

	// GetAllLabels
	labels, _, err := illumioapi.GetAllLabels(pce)
	if err != nil {
		log.Fatalf("Error getting all labels - %s", err)
	}
	labelMap := make(map[string]illumioapi.Label)
	for _, l := range labels {
		labelMap[l.Key+l.Value] = l
	}

	// Create a slice to hold the workloads we will update and create
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
		} else if err != nil {
			log.Fatalf("Error - reading CSV file - %s", err)
		}

		// Skipe the header row
		if i == 1 {
			continue
		}

		// Check if the workload exists
		if _, ok := wkldMap[line[hostCol]]; !ok {

			if umwl {
				// Create the network interfaces
				netInterfaces := []*illumioapi.Interface{}
				fmt.Println(intCol)
				nic := strings.Split(line[intCol], ";")
				for _, n := range nic {
					x := strings.Split(n, ":")
					if len(x) != 2 {
						log.Fatalf("ERROR - CSV line %d - Interface not provided in proper format. Example of proper format is eth1:192.168.100.20", i)
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
						log.Fatal(err)
					}
					// Add that label to the new labels slice
					labels = append(labels, &illumioapi.Label{Href: l.Href})
				}

				// Add to the slice to process via bulk
				newUMWLs = append(newUMWLs, illumioapi.Workload{Hostname: line[hostCol], Interfaces: netInterfaces, Labels: labels})

				// Once the WL is in the new slice, we can go to the next CSV entry
				continue

				// If umwl flag is not set, log the entry
			} else {
				log.Printf("INFO - %s is not a workload. Include umwl flag to create it. Nothing done.", line[hostCol])
				continue
			}
		}

		// Create a slice told hold new labels if we need to change them
		newLabels := []*illumioapi.Label{}

		// Initialize the change variable
		change := false

		// Set slices to iterate through the 4 keys
		columns := []int{appCol, roleCol, envCol, locCol}
		labels := []illumioapi.Label{wkldMap[line[hostCol]].App, wkldMap[line[hostCol]].Role, wkldMap[line[hostCol]].Env, wkldMap[line[hostCol]].Loc}
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
					log.Fatal(err)
				}
				// Add that label to the new labels slice
				newLabels = append(newLabels, &illumioapi.Label{Href: l.Href})
			} else {
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
			log.Printf("ERROR - bulk updating workloads - %s\r\n", err)
		}
		for _, a := range api {
			log.Println(a.RespBody)
		}
	}

	// Bulk create if we have new workloads
	if len(newUMWLs) > 0 {
		api, err := illumioapi.BulkWorkload(pce, newUMWLs, "create")
		if err != nil {
			log.Printf("ERROR - bulk creating workloads - %s\r\n", err)
		}
		for _, a := range api {
			log.Println(a.RespBody)
		}
	}
}
