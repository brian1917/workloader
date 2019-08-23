package subnet

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var csvFile string
var netCol, envCol, locCol int
var auto, debug, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

type match struct {
	workload illumioapi.Workload
	oldLoc   string
	oldEnv   string
}

type subnet struct {
	network net.IPNet
	loc     string
	env     string
}

func init() {
	SubnetCmd.Flags().StringVarP(&csvFile, "in", "i", "", "Input csv file. The first row (headers) will be skipped.")
	SubnetCmd.MarkFlagRequired("in")
	SubnetCmd.Flags().BoolVar(&auto, "auto", false, "Make changes in PCE. Default with output a log file with updates.")
	SubnetCmd.Flags().IntVarP(&netCol, "net", "n", 1, "Column number with network. First column is 1.")
	SubnetCmd.Flags().IntVarP(&envCol, "env", "e", 2, "Column number with new env label.")
	SubnetCmd.Flags().IntVarP(&locCol, "loc", "l", 3, "Column number with new loc label.")

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

		pce, err = utils.GetPCE()
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for subnet command - %s", err)
		}

		// Get Viper configuration
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		subnetParser()
	},
}

// locParser used to parse subnet to environment and location labels
func locParser(csvFile string, netCol, envCol, locCol int) []subnet {
	var results []subnet

	// Open CSV File
	file, err := os.Open(csvFile)
	if err != nil {
		utils.Logger.Fatalf("Error opening CSV - %s", err)
	}
	defer file.Close()
	reader := csv.NewReader(bufio.NewReader(file))

	// Start the counter
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
			utils.Logger.Fatalf("Error - reading CSV file - %s", err)
		}

		// Skipe the header row
		if i == 1 {
			continue
		}

		//make sure location label not empty
		if line[locCol] == "" {
			utils.Logger.Fatal("Error - Label field cannot be empty")
		}

		//Place subnet into net.IPNet data structure as part of subnetLabel struct
		_, network, err := net.ParseCIDR(line[netCol])
		if err != nil {
			utils.Logger.Fatal("Error - The Subnet field cannot be parsed.  The format is 10.10.10.0/24")
		}

		//Set struct values
		results = append(results, subnet{network: *network, env: line[envCol], loc: line[locCol]})
	}
	return results
}

func subnetParser() {

	// If debug, log the columns before adjusting by 1
	if debug {
		utils.Log(2, fmt.Sprintf("CSV Columns. Network: %d; Env: %d; Loc: %d", netCol, envCol, locCol))
	}

	// Adjust the columns so they are one less (first column should be 0)
	netCol = netCol - 1
	envCol = envCol - 1
	locCol = locCol - 1

	// Parse the input CSV
	subnetLabels := locParser(csvFile, netCol, envCol, locCol)

	// GetAllWorkloads
	wklds, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloads", a)
	}
	if err != nil {
		utils.Logger.Fatalf("Error getting all workloads - %s", err)
	}

	// GetAllLabels
	labelMap, a, err := pce.GetLabelMapKV()
	if debug {
		utils.LogAPIResp("GetLabelMapKV", a)
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting label map - %s", err))
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
				if nets.loc != "" && nets.loc != w.GetLoc(labelMap).Value {
					changed = true
					m.oldLoc = w.GetLoc(labelMap).Value
					w.ChangeLabel(pce, "loc", nets.loc)
				}
				if nets.env != "" && nets.env != w.GetEnv(labelMap).Value {
					changed = true
					m.oldEnv = w.GetEnv(labelMap).Value
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

		// Create our data slice
		data := [][]string{[]string{"hostname", "href", "ip_address", "role", "app", "original_env", "original_loc", "new_loc", "new_env"}}
		for _, m := range matches {
			data = append(data, []string{m.workload.Hostname, m.workload.Href, m.workload.Interfaces[0].Address, m.workload.GetRole(labelMap).Value, m.workload.GetApp(labelMap).Value, m.oldLoc, m.oldEnv, m.workload.GetLoc(labelMap).Value, m.workload.GetEnv(labelMap).Value})
		}

		utils.WriteOutput(data, data, "workloader-subnet-output-"+time.Now().Format("20060102_150405")+".csv")

		// Print number of workloads requiring update to the terminal
		fmt.Printf("%d workloads requiring label update.\r\n", len(updatedWklds))

		// If updatePCE is disabled, we are just going to alert the user what will happen and log
		if !updatePCE {
			utils.Log(0, fmt.Sprintf("%d workloads requiring mode change.", len(data)-1))
			fmt.Printf("Subnet identified %d workloads requiring label change. To update their labels, run again using --update-pce flag. The --auto flag will bypass the prompt if used with --update-pce.\r\n", len(data)-1)
			utils.Log(0, "completed running subnet command")
			return
		}

		// If updatePCE is set, but not noPrompt, we will prompt the user.
		if updatePCE && !noPrompt {
			var prompt string
			fmt.Printf("Subnet will change the labels of %d workloads. Do you want to run the change (yes/no)? ", len(data)-1)
			fmt.Scanln(&prompt)
			if strings.ToLower(prompt) != "yes" {
				utils.Log(0, fmt.Sprintf("subnet identified %d workloads requiring label change. user denied prompt", len(data)-1))
				fmt.Println("Prompt denied.")
				utils.Log(0, "completed running subnet command")
				return
			}
		}

		// If we get here, user accepted prompt or no-prompt was set.
		api, err := pce.BulkWorkload(updatedWklds, "update")
		if debug {
			for _, a := range api {
				utils.LogAPIResp("BulkWorkloadUpdate", a)
			}
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("running bulk update - %s", err))
		}
		// Log successful run.
		utils.Log(0, fmt.Sprintf("bulk updated %d workloads.", len(updatedWklds)))
		if !debug {
			for _, a := range api {
				utils.Log(0, a.RespBody)
			}
		}
	} else {
		fmt.Println("no workloads identified for label change")
		utils.Log(0, "submet completed running without identifying any workloads requiring change.")
	}
}
