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

// Set the Headers
const (
	headerHref        = "href"
	headerEnforcement = "enforcement"
	headerVisibility  = "visibility"
)

// Set global variables for flags
var csvFile string
var useIndividualAPI, legacyPCE, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

// Init handles flags
func init() {
	ModeCmd.Flags().BoolVarP(&useIndividualAPI, "individual-api", "i", false, "Use individual API calls getting workloads from the PCE. This will save time for PCEs with large number of workloads when a small amount is being changed.")
}

// ModeCmd runs the hostname parser
var ModeCmd = &cobra.Command{
	Use:   "mode [csv file with mode info]",
	Short: "Change the state of workloads based on a CSV input.",
	Long: `
Change a workload's state based on an input CSV with at least two columns: workload href and desired state.

VENs can accept the following values: idle, build, test, enforced-no, enforced-low, or enforced-high. The three enforced options include logging (no, low detail, or high).

PCE versions 20.x or more recent can optionally leverage the new workload properties below.
 
CSV input should have at least two columns: href and enforcement.  A third column for visibility is optional. Additional columns will be ignored
 
VENs can accept the following enforcement values: idle, visibility_only, selective, or full.  When setting VEN enforcement to visibility_only the default condition is blocked_allowed. VENs accept the following optional visibility values: off, blocked, blocked_allowed.`,

	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(fmt.Sprintf("getting PCE for mode command - %s", err))
		}

		// Set the hostfile
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get Viper configuration
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		modeUpdate()
	},
}

type target struct {
	href        string
	enforcement string
	visibility  string
}

func parseCsv(filename string) []target {

	// Get PCE Version
	version, api, err := pce.GetVersion()
	utils.LogAPIResp("GetVersion", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	if version.Major < 20 || (version.Major == 20 && version.Minor < 2) {
		legacyPCE = true
	}

	// Create our targets slice to hold results
	var targets []target

	// Open CSV File and create the reader
	file, err := os.Open(filename)
	if err != nil {
		utils.LogError(fmt.Sprintf("opening CSV - %s", err))
	}
	defer file.Close()
	reader := csv.NewReader(utils.ClearBOM(bufio.NewReader(file)))

	// Start the counter
	i := 0

	// Initiate the header map
	csvHeaders := make(map[string]*int)

	// Iterate through CSV entries
	for {

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.LogError(fmt.Sprintf("reading CSV file - %s", err))
		}

		// Increment the counter
		i++

		// Populate headers
		if i == 1 {
			for r, header := range line {
				x := r
				if header == "state" {
					csvHeaders[headerEnforcement] = &x
				} else {
					csvHeaders[header] = &x
				}
			}
			if csvHeaders[headerEnforcement] == nil || csvHeaders[headerHref] == nil {
				utils.LogError("href and enforcement are required headers.")
			}
			continue
		}

		// Check to make sure we have a valid build state and then append to targets slice
		targetMode := strings.ToLower(line[*csvHeaders[headerEnforcement]])
		if legacyPCE {
			if targetMode != "idle" && targetMode != "build" && targetMode != "test" && targetMode != "enforced-no" && targetMode != "enforced-low" && targetMode != "enforced-high" {
				utils.LogError(fmt.Sprintf("csv line %d - invalid mode for a %d.%d pce - %s not acceptable. Values must be idle, build, test, enforced-no, enforced-low, enforced-high", i, version.Major, version.Minor, line[*csvHeaders[headerEnforcement]]))
			}
		} else {
			if targetMode != "idle" && targetMode != "visibility_only" && targetMode != "selective" && targetMode != "full" {
				utils.LogError(fmt.Sprintf("csv line %d - invalid mode for a %d.%d pce - %s not acceptable. Values must be idle, visibility_only, selective, full", i, version.Major, version.Minor, line[*csvHeaders[headerEnforcement]]))
			}
		}

		targetVisibility := ""
		if csvHeaders[headerVisibility] != nil && !legacyPCE {
			targetVisibility = strings.ToLower(line[*csvHeaders[headerVisibility]])
			if targetVisibility != "off" && targetVisibility != "blocked" && targetVisibility != "blocked_allowed" && targetVisibility != "enhanced_data_collection" && targetVisibility != "" {
				utils.LogError(fmt.Sprintf("csv line %d - invalid mode - %s not acceptable. Values must be off, blocked, blocked_allowed, enhanced_data_collection, or a blank value", i, line[*csvHeaders[headerVisibility]]))
			}
			if targetMode != "full" && (targetVisibility != "blocked_allowed" && targetVisibility != "enhanced_data_collection") {
				utils.LogError(fmt.Sprintf("csv line %d - invalid combination - %s visibility and %s enforcement", i, targetVisibility, targetMode))
			}
		}

		targets = append(targets, target{href: line[*csvHeaders[headerHref]], enforcement: targetMode, visibility: targetVisibility})
	}

	return targets
}

