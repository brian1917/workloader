package mode

import (
	"fmt"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Set global variables for flags
var hostFile string
var logOnly, verbose bool
var hrefCol, desiredStateCol int
var pce illumioapi.PCE
var err error

// Init handles flags
func init() {

	ModeCmd.Flags().StringVarP(&hostFile, "input", "i", "", "Input CSV file.")
	ModeCmd.Flags().IntVar(&hrefCol, "hrefCol", 1, "Column number with href value. First column is 1.")
	ModeCmd.Flags().IntVar(&desiredStateCol, "stateCol", 2, "Column number with desired state value.")
	ModeCmd.Flags().BoolVarP(&logOnly, "logonly", "l", false, "Will not make changes in PCE.")
	ModeCmd.Flags().SortFlags = false

}

// ModeCmd runs the hostname parser
var ModeCmd = &cobra.Command{
	Use:   "mode",
	Short: "Change the state of workloads based on a CSV input.",
	Long: `
Change a workload's state based on an input CSV with at least two columns: workload href and desired state.

The state must be either idle, build, test, or enforced.

An example is below:

+--------------------------------------------------------+----------+
|                          href                          |  state   |
+--------------------------------------------------------+----------+
| /orgs/1/workloads/721d1621-31a6-40a0-a0cb-1e4b1c051210 | build    |
| /orgs/1/workloads/d1e6266c-0b07-4b6e-b68f-c3f2386bdf08 | test     |
| /orgs/1/workloads/77d72edc-8734-4a5d-a01d-d055898e6ba1 | enforced |
+--------------------------------------------------------+----------+

Use --hrefCol and --stateCol to specify the columns if not default (href=1, state=2). Additional columns will be ignored.`,

	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetPCE("pce.json")
		if err != nil {
			fmt.Println("Error - see workloader.log file")
			utils.Logger.Fatalf("[ERROR] - getting PCE for mode command - %s", err)
		}

		modeUpdate()
	},
}

func modeUpdate() {

	// Log start of execution
	utils.Logger.Println("[INFO] - running mode command")

	// Log the logonly mode
	utils.Logger.Printf("[INFO] - Log only mode set to %t", logOnly)

	// Get all managed workloads
	wklds, _, err := illumioapi.GetAllWorkloads(pce)
	if err != nil {
		fmt.Println("Error - see workloader.log file")
		utils.Logger.Fatalf("[ERROR] - getting all workloads - %s", err)
	}

	// Build a map of all managed workloads
	managedWklds := make(map[string]illumioapi.Workload)
	for _, w := range wklds {
		if w.Agent != nil {
			managedWklds[w.Href] = w
		}
	}

	// Get targets
	targets := parseCsv(hostFile)

	// Create a slice to hold all the workloads we need to update
	workloadUpdates := []illumioapi.Workload{}

	// Cycle through each entry in the CSV
	for _, t := range targets {
		// Get the current mode
		var mode string
		if managedWklds[t.workloadHref].Agent.Config.Mode == "illuminated" && !managedWklds[t.workloadHref].Agent.Config.LogTraffic {
			mode = "build"
		} else if managedWklds[t.workloadHref].Agent.Config.Mode == "illuminated" && managedWklds[t.workloadHref].Agent.Config.LogTraffic {
			mode = "test"
		} else {
			mode = managedWklds[t.workloadHref].Agent.Config.Mode
		}
		// Check if the current mode is NOT the target mode
		if mode != t.targetMode {
			// Log the change is needed
			utils.Logger.Printf("[INFO] - Required Change - %s - Current state: %s - Desired state: %s\r\n", managedWklds[t.workloadHref].Hostname, mode, t.targetMode)

			// Copy workload with the right target mode and append to slice
			w := managedWklds[t.workloadHref]
			if t.targetMode == "build" {
				w.Agent.Config.Mode = "illuminated"
				w.Agent.Config.LogTraffic = false
			} else if t.targetMode == "test" {
				w.Agent.Config.Mode = "illuminated"
				w.Agent.Config.LogTraffic = true
			} else {
				w.Agent.Config.Mode = t.targetMode
			}
			workloadUpdates = append(workloadUpdates, w)
		}
	}

	// Print number requiring updates to the terminal
	fmt.Printf("%d workloads requiring state update. See workloader.log for details.\r\n", len(workloadUpdates))

	// Bulk update the workloads if we have some
	if len(workloadUpdates) > 0 && !logOnly {
		api, err := illumioapi.BulkWorkload(pce, workloadUpdates, "update")
		if err != nil {
			fmt.Println("Error - see workloader.log file")
			utils.Logger.Fatalf("[ERROR] - running bulk update - %s", err)
		}
		utils.Logger.Println("[INFO] - API Responses:")
		for _, a := range api {
			utils.Logger.Printf(a.RespBody)
		}
	}

	// Print completion to the terminal
	fmt.Printf("%d workloads updated. See workloader.log for details.\r\n", len(workloadUpdates))
}
