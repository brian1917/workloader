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
	data := [][]string{[]string{"hostname", "name", "href", "online", "os_id", "role", "app", "env", "loc", "status", "interfaces", "public_ip", "ven_version"}}

	// GetAllWorkloads
	wklds, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloads", a)
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
			interfaces = append(interfaces, i.Name+":"+i.Address+"/"+strconv.Itoa(i.CidrBlock))
		}

		// Set VEN version
		if w.Agent == nil || w.Agent.Href == "" {
			venVersion = "Unmanaged"
		} else {
			venVersion = w.Agent.Status.AgentVersion
		}

		// Append to data slice
		data = append(data, []string{w.Hostname, w.Name, w.Href, strconv.FormatBool(w.Online), w.OsID, w.GetRole(labelMap).Value, w.GetApp(labelMap).Value, w.GetEnv(labelMap).Value, w.GetLoc(labelMap).Value, w.GetMode(), strings.Join(interfaces, ";"), w.PublicIP, venVersion})
	}

	if len(data) > 1 {
		fmt.Printf("\r\n%d workloads exported.\r\n", len(data)-1)
		utils.WriteOutput(data, fmt.Sprintf("workloader-export-%s-.csv", time.Now().Format("20060102_150405")))
		utils.Log(0, fmt.Sprintf("export complete - %d workloads exported", len(data)-1))
	} else {
		// Log command execution for 0 results
		fmt.Println("No workloads in PCE.")
		utils.Log(0, "no workloads in PCE.")
	}

}