func modeUpdate() {

	// Log start of execution
	utils.LogStartCommand("mode")

	// Get targets
	targets := parseCsv(csvFile)

	// Get workloads
	var wklds []illumioapi.Workload
	var a illumioapi.APIResponse

	if useIndividualAPI {
		for i, t := range targets {
			w, a, err := pce.GetWkldByHref(t.href)
			utils.LogAPIResp("GetWkldByHref", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			fmt.Printf("\r%s [INFO] - Getting %d of %d workloads (%d%%) from PCE.", time.Now().Format("2006-01-02 15:04:05 "), i+1, len(targets), (i+1)*100/len(targets))
			wklds = append(wklds, w)
		}
		fmt.Println()
	} else {
		var qp = (map[string]string{"managed": "true"})
		utils.LogInfo("Getting all managed workloads from the PCE. For large deployments and limited number of mode changes, it might be quicker to use the -i flag to run individual API calls to get just workloads that will be changed.", true)
		wklds, a, err = pce.GetWklds(qp)
		utils.LogAPIResp("GetAllWorkloadsQP", a)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Build the map
	wkldMap := make(map[string]illumioapi.Workload)
	for _, w := range wklds {
		wkldMap[w.Href] = w
	}
	utils.LogAPIResp("GetWkldHrefMap", a)

	// Create a slice to hold all the workloads we need to update
	workloadUpdates := []illumioapi.Workload{}

	// Build data slice for writing
	data := [][]string{{"hostname", "href", "role", "app", "env", "loc", "current_enforcement", "target_enforcement", "current_visibility", "target_visibility"}}

	// Enforcement switch is false unless we are moving a workload into enforcement
	enforceCount := 0

	// Cycle through each entry in the CSV
	for _, t := range targets {

		// Check if the mode matches the target mode
		if w, ok := wkldMap[t.href]; ok {
			update := false
			currentEnforcement := ""
			currentVisibility := ""
			// Enforcement
			if w.GetMode() != t.enforcement && t.enforcement != "" {
				utils.LogInfo(fmt.Sprintf("required change - %s - current enforcement: %s - desired enforcement: %s", w.Hostname, w.GetMode(), t.enforcement), false)
				update = true
				currentEnforcement = w.GetMode()
				if err := w.SetMode(t.enforcement); err != nil {
					utils.LogError(fmt.Sprintf("error setting enforcemment - %s", err))
				}
			}
			// Visibility
			if !legacyPCE && (w.GetVisibilityLevel() != t.visibility) && t.visibility != "" {
				utils.LogInfo(fmt.Sprintf("required change - %s - current visibility: %s - desired visibility: %s", w.Hostname, w.GetVisibilityLevel(), t.visibility), false)
				update = true
				currentVisibility = w.GetVisibilityLevel()
				if err := w.SetVisibilityLevel(t.visibility); err != nil {
					utils.LogError(fmt.Sprintf("error setting visibility - %s", err))
				}
			}
			if update {
				data = append(data, []string{w.Hostname, w.Href, w.GetRole(pce.Labels).Value, w.GetApp(pce.Labels).Value, w.GetEnv(pce.Labels).Value, w.GetLoc(pce.Labels).Value, currentEnforcement, w.GetMode(), currentVisibility, w.GetVisibilityLevel()})
				workloadUpdates = append(workloadUpdates, w)
				// Check if we are going into enforcement
				if t.enforcement == "enforced-no" || t.enforcement == "enforced-low" || t.enforcement == "enforced-high" || t.enforcement == "full" || t.enforcement == "selective" {
					enforceCount++
				}
			}

		} else {
			utils.LogWarning(fmt.Sprintf("%s is not a managed workload in the PCE", t.href), true)
		}
	}

	// Process output
	if len(workloadUpdates) == 0 {
		fmt.Println("0 workloads requiring state update.")
	}

	if len(workloadUpdates) > 0 {
		utils.WriteOutput(data, data, fmt.Sprintf("workloader-mode-%s.csv", time.Now().Format("20060102_150405")))
		utils.LogInfo(fmt.Sprintf("%d workloads requiring state update.", len(data)-1), true)

		// If updatePCE is disabled, we are just going to alert the user what will happen and log
		if !updatePCE {
			utils.LogInfo(fmt.Sprintf("workloader identified %d workloads requiring mode change in %s (%s). To update their modes, run again using --update-pce flag. The --no-prompt flag will bypass the prompt if used with --update-pce.", len(data)-1, pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string)), true)
			utils.LogEndCommand("mode")
			return
		}

		// If updatePCE is set, but not noPrompt, we will prompt the user.
		if updatePCE && !noPrompt {
			var prompt string
			fmt.Printf("\r\n%s [PROMPT] - workloader will change the state of %d workloads. Do you want to run the change (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), len(data)-1)
			fmt.Scanln(&prompt)
			if strings.ToLower(prompt) != "yes" {
				utils.LogInfo(fmt.Sprintf("prompt denied to change mode for %d workloads.", len(data)-1), true)
				utils.LogEndCommand("mode")
				return
			}

			if enforceCount > 0 {
				fmt.Printf("\r\n%s [PROMPT] - this mode change includes changing %d workloads into a new enforcement state. Please type \"enforce\" to confirm you want to continue: ", time.Now().Format("2006-01-02 15:04:05 "), enforceCount)
				fmt.Scanln(&prompt)
				fmt.Println()
				if strings.ToLower(prompt) != "enforce" {
					utils.LogInfo(fmt.Sprintf("prompt denied to change mode for %d workloads.", len(data)-1), true)
					utils.LogEndCommand("mode")
					return
				}
			}

		}

		// If we get here, user accepted prompt or no-prompt was set.
		api, err := pce.BulkWorkload(workloadUpdates, "update", true)
		for _, a := range api {
			utils.LogAPIResp("BulkWorkloadUpdate", a)
			for _, w := range a.Warnings {
				utils.LogWarning(w, true)
			}
		}
		if err != nil {
			utils.LogError(fmt.Sprintf("running bulk update - %s", err))
		}
		// Log successful run.
		utils.LogInfo(fmt.Sprintf("bulk updated %d workloads. API Responses:", len(workloadUpdates)), false)
		for _, a := range api {
			utils.LogInfo(a.RespBody, false)

		}
	}

	// Print completion to the terminal
	utils.LogInfo(fmt.Sprintf("%d workloads mode updated. See workloader.log for details.", len(workloadUpdates)), true)
	utils.LogEndCommand("mode")
}
