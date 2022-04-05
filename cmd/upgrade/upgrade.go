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

	UpgradeCmd.Flags().StringVar(&targetVersion, "version", "", "target ven version in format of \"19.1.0-5631\"")
	UpgradeCmd.MarkFlagRequired("version")
	UpgradeCmd.Flags().StringVarP(&hostFile, "hostFile", "i", "", "input csv file with hostname list. hostnames in first column (other columns are ok). header is optional. label flags ignored with input file.")
	UpgradeCmd.Flags().BoolVarP(&singleAPI, "single-api", "s", false, "get each workload and ven info from the csv as a single api call instead of getting all managed workloads and vens in the pce. optimal for pces with a lot of workloads and a relatively small input file. flag ignored if no host file provided.")
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

func GetWkldsByHostnameList(hostnames []string) (wklds []illumioapi.Workload, vens []illumioapi.VEN) {
	venMap := make(map[string]illumioapi.VEN)
	wkldMap := make(map[string]illumioapi.Workload)
	utils.LogInfo("getting workloads individually from pce...", true)
	for i, h := range hostnames {
		w, a, err := pce.GetWkldByHostname(h)
		utils.LogAPIResp("GetWkldByHostname", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		if w.Hostname == "" {
			utils.LogInfo(fmt.Sprintf("getting %s - %d of %d workloads (%d%%). does not exist.", h, i+1, len(hostnames), (i+1)*100/len(hostnames)), true)
		} else {
			wklds = append(wklds, w)
			wkldMap[w.Href] = w
			if w.Hostname != "" {
				wkldMap[w.Hostname] = w
			}
			if w.Name != "" {
				wkldMap[w.Name] = w
			}
			if w.ExternalDataSet != "" && w.ExternalDataReference != "" {
				wkldMap[w.ExternalDataSet+w.ExternalDataReference] = w
			}
			ven, a, err := pce.GetVenByHref(w.VEN.Href)
			utils.LogAPIResp("GetVenByHref", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			vens = append(vens, ven)
			venMap[ven.Href] = ven
			venMap[ven.Name] = ven
		}
		pce.Workloads = wkldMap
		pce.VENs = venMap
		if w.Hostname != "" {
			utils.LogInfo(fmt.Sprintf("getting %s - %d of %d workloads (%d%%). success.", h, i+1, len(hostnames), (i+1)*100/len(hostnames)), true)
		}

	}
	return wklds, vens
}

func wkldUpgrade() {

	utils.LogStartCommand("upgrade")

	var wklds []illumioapi.Workload
	var err error
	var a illumioapi.APIResponse
	var csvData [][]string
	var targetWklds []illumioapi.Workload

	if hostFile == "" || !singleAPI {
		// Get all managed workloads
		utils.LogInfo("getting all workload and ven info...", true)
		wklds, a, err = pce.GetAllWorkloadsQP(map[string]string{"managed": "true"})
		utils.LogAPIResp("GetAllWorkloads", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		_, a, err = pce.GetAllVens(nil)
		utils.LogAPIResp("GetAllVens", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("get all managed workload and ven info complete (%d workloads)", len(wklds)), true)
	}

	// If we are given a hostfile, parse that.
	if hostFile != "" {
		// Parse CSV File
		csvData, err = utils.ParseCSV(hostFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		if singleAPI {
			hostnameList := []string{}
			for _, row := range csvData {
				hostnameList = append(hostnameList, row[0])
			}
			wklds, _ = GetWkldsByHostnameList(hostnameList)
		}
		for i, row := range csvData {
			if val, ok := pce.Workloads[row[0]]; !ok {
				utils.LogWarning(fmt.Sprintf("line %d - %s is not a workload. skipping", i, row[0]), true)
				continue
			} else if pce.VENs[val.VEN.Href].Version == targetVersion {
				utils.LogInfo(fmt.Sprintf("line %d - %s is already at %s. skipping", i, val.Hostname, targetVersion), true)
				continue
			} else if pce.VENs[val.VEN.Href].Status != "active" || !pce.Workloads[val.Href].Online {
				utils.LogInfo(fmt.Sprintf("line %d - %s ven status is %s and workload online status is %t. skipping", i, val.Hostname, pce.VENs[val.VEN.Href].Status, pce.Workloads[val.Href].Online), true)
			} else {
				targetWklds = append(targetWklds, val)
			}
		}
	}

	// If we don't have a hostfile, check the labels to find our matches.
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
			if pce.VENs[w.VEN.Href].Version == targetVersion {
				continue
			}
			if pce.VENs[w.VEN.Href].Status != "active" || !pce.Workloads[w.Href].Online {
				continue
			}
			targetWklds = append(targetWklds, w)
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
			outputData = append(outputData, []string{t.Hostname, t.Href, t.GetRole(pce.Labels).Value, t.GetApp(pce.Labels).Value, t.GetEnv(pce.Labels).Value, t.GetLoc(pce.Labels).Value, pce.VENs[t.VEN.Href].Version, targetVersion})
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

		// Populate VEN list
		for _, w := range targetWklds {
			targetVENs = append(targetVENs, *w.VEN)
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
