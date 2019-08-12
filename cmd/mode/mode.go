package mode

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

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
		pce, err = utils.GetPCE()
		if err != nil {
			utils.Log(1, fmt.Sprintf("getting PCE for mode command - %s", err))
		}

		modeUpdate()
	},
}

type target struct {
	workloadHref string
	targetMode   string
}

func parseCsv(filename string) []target {
	// Adjust the columns so first column is  0
	hrefCol--
	desiredStateCol--

	// Create our targets slice to hold results
	var targets []target

	// Open CSV File and create the reader
	file, err := os.Open(filename)
	if err != nil {
		utils.Log(1, fmt.Sprintf("opening CSV - %s", err))
	}
	defer file.Close()
	reader := csv.NewReader(utils.ClearBOM(bufio.NewReader(file)))

	// Start the counter
	i := 0

	// Iterate through CSV entries
	for {

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("reading CSV file - %s", err))
		}

		// Increment the counter
		i++

		// Skipe the header row
		if i == 1 {
			continue
		}

		// Check to make sure we have a valid build state and then append to targets slice
		m := strings.ToLower(line[desiredStateCol])
		if m != "idle" && m != "build" && m != "test" && m != "enforced" {
			utils.Log(1, fmt.Sprintf("invalid mode on line %d - %s not acceptable", i, line[desiredStateCol]))
		}
		targets = append(targets, target{workloadHref: line[hrefCol], targetMode: m})
	}

	return targets
}

func modeUpdate() {

	// Log start of execution
	utils.Log(0, "running mode command")

	// Log the logonly mode
	utils.Log(0, fmt.Sprintf("log only mode set to %t", logOnly))

	// Build a map of all managed workloads
	wkldMap, err := pce.GetWkldHrefMap()
	if err != nil {
		utils.Log(1, fmt.Sprintf("error getting workload map - %s", err))
	}

	// Get targets
	targets := parseCsv(hostFile)

	// Create a slice to hold all the workloads we need to update
	workloadUpdates := []illumioapi.Workload{}

	// Cycle through each entry in the CSV
	for _, t := range targets {

		// Check if the mode matches the target mode
		w := wkldMap[t.workloadHref]
		if w.GetMode() != t.targetMode && t.targetMode != "unmanaged" {
			// Log the change is needed
			utils.Log(0, fmt.Sprintf("required Change - %s - current state: %s - desired state: %s\r\n", w.Hostname, w.GetMode(), t.targetMode))
			// Copy workload with the right target mode and append to slice
			if err := w.SetMode(t.targetMode); err != nil {
				utils.Log(1, fmt.Sprintf("error setting mode - %s", err))
			}
			workloadUpdates = append(workloadUpdates, w)
		}
	}

	// Print number requiring updates to the terminal
	fmt.Printf("%d workloads requiring state update. See workloader.log for details.\r\n", len(workloadUpdates))

	// Bulk update the workloads if we have some
	if len(workloadUpdates) > 0 && !logOnly {
		api, err := pce.BulkWorkload(workloadUpdates, "update")
		if err != nil {
			utils.Log(1, fmt.Sprintf("running bulk update - %s", err))
		}
		utils.Log(0, fmt.Sprintf("bulk updated %d workloads. API Responses:", len(workloadUpdates)))
		for _, a := range api {
			utils.Log(0, a.RespBody)
		}
	}

	// Print completion to the terminal
	fmt.Printf("%d workloads updated. See workloader.log for details.\r\n", len(workloadUpdates))
}
