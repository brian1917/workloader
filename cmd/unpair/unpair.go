package unpair

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var hrefFile, role, app, env, loc, restore, outputFileName string
var updatePCE, noPrompt, setLabelExcl, includeOnline, singleGetWkld, singleUnpair bool
var hoursSinceLastHB int
var pce illumioapi.PCE
var err error

// Init handles flags
func init() {
	UnpairCmd.Flags().StringVar(&restore, "restore", "saved", "Restore value. Must be saved, default, or disable.")
	UnpairCmd.Flags().StringVarP(&hrefFile, "href", "f", "", "Location of file with HREFs to be used instead of starting with all workloads.")
	UnpairCmd.Flags().BoolVar(&singleGetWkld, "single-get-wkld", false, "get workloads in a host file by a single API call vs. bulk API.")
	UnpairCmd.Flags().StringVarP(&role, "role", "r", "", "Role Label. Blank means all roles.")
	UnpairCmd.Flags().StringVarP(&app, "app", "a", "", "Application Label. Blank means all applications.")
	UnpairCmd.Flags().StringVarP(&env, "env", "e", "", "Environment Label. Blank means all environments.")
	UnpairCmd.Flags().StringVarP(&loc, "loc", "l", "", "Location Label. Blank means all locations.")
	UnpairCmd.Flags().BoolVarP(&setLabelExcl, "exclude-labels", "x", false, "Use provided label filters as excludes.")
	UnpairCmd.Flags().IntVar(&hoursSinceLastHB, "hours", 0, "Hours since last heartbeat. No value (i.e., 0) will ignore heartbeats.")
	UnpairCmd.Flags().BoolVar(&includeOnline, "include-online", false, "Include workloads that are online. By default only offline workloads that meet criteria will be unpaired.")
	UnpairCmd.Flags().BoolVar(&singleUnpair, "single-unpair", false, "One API call per unpair versus one API call per 1000 workloads. This will be significantly slower but provide more details in the PCE's syslog messages.")
	UnpairCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	UnpairCmd.Flags().SortFlags = false
}

// UnpairCmd runs the unpair
var UnpairCmd = &cobra.Command{
	Use:   "unpair",
	Short: "Unpair workloads through an input file or by a combination of labels and hours since last heartbeat.",

	Long: `  
Unpair workloads through an input file or by combination of labels and hours since last heartbeat.

Default output is a CSV file with what would be unpaired.
Use the --update-pce command to run the unpair with a user prompt confirmation.
Use --update-pce and --no-prompt to run unpair with no prompts.`,

	Example: `# Unpair all workloads that have not had a heart beat in 50 hours with no user prompt (e.g., command to run on cron):
  workloader unpair --hours 50 --restore saved --update-pce --no-prompt

  # Unpair workloads in ERP application in Production that have not had a heartbeat for 40 hours with no prompt (e.g., command to run on cron).
  workloader unpair --hours 40 --app ERP --env PROD --restore saved --update-pce --no-prompt

  # See what workloads would unpair if we set the threshold for 24 hours for all labels:
  workloader unpair --hours 24 --restore saved
 `,
	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get persistent flags from Viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		unpair()
	},
}

