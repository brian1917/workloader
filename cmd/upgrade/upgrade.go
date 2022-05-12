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
var singleAPI, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

// Init handles flags
func init() {

	UpgradeCmd.Flags().StringVar(&targetVersion, "version", "v", "target ven version in format of \"19.1.0-5631\"")
	UpgradeCmd.MarkFlagRequired("version")
	UpgradeCmd.Flags().StringVarP(&hostFile, "host-file", "i", "", "csv file with ven hrefs or hostnames. any labels are ignored with this flag.")
	UpgradeCmd.Flags().BoolVarP(&singleAPI, "single-api", "s", false, "csv file with hrefs or hostnames. any labels are ignored with this flag.")
	UpgradeCmd.Flags().StringVarP(&loc, "loc", "l", "", "location label. blank means all locations.")
	UpgradeCmd.Flags().StringVarP(&env, "env", "e", "", "environment label. blank means all environments.")
	UpgradeCmd.Flags().StringVarP(&app, "app", "a", "", "application label. blank means all applications.")
	UpgradeCmd.Flags().StringVarP(&role, "role", "r", "", "role Label. blank means all roles.")
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

	// Set up the target slices
	var targetVENs []illumioapi.VEN
	var targetWorkloads []illumioapi.Workload

	// If the hostFile is blank or if singleAPI is set to false get all VENs and workloads
	if hostFile == "" || !singleAPI {
		// Get all VENs
		utils.LogInfo("getting all vens and workloads ...", true)
		_, a, err := pce.GetAllVens(nil)
		utils.LogAPIResp("GetAllVens", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		_, a, err = pce.GetAllWorkloadsQP(map[string]string{"managed": "true"})
		utils.LogAPIResp("GetAllWorkloadsQP", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("get all vens and workloads complete (%d vens)", len(pce.VENsSlice)), true)
	}

	// Parse the hostfile if it's provided
	if hostFile != "" {

		hostFileCsvData, err := utils.ParseCSV(hostFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		// Iterate through the hostfile
		for i, row := range hostFileCsvData {
			var ven illumioapi.VEN
			var a illumioapi.APIResponse
			var err error

			// If singleAPI get the VEN and workload
			if singleAPI {
				// Get by href of /orgs/ is present
				if strings.Contains(row[0], "/orgs/") {
					ven, a, err = pce.GetVenByHref(row[0])
					utils.LogAPIResp("GetVenByHref", a)
					if err != nil {
						utils.LogError(err.Error())
					}
					// Get by hostname if /orgs/ isn't present
				} else {
					ven, a, err = pce.GetVenByHostname(row[0])
					utils.LogAPIResp("GetVenByHostname", a)
					if err != nil {
						utils.LogError(err.Error())
					}
				}
				// If we aren't using singleAPI, the VEN is the value from the pce map
			} else {
				ven = pce.VENs[row[0]]
			}
			// If the VEN doesn't have a hostname, it doesn't exist
			if ven.Hostname == "" {
				utils.LogInfo(fmt.Sprintf("csv line %d - %s does not exist as a ven. skipping.", i+1, row[0]), true)
				continue
			}
			// If the version is already correct, skip
			if ven.Version == targetVersion {
				utils.LogInfo(fmt.Sprintf("csv line %d - %s is already on %s. skipping.", i+1, row[0], targetVersion), true)
				continue
			}

			// Add to the VEN slice
			targetVENs = append(targetVENs, ven)

			// Get the corresponding workload if the VEN is valid. If singleAPI is set, make the API call
			if singleAPI {
				wkld, a, err := pce.GetAllWorkloadsQP(map[string]string{"ven": ven.Href})
				utils.LogAPIResp("GetAllWorkloadsQP", a)
				if err != nil {
					utils.LogError(err.Error())
				}
				targetWorkloads = append(targetWorkloads, wkld[0])
			} else {
				// If singleAPI isn't set, we have it from the PCE map
				targetWorkloads = append(targetWorkloads, pce.Workloads[ven.Hostname])
			}

		}
	} else {
		// Not using a hostfile so iterate through all workloads
		for _, w := range pce.WorkloadsSlice {
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
			if pce.VENs[w.VEN.Href].Version == targetVersion {
				continue
			}
			if pce.VENs[w.VEN.Href].Status != "active" || !pce.Workloads[w.Href].Online {
				continue
			}
			targetVENs = append(targetVENs, pce.VENs[w.VEN.Href])
			targetWorkloads = append(targetWorkloads, w)
		}
	}

	// Build a workload lookup map
	wkldByVenHrefMap := make(map[string]illumioapi.Workload)
	for _, w := range targetWorkloads {
		wkldByVenHrefMap[w.VEN.Href] = w
	}

	// Ensure PCE Maps are loaded
	pce.VENsSlice = targetVENs
	pce.WorkloadsSlice = targetWorkloads
	pce.LoadVenMap()
	pce.LoadWorkloadMap()

	// Check length of target workloads
	if len(targetVENs) > 25000 {
		utils.LogError("target vens exceed max length of 25,000")
	}

	// Build output data
	if len(targetVENs) > 0 {
		outputData := [][]string{{"hostname", "ven_href", "wkld_href", "role", "app", "env", "loc", "current_ven_version", "targeted_ven_version"}}
		for _, t := range targetVENs {
			targetWkld := pce.Workloads[t.Hostname]
			outputData = append(outputData, []string{t.Hostname, t.Href, targetWkld.Href, targetWkld.GetRole(pce.Labels).Value, targetWkld.GetApp(pce.Labels).Value, targetWkld.GetEnv(pce.Labels).Value, targetWkld.GetLoc(pce.Labels).Value, t.Version, targetVersion})
		}
		if outputFileName == "" {
			outputFileName = "workloader-upgrade-" + time.Now().Format("20060102_150405") + ".csv"
		}
		utils.WriteOutput(outputData, outputData, outputFileName)

		// If updatePCE is disabled, we are just going to alert the user what will happen and log
		if !updatePCE {
			utils.LogInfo(fmt.Sprintf("workloader identified %d workloads requiring VEN upgrades. See %s for details. To do the upgrade, run again using --update-pce flag. The --no-prompt flag will bypass the prompt if used with --update-pce.", len(targetVENs), outputFileName), true)
			utils.LogEndCommand("upgrade")
			return
		}

		// If updatePCE is set, but not noPrompt, we will prompt the user.
		if updatePCE && !noPrompt {
			var prompt string
			fmt.Printf("[PROMPT] - workloader identified %d workloads in %s (%s) requiring VEN updates. See %s for details. Do you want to run the upgrade? (yes/no)? ", len(targetVENs), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string), outputFileName)
			fmt.Scanln(&prompt)
			if strings.ToLower(prompt) != "yes" {
				utils.LogInfo(fmt.Sprintf("prompt denied to upgrade %d workloads", len(targetVENs)), true)
				utils.LogEndCommand("upgrade")
				return
			}
		}

		// Call the API
		resp, a, err := pce.UpgradeVENs(targetVENs, targetVersion)
		utils.LogAPIResp("UpgradeVENs", a)
		if err != nil {
			utils.LogError(err.Error())
		}

		utils.LogInfo(fmt.Sprintf("bulk ven upgrade for %d workloads to %s received status code of %d with %d errors.", len(targetVENs), targetVersion, a.StatusCode, len(resp.VENUpgradeErrors)), true)
		if err != nil {
			utils.LogError(err.Error())
		}
		for i, e := range resp.VENUpgradeErrors {
			utils.LogInfo(fmt.Sprintf("error %d - token: %s; message: %s; hrefs: %s", i+1, e.Token, e.Message, strings.Join(e.Hrefs, ", ")), true)
		}

	}

	utils.LogEndCommand("upgrade")
}
