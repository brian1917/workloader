package mode

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var csvFile string
var verbose, debug, updatePCE, noPrompt bool
var hrefCol, desiredStateCol int
var pce illumioapi.PCE
var err error

// Init handles flags
func init() {

	ModeCmd.Flags().IntVar(&hrefCol, "hrefCol", 1, "Column number with href value. First column is 1.")
	ModeCmd.Flags().IntVar(&desiredStateCol, "stateCol", 2, "Column number with desired state value.")
	ModeCmd.Flags().SortFlags = false

}

// ModeCmd runs the hostname parser
var ModeCmd = &cobra.Command{
	Use:   "mode [csv file with mode info]",
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

	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetPCE()
		if err != nil {
			utils.Log(1, fmt.Sprintf("getting PCE for mode command - %s", err))
		}

		// Set the hostfile
		csvFile = args[0]

		// Get Viper configuration
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		modeUpdate()
	},
}

type target struct {
	workloadHref string
	targetMode   string
}

func parseCsv(filename string) []target {

	// If debug, log the columns before adjusting by 1
	if debug {
		utils.Log(2, fmt.Sprintf("CSV Columns. Href: %d; DesiredState: %d", hrefCol, desiredStateCol))
	}

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

	// Build a map of all managed workloads
	wkldMap, a, err := pce.GetWkldHrefMap()
	if debug {
		utils.LogAPIResp("GetWkldHrefMap", a)
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("error getting workload map - %s", err))
	}

	// Get labelMap
	labelMap, a, err := pce.GetLabelMapH()
	if debug {
		utils.LogAPIResp("GetLabelMapH", a)
	}
	if err != nil {
		utils.Log(1, err.Error())
	}

	// Get targets
	targets := parseCsv(csvFile)

	// Create a slice to hold all the workloads we need to update
	workloadUpdates := []illumioapi.Workload{}

	// Build data slice for writing
	data := [][]string{[]string{"hostname", "href", "role", "app", "env", "loc", "current_mode", "target_mode"}}

	// Cycle through each entry in the CSV
	for _, t := range targets {

		// Check if the mode matches the target mode
		w := wkldMap[t.workloadHref]
		if w.GetMode() != t.targetMode && t.targetMode != "unmanaged" {
			// Log the change is needed
			utils.Log(0, fmt.Sprintf("required Change - %s - current state: %s - desired state: %s\r\n", w.Hostname, w.GetMode(), t.targetMode))
			data = append(data, []string{w.Hostname, w.Href, w.GetRole(labelMap).Value, w.GetApp(labelMap).Value, w.GetEnv(labelMap).Value, w.GetLoc(labelMap).Value, w.GetMode(), t.targetMode})
			// Copy workload with the right target mode and append to slice
			if err := w.SetMode(t.targetMode); err != nil {
				utils.Log(1, fmt.Sprintf("error setting mode - %s", err))
			}
			workloadUpdates = append(workloadUpdates, w)
		}
	}

	// Process output
	if len(workloadUpdates) == 0 {
		fmt.Println("0 workloads requiring state update.")
	}

	if len(workloadUpdates) > 0 {
		utils.WriteOutput(data, data, fmt.Sprintf("workloader-mode-%s.csv", time.Now().Format("20060102_150405")))
		fmt.Printf("%d workloads requiring state update.\r\n", len(data)-1)

		// If updatePCE is disabled, we are just going to alert the user what will happen and log
		if !updatePCE {
			utils.Log(0, fmt.Sprintf("%d workloads requiring mode change.", len(data)-1))
			fmt.Printf("Mode identified %d workloads requiring mode change. To update their modes, run again using --update-pce flag. The --auto flag will bypass the prompt if used with --update-pce.\r\n", len(data)-1)
			utils.Log(0, "completed running mode command")
			return
		}

		// If updatePCE is set, but not noPrompt, we will prompt the user.
		if updatePCE && !noPrompt {
			var prompt string
			fmt.Printf("Mode will change the state of %d workloads. Do you want to run the change (yes/no)? ", len(data)-1)
			fmt.Scanln(&prompt)
			if strings.ToLower(prompt) != "yes" {
				utils.Log(0, fmt.Sprintf("mode identified %d workloads requiring mode change. user denied prompt", len(data)-1))
				fmt.Println("Prompt denied.")
				utils.Log(0, "completed running mode command")
				return
			}
		}

		// If we get here, user accepted prompt or no-prompt was set.
		api, err := pce.BulkWorkload(workloadUpdates, "update")
		if debug {
			for _, a := range api {
				utils.LogAPIResp("BulkWorkloadUpdate", a)
			}
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("running bulk update - %s", err))
		}
		// Log successful run.
		utils.Log(0, fmt.Sprintf("bulk updated %d workloads. API Responses:", len(workloadUpdates)))
		if !debug {
			for _, a := range api {
				utils.Log(0, a.RespBody)
			}
		}
	}

	// Print completion to the terminal
	fmt.Printf("%d workloads mode updated. See workloader.log for details.\r\n", len(workloadUpdates))
}