func unpair() {

	utils.LogStartCommand("unpair")

	// Check that we aren't unpairing the whole PCE
	if app == "" && role == "" && env == "" && loc == "" && hoursSinceLastHB == 0 && hrefFile == "" {
		utils.LogError("must provide labels, hours, or an input file.")
	}

	// Check the restore value
	restore = strings.ToLower(restore)
	if restore != "saved" && restore != "default" && restore != "disable" {
		utils.LogError("restore value must be saved, default, or disable.")
	}

	// Check invalid flag combinations
	if singleGetWkld && hrefFile == "" {
		utils.LogError("single-get-wkld flag requires an href file")
	}

	// If we have an hrefFile, process it
	var hrefFileData [][]string
	if hrefFile != "" {
		hrefFileData, err = utils.ParseCSV(hrefFile)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Set workload variables
	var wklds, targetWklds []illumioapi.Workload
	csvWklds := make(map[string]bool)

	// Process singleGetWkld - we know href file is provided because already checked above
	if singleGetWkld {
		// Iterate through file
		for rowIndex, row := range hrefFileData {
			// If singleGetWkld, make api call
			if strings.Contains(row[0], "/orgs/") {
				wkld, a, _ := pce.GetWkldByHref(row[0])
				utils.LogAPIResp("GetWkldByHref", a)
				// If the status code is in the 400s log it and skip
				if a.StatusCode >= 400 && a.StatusCode <= 499 {
					utils.LogWarning(fmt.Sprintf("href file line %d - %s is not a workload - %d status code. skipping.", rowIndex+1, row[0], a.StatusCode), true)
					// If the status code is not in the 400s and there is an error, error out
				} else if err != nil {
					utils.LogError(err.Error())
					// No error, then add to workloads slice
				} else {
					wklds = append(wklds, wkld)
					utils.LogInfo(fmt.Sprintf("href file line %d of %d - %s - status code: %d", rowIndex+1, len(hrefFileData), row[0], a.StatusCode), true)
				}
			}
		}
	}

	// Get all managed workloads if single-get-wkld is false
	if !singleGetWkld {
		// Fill csv map in case needed
		for _, row := range hrefFileData {
			// Add to csvWklds map
			csvWklds[row[0]] = true
		}

		// Get all managed workloads
		allManagedWklds, a, err := pce.GetWklds(map[string]string{"managed": "true"})
		utils.LogAPIResp("GetAllWorkloads", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		// If there is an href file, iterate through the hrefData and check if the workloads exists.
		if hrefFile != "" {
			for rowIndex, row := range hrefFileData {
				// If it does not exist, warn (if it is a valid href)
				if val, exists := pce.Workloads[row[0]]; !exists {
					if strings.Contains(row[0], "/orgs/") {
						utils.LogWarning(fmt.Sprintf("href file line %d - %s is not a workload. skipping.", rowIndex+1, row[0]), true)
					}
				} else {
					// If it does exist, add it to the slice
					wklds = append(wklds, val)
				}
			}
		} else {
			// No href file, use all the managed workloads
			wklds = allManagedWklds
		}
	}

	// Confirm it's not unmanaged and check the labels to find our matches.
	for _, w := range wklds {
		if w.GetMode() == "unmanaged" {
			continue
		}
		if hoursSinceLastHB > 0 && w.HoursSinceLastHeartBeat() < float64(hoursSinceLastHB) {
			continue
		}
		if w.Online && !includeOnline {
			continue
		}
		roleCheck, appCheck, envCheck, locCheck := true, true, true, true
		if app != "" && w.GetApp(pce.Labels).Value != app {
			appCheck = false
		}
		if role != "" && w.GetRole(pce.Labels).Value != role {
			roleCheck = false
		}
		if env != "" && w.GetEnv(pce.Labels).Value != env {
			envCheck = false
		}
		if loc != "" && w.GetLoc(pce.Labels).Value != loc {
			locCheck = false
		}
		if roleCheck && appCheck && locCheck && envCheck && !setLabelExcl {
			targetWklds = append(targetWklds, w)
		} else if (!roleCheck || !appCheck || !locCheck || !envCheck) && setLabelExcl {
			targetWklds = append(targetWklds, w)
		}
	}

	if len(targetWklds) == 0 {
		if !includeOnline {
			utils.LogInfo("zero workloads identified. The --include-online option was not set so only offline workloads were evaluated.", true)
		} else {
			utils.LogInfo("zero workloads identified.", true)
		}
		return
	}

	// If there are more than 0 workloads, build the data slice for writing
	data := [][]string{{"hostname", "href", "role", "app", "env", "loc", "policy_sync_status", "last_heartbeat", "hours_since_last_heartbeat"}}
	for _, t := range targetWklds {
		// Reset the time value
		hoursSinceLastHB := ""
		// Get the hours since last heartbeat
		timeParsed, err := time.Parse(time.RFC3339, t.Agent.Status.LastHeartbeatOn)
		if err != nil {
			utils.LogWarning(fmt.Sprintf("%s - %s - agent.status.last_heartbeat_on: %s - error parsing time since last heartbeat - %s", t.Hostname, t.Href, t.Agent.Status.LastHeartbeatOn, err.Error()), true)
			hoursSinceLastHB = "NA"
		} else {
			now := time.Now().UTC()
			hoursSinceLastHB = fmt.Sprintf("%f", now.Sub(timeParsed).Hours())
		}
		// Append to our data array
		data = append(data, []string{t.Hostname, t.Href, t.GetRole(pce.Labels).Value, t.GetApp(pce.Labels).Value, t.GetEnv(pce.Labels).Value, t.GetLoc(pce.Labels).Value, t.Agent.Status.SecurityPolicySyncState, t.Agent.Status.LastHeartbeatOn, hoursSinceLastHB})
	}

	// Write CSV data
	if outputFileName == "" {
		outputFileName = fmt.Sprintf("workloader-unpair-%s.csv", time.Now().Format("20060102_150405"))
	}
	utils.WriteOutput(data, data, outputFileName)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d workloads requiring unpairing. See %s for details. To do the unpair, run again using --update-pce flag. The --no-prompt flag will bypass the prompt if used with --update-pce.", len(targetWklds), outputFileName), true)
		utils.LogEndCommand("unpair")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("%s [PROMPT] - workloader identified %d workloads requiring unpairing in %s (%s). See %s for details. Do you want to run the unpair? (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), len(targetWklds), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string), outputFileName)
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("prompt denied to unpair %d workloads.", len(targetWklds)), true)
			utils.LogEndCommand("unpair")
			return
		}
	}

	// If single
	if singleUnpair {
		// Create a slice of slices
		singleTargetWklds := [][]illumioapi.Workload{}
		for _, w := range targetWklds {
			singleTargetWklds = append(singleTargetWklds, []illumioapi.Workload{w})
		}

		// Iterate through those for unpairing
		for i, w := range singleTargetWklds {
			apiResps, err := pce.WorkloadsUnpair(w, restore)
			utils.LogAPIResp("unpair workloads", apiResps[0])
			if err != nil {
				utils.LogError(err.Error())
			}
			// Update progress
			utils.LogInfo(fmt.Sprintf("unpaired %d of %d - %s - status code %d", i+1, len(singleTargetWklds), w[0].Href, apiResps[0].StatusCode), true)
		}
	} else {
		// We will only get here if we have need to run the unpair
		apiResps, err := pce.WorkloadsUnpair(targetWklds, restore)
		for _, a := range apiResps {
			utils.LogAPIResp("unpair workloads", a)
		}
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	utils.LogEndCommand("unpair")
}
