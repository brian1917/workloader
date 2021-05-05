package unpair

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
var hrefFile, role, app, env, loc, restore, outputFileName string
var debug, updatePCE, noPrompt, setLabelExcl, includeOnline, single bool
var hoursSinceLastHB int
var pce illumioapi.PCE
var err error

// Init handles flags
func init() {
	UnpairCmd.Flags().StringVar(&restore, "restore", "", "Restore value. Must be saved, default, or disable.")
	UnpairCmd.Flags().StringVarP(&hrefFile, "href", "f", "", "Location of file with HREFs to be used instead of starting with all workloads.")
	UnpairCmd.Flags().StringVarP(&role, "role", "r", "", "Role Label. Blank means all roles.")
	UnpairCmd.Flags().StringVarP(&app, "app", "a", "", "Application Label. Blank means all applications.")
	UnpairCmd.Flags().StringVarP(&env, "env", "e", "", "Environment Label. Blank means all environments.")
	UnpairCmd.Flags().StringVarP(&loc, "loc", "l", "", "Location Label. Blank means all locations.")
	UnpairCmd.Flags().BoolVarP(&setLabelExcl, "exclude-labels", "x", false, "Use provided label filters as excludes.")
	UnpairCmd.Flags().IntVar(&hoursSinceLastHB, "hours", 0, "Hours since last heartbeat. No value (i.e., 0) will ignore heartbeats.")
	UnpairCmd.Flags().BoolVar(&includeOnline, "include-online", false, "Include workloads that are online. By default only offline workloads that meet criteria will be unpaired.")
	UnpairCmd.Flags().BoolVar(&single, "single", false, "One API call per unpair versus one API call per 1000 workloads. This will be dramatically slower, but provide more details in syslog messages.")
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
  workloader unpair --hours 50 --app ERP --env PROD --restore saved --update-pce --no-prompt

  # See what workloads would unpair if we set the threshold for 24 hours for all labels:
  workloader unpair --hours 50 --restore saved
 `,
	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get persistent flags from Viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		unpair()
	},
}

func unpair() {

	utils.LogStartCommand("unpair")

	// Check that we aren't unpairing the whole PCE
	if app == "" && role == "" && env == "" && loc == "" && hoursSinceLastHB == 0 && hrefFile == "" {
		utils.LogError("Must provide labels, hours, or an input file.")
	}

	// Check the restore value
	restore = strings.ToLower(restore)
	if restore != "saved" && restore != "default" && restore != "disable" {
		utils.LogError("Restore value must be saved, default, or disable.")
	}

	// Get all workloads
	allWklds, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloads", a)
	}
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create our wkld slice that will either be all workloads or the workloads that match the HREF input
	var wklds []illumioapi.Workload

	// If we are given a hrefFile, parse that.
	if hrefFile != "" {
		// Create our href list
		csvWklds := make(map[string]bool)

		// Open CSV File
		csvFile, _ := os.Open(hrefFile)
		reader := csv.NewReader(bufio.NewReader(csvFile))

		// Cycle through CSV to add to it.
		for {
			line, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				utils.LogError(fmt.Sprintf("Reading CSV File - %s", err))
			}
			csvWklds[line[0]] = true
		}

		// Create our wklds slice
		for _, w := range allWklds {
			if csvWklds[w.Href] {
				wklds = append(wklds, w)
			}
		}
	} else {
		// If we don't have an HREF file, all workloads is our wkld slice
		wklds = allWklds
	}

	// Create our targetWklds slice
	var targetWklds []illumioapi.Workload

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
	data := [][]string{[]string{"hostname", "href", "role", "app", "env", "loc", "policy_sync_status", "last_heartbeat", "hours_since_last_heartbeat"}}
	for _, t := range targetWklds {
		// Reset the time value
		hoursSinceLastHB := ""
		// Get the hours since last heartbeat
		timeParsed, err := time.Parse(time.RFC3339, t.Agent.Status.LastHeartbeatOn)
		if err != nil {
			utils.LogWarning(fmt.Sprintf("error parsing time - %s", err.Error()), true)
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
	if single {
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
