package subnet

import (
	"log"
	"net"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

type match struct {
	workload illumioapi.Workload
	oldLoc   string
	oldEnv   string
}

func init() {
	SubnetCmd.Flags().StringP("in", "i", "", "Input csv file. The first row (headers) will be skipped.")
	SubnetCmd.MarkFlagRequired("in")
	SubnetCmd.Flags().Bool("auto", false, "Make changes in PCE. Default with output a log file with updates.")
	SubnetCmd.Flags().IntP("net", "n", 1, "Column number with network. First column is 1.")
	SubnetCmd.Flags().IntP("env", "e", 2, "Column number with new env label.")
	SubnetCmd.Flags().IntP("loc", "l", 3, "Column number with new loc label.")

	SubnetCmd.Flags().SortFlags = false

}

// SubnetCmd runs the workload identifier
var SubnetCmd = &cobra.Command{
	Use:   "subnet",
	Short: "Assign environment and location labels based on a workload's network.",
	Long: `
Assign envrionment and location labels based on a workload's network.
	
The workload's first interface's IP address determines the workload's network.

The input CSV requires headers and at least three columns: network, environment label, and location label.

The names of the headers do not matter. If there are additional columns or the columns are not in the default order below, specify the column numbers
using the appropriate flags. Example default:

+----------------+------+-----+
|    Network     | Env  | Loc |
+----------------+------+-----+
| 10.0.0.0/8     | PROD | BOS |
| 192.168.0.0/16 | DEV  | NYC |
+----------------+------+-----+`,
	Run: func(cmd *cobra.Command, args []string) {

		csvFile, _ := cmd.Flags().GetString("in")
		netCol, _ := cmd.Flags().GetInt("net")
		envCol, _ := cmd.Flags().GetInt("env")
		locCol, _ := cmd.Flags().GetInt("loc")
		auto, _ := cmd.Flags().GetBool("auto")

		pce, err := utils.GetPCE("pce.json")
		if err != nil {
			log.Fatalf("Error getting PCE for traffic command - %s", err)
		}

		subnetParser(pce, csvFile, netCol-1, envCol-1, locCol-1, auto)
	},
}

func subnetParser(pce illumioapi.PCE, file string, netCol, envCol, locCol int, auto bool) {

	// Parse the input CSV
	subnetLabels := locParser(file, netCol, envCol, locCol)

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

	// Create a slice to store our results
	updatedWklds := []illumioapi.Workload{}
	matches := []match{}

	// Iterate through workloads
	for _, w := range wklds {
		m := match{}
		changed := false
		// For each workload we need to check the subnets provided in CSV
		for _, nets := range subnetLabels {
			// Check to see if it matches
			if nets.network.Contains(net.ParseIP(w.Interfaces[0].Address)) {
				// Update labels (not in PCE yet, just on object)
				if nets.loc != "" && nets.loc != w.Loc.Value {
					changed = true
					m.oldLoc = w.Loc.Value
					w.ChangeLabel(pce, "loc", nets.loc)
				}
				if nets.env != "" && nets.env != w.Env.Value {
					changed = true
					m.oldEnv = w.Env.Value
					w.ChangeLabel(pce, "env", nets.env)
				}
			}
		}
		if changed == true {
			matches = append(matches, m)
			updatedWklds = append(updatedWklds, w)
		}
	}

	// Bulk update if we have workloads that need updating
	if len(updatedWklds) > 0 {
		csvWriter(pce, matches)
		if auto {
			api, err := illumioapi.BulkWorkload(pce, updatedWklds, "update")
			if err != nil {
				log.Printf("ERROR - bulk updating workloads - %s\r\n", err)
			}
			for _, a := range api {
				log.Println(a.RespBody)
			}
		}
	}
}
