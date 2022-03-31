package upgrade

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
var targetVersion, hostFile, loc, env, app, role, outputFileName string
var updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

// Init handles flags
func init() {

	UpgradeCmd.Flags().StringVar(&targetVersion, "version", "", "Target VEN version in format of \"19.1.0-5631\"")
	UpgradeCmd.MarkFlagRequired("version")
	UpgradeCmd.Flags().StringVarP(&hostFile, "hostFile", "i", "", "Input CSV file with hostname list. Hostnames in first column (other columns are ok). Header is optional. Using this ignore loc, env, app, and role label flags.")
	UpgradeCmd.Flags().StringVarP(&loc, "loc", "l", "", "Location Label. Blank means all locations.")
	UpgradeCmd.Flags().StringVarP(&env, "env", "e", "", "Environment Label. Blank means all environments.")
	UpgradeCmd.Flags().StringVarP(&app, "app", "a", "", "Application Label. Blank means all applications.")
	UpgradeCmd.Flags().StringVarP(&role, "role", "r", "", "Role Label. Blank means all roles.")
	UpgradeCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	UpgradeCmd.Flags().SortFlags = false

}

// UpgradeCmd runs the hostname parser
var UpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the VEN installed on workloads by labels or an input hostname list.",
	Long: `
Upgrade the VEN installed on workloads by labels or an input hostname list.

If a host file is used, the label flags are ignored.

All workloads will be upgraded if there is no hostfile and no provided labels.

Default output is a CSV file with what would be upgraded. Use the --update-pce command to run the upgrades with a user prompt confirmation. Use --update-pce and --no-prompt to run upgrade with no prompts.`,
	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get persistent flags from Viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		wkldUpgrade()
	},
}

func wkldUpgrade() {

	utils.LogStartCommand("upgrade")

	// Get all managed workloads
	wklds, a, err := pce.GetAllWorkloadsQP(map[string]string{"managed": "true"})
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create our target wkld slice
	var targetWklds []illumioapi.Workload

	// If we don't have a hostfile, confirm it's not unmanaged and check the labels to find our matches.
	if hostFile == "" {
		for _, w := range wklds {
			if app != "" && w.GetApp(pce.Labels).Value != app {
				continue
			}
			if role != "" && w.GetRole(pce.Labels).Value != role {
				continue
			}
			if env != "" && w.GetEnv(pce.Labels).Value != env {
				continue
			}
			if loc != "" && w.GetLoc(pce.Labels).Value != loc {
				continue
			}
			// if w.VEN.Version == targetVersion {
			// 	continue
			// }
			targetWklds = append(targetWklds, w)

		}
	}

	// If we are given a hostfile, parse that.
	if hostFile != "" {
		// Parse CSV File
		csvData, err := utils.ParseCSV(hostFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		for i, row := range csvData {
			if val, ok := pce.Workloads[row[0]]; !ok {
				utils.LogWarning(fmt.Sprintf("line %d - %s is not a workload. skipping", i, row[0]), true)
				continue
				// } else if val.VEN.Version == targetVersion {
				// 	utils.LogInfo(fmt.Sprintf("line %d - %s is already at %s. skipping", i, val.Hostname, targetVersion), true)
				// 	continue
			} else {
				targetWklds = append(targetWklds, val)
			}
		}
	}

	// Check length of target workloads
	if len(targetWklds) > 25000 {
		utils.LogError("target workloads exceed max length of 25,000")
	}

	// Build output data
	if len(targetWklds) > 0 {
		outputData := [][]string{{"hostname", "href", "role", "app", "env", "loc", "current_ven_version", "targeted_ven_version"}}
		for _, t := range targetWklds {
			outputData = append(outputData, []string{t.Hostname, t.Href, t.GetRole(pce.Labels).Value, t.GetApp(pce.Labels).Value, t.GetEnv(pce.Labels).Value, t.GetLoc(pce.Labels).Value, t.VEN.Version, targetVersion})
		}
		if outputFileName == "" {
			outputFileName = "workloader-upgrade-" + time.Now().Format("20060102_150405") + ".csv"
		}
		utils.WriteOutput(outputData, outputData, outputFileName)

		// If updatePCE is disabled, we are just going to alert the user what will happen and log
		if !updatePCE {
			utils.LogInfo(fmt.Sprintf("workloader identified %d workloads requiring VEN upgrades. See %s for details. To do the upgrade, run again using --update-pce flag. The --no-prompt flag will bypass the prompt if used with --update-pce.", len(targetWklds), outputFileName), true)
			utils.LogEndCommand("upgrade")
			return
		}

		// If updatePCE is set, but not noPrompt, we will prompt the user.
		if updatePCE && !noPrompt {
			var prompt string
			fmt.Printf("[PROMPT] - workloader identified %d workloads in %s (%s) requiring VEN updates. See %s for details. Do you want to run the upgrade? (yes/no)? ", len(targetWklds), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string), outputFileName)
			fmt.Scanln(&prompt)
			if strings.ToLower(prompt) != "yes" {
				utils.LogInfo(fmt.Sprintf("prompt denied to upgrade %d workloads", len(targetWklds)), true)
				utils.LogEndCommand("upgrade")
				return
			}
		}

		// We will only get here if we have need to run the upgrade. Start by creating the target VENs list
		targetVENs := []illumioapi.VEN{}
		for _, t := range targetWklds {
			// Log the current version
			utils.LogInfo(fmt.Sprintf("%s to be upgraded from %s to %s.", t.Hostname, t.Agent.Status.AgentVersion, targetVersion), false)

			// Populate VEN list
			for _, w := range targetWklds {
				targetVENs = append(targetVENs, *w.VEN)
			}
		}

		// Call the API
		resp, a, err := pce.UpgradeVENs(targetVENs, targetVersion)
		utils.LogAPIResp("UpgradeVENs", a)
		if err != nil {
			utils.LogError(err.Error())
		}

		utils.LogInfo(fmt.Sprintf("bulk ven upgrade for %d hosts to %s received status code of %d with %d errors.", len(targetVENs), targetVersion, a.StatusCode, len(resp.VENUpgradeErrors)), true)
		if err != nil {
			utils.LogError(err.Error())
		}
		for i, e := range resp.VENUpgradeErrors {
			utils.LogInfo(fmt.Sprintf("error %d - token: %s; message: %s; hrefs: %s", i+1, e.Token, e.Message, strings.Join(e.Hrefs, ", ")), true)
		}

	}

	utils.LogEndCommand("upgrade")
}
