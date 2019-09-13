package compatibility

import (
	"fmt"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debug bool
var pce illumioapi.PCE
var err error

// CompatibilityCmd runs the workload identifier
var CompatibilityCmd = &cobra.Command{
	Use:   "compatibility",
	Short: "Generate a compatibility report for all Idle workloads.",
	Long: `
Generate a compatibility report for all Idle workloads. The update-pce and auto flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetPCE(false)
		if err != nil {
			utils.Log(1, err.Error())
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		compatibilityReport()
	},
}

func compatibilityReport() {

	// Log command
	utils.Log(0, "running compatability command")

	// Start the data slice with the headers. We will append data to this.
	var csvData, stdOutData [][]string
	csvData = append(csvData, []string{"hostname", "href", "status", "raw_data"})
	stdOutData = append(stdOutData, []string{"hostname", "href", "status"})

	// Get all workloads
	wklds, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloadsH", a)
	}
	if err != nil {
		utils.Log(1, err.Error())
	}

	// Iterate through each workload
	for _, w := range wklds {

		// Skip if it's not in idle
		if w.Agent.Config.Mode != "idle" {
			continue
		}

		// Get the compatibility report and append
		cr, a, err := pce.GetCompatibilityReport(w)
		if debug {
			utils.LogAPIResp("GetCompatibilityReport", a)
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("getting compatibility report for %s (%s) - %s", w.Hostname, w.Href, err))
		}

		csvData = append(csvData, []string{w.Hostname, w.Href, cr.QualifyStatus, a.RespBody})

		stdOutData = append(stdOutData, []string{w.Hostname, w.Href, cr.QualifyStatus})
	}

	// If the CSV data has more than just the headers, create output file and write it.
	if len(csvData) > 1 {
		utils.WriteOutput(csvData, stdOutData, fmt.Sprintf("workloader-compatibility-%s.csv", time.Now().Format("20060102_150405")))
		fmt.Println("Note - CSV will have verbose information on tests for yellow/red status")
		fmt.Printf("\r\n%d compatibility reports exported.\r\n", len(csvData)-1)
		utils.Log(0, fmt.Sprintf("export complete - %d workloads exported", len(csvData)-1))
	} else {
		// Log command execution for 0 results
		fmt.Println("No workloads in idle mode.")
		utils.Log(0, "no workloads in idle mode.")
	}

}
