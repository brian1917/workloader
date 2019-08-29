package export

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug bool
var outFormat string

// ExportCmd runs the workload identifier
var ExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Create a CSV export of all workloads in the PCE.",
	Long: `
Create a CSV export of all workloads in the PCE. The update-pce and auto flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetPCE()
		if err != nil {
			utils.Log(1, err.Error())
		}

		// Get the viper values
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		exportWorkloads()
	},
}

func exportWorkloads() {

	// Log command execution
	utils.Log(0, "running export command")

	// Start the data slice with headers
	csvData := [][]string{[]string{"hostname", "href", "role", "app", "env", "loc", "interfaces", "name", "public_ip", "mode", "online", "os_id", "ven_version"}}
	stdOutData := [][]string{[]string{"hostname", "role", "app", "env", "loc", "interfaces", "mode", "os_id", "ven_version"}}

	// GetAllWorkloads
	wklds, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloads", a)
		// Logging code added to debug specific issue
		for _, w := range wklds {
			utils.Log(2, fmt.Sprintf("%s mode: %s", w.Hostname, w.GetMode()))
			utils.Log(2, fmt.Sprintf("%s Agent Href: %s", w.Hostname, w.Agent.Href))
			utils.Log(2, fmt.Sprintf("%s Agent Mode: %s", w.Hostname, w.Agent.Config.Mode))
		}
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting all workloads - %s", err))
	}

	// Get LabelMap
	labelMap, a, err := pce.GetLabelMapH()
	if debug {
		utils.LogAPIResp("GetLabelMapH", a)
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting label map - %s", err))
	}

	for _, w := range wklds {

		// Skip deleted workloads
		if *w.Deleted {
			continue
		}

		// Set up variables
		interfaces := []string{}
		venVersion := ""

		// Get interfaces
		for _, i := range w.Interfaces {
			ipAddress := fmt.Sprintf("%s:%s", i.Name, i.Address)
			if i.CidrBlock != nil {
				ipAddress = fmt.Sprintf("%s:%s/%s", i.Name, i.Address, strconv.Itoa(*i.CidrBlock))
			}
			interfaces = append(interfaces, ipAddress)
		}

		// Set VEN version
		if w.Agent == nil || w.Agent.Href == "" {
			venVersion = "unmanaged"
		} else {
			venVersion = w.Agent.Status.AgentVersion
		}

		// Append to data slice
		csvData = append(csvData, []string{w.Hostname, w.Href, w.GetRole(labelMap).Value, w.GetApp(labelMap).Value, w.GetEnv(labelMap).Value, w.GetLoc(labelMap).Value, strings.Join(interfaces, ";"), w.Name, w.PublicIP, w.GetMode(), strconv.FormatBool(w.Online), w.OsID, venVersion})
		stdOutData = append(stdOutData, []string{w.Hostname, w.GetRole(labelMap).Value, w.GetApp(labelMap).Value, w.GetEnv(labelMap).Value, w.GetLoc(labelMap).Value, strings.Join(interfaces, ";"), w.GetMode(), w.OsID, venVersion})
	}

	if len(csvData) > 1 {
		utils.WriteOutput(csvData, stdOutData, fmt.Sprintf("workloader-export-%s.csv", time.Now().Format("20060102_150405")))
		fmt.Printf("\r\n%d workloads exported.\r\n", len(csvData)-1)
		fmt.Println("Note - the CSV export will include 4 additional columns: href, name, PublicIP, and Online")
		utils.Log(0, fmt.Sprintf("export complete - %d workloads exported", len(csvData)-1))
	} else {
		// Log command execution for 0 results
		fmt.Println("No workloads in PCE.")
		utils.Log(0, "no workloads in PCE.")
	}

}
