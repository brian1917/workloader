package upgrade

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
var targetVersion, hostFile, loc, env, app, role string
var debug, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

// Init handles flags
func init() {

	UpgradeCmd.Flags().StringVar(&targetVersion, "version", "", "Target VEN version in format of \"19.1.0-5631\"")
	UpgradeCmd.Flags().StringVarP(&hostFile, "hostFile", "i", "", "Input CSV file with hostname list. Using this ignore loc, env, app, and role label flags.")
	UpgradeCmd.Flags().StringVarP(&loc, "loc", "l", "", "Location Label. Blank means all locations.")
	UpgradeCmd.Flags().StringVarP(&env, "env", "e", "", "Environment Label. Blank means all environments.")
	UpgradeCmd.Flags().StringVarP(&app, "app", "a", "", "Application Label. Blank means all applications.")
	UpgradeCmd.Flags().StringVarP(&role, "role", "r", "", "Role Label. Blank means all roles.")
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
		pce, err = utils.GetPCE(true)
		if err != nil {
			utils.Log(1, err.Error())
		}

		// Get persistent flags from Viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		wkldUpgrade()
	},
}

func wkldUpgrade() {

	// Get all workloads
	wklds, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloads", a)
	}
	if err != nil {
		utils.Log(1, err.Error())
	}

	// Create our target wkld slice
	var targetWklds []illumioapi.Workload

	// If we don't have a hostfile, confirm it's not unmanaged and check the labels to find our matches.
	if hostFile == "" {
		for _, w := range wklds {
			if w.GetMode() == "unmanaged" {
				continue
			}
			if app != "" && w.GetApp(pce.LabelMapH).Value != app {
				continue
			}
			if role != "" && w.GetRole(pce.LabelMapH).Value != role {
				continue
			}
			if env != "" && w.GetEnv(pce.LabelMapH).Value != env {
				continue
			}
			if loc != "" && w.GetLoc(pce.LabelMapH).Value != loc {
				continue
			}
			targetWklds = append(targetWklds, w)

		}
	}

	// If we are given a hostfile, parse that.
	if hostFile != "" {
		// Open CSV File
		csvFile, _ := os.Open(hostFile)
		reader := csv.NewReader(bufio.NewReader(csvFile))

		// Create our hostFileMap and cycle through CSV to add to it.
		hostFileMap := make(map[string]bool)
		for {
			line, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				utils.Log(1, fmt.Sprintf("Reading CSV File - %s", err))
			}
			hostFileMap[line[0]] = true
		}

		// Cycle through workloads and populate our targetWklds slice
		for _, w := range wklds {
			if w.GetMode() == "unmanaged" {
				continue
			}
			if hostFileMap[w.Hostname] {
				targetWklds = append(targetWklds, w)
			}
		}
	}

	// Create a CSV wtih the upgrades
	outFile, err := os.Create("workloader-upgrade-" + time.Now().Format("20060102_150405") + ".csv")
	if err != nil {
		utils.Log(1, fmt.Sprintf("creating CSV - %s\n", err))
	}

	// Build the data slice for writing
	data := [][]string{[]string{"hostname", "href", "role", "app", "env", "loc", "current_ven_version", "targeted_ven_version"}}
	for _, t := range targetWklds {
		data = append(data, []string{t.Hostname, t.Href, t.GetRole(pce.LabelMapH).Value, t.GetApp(pce.LabelMapH).Value, t.GetEnv(pce.LabelMapH).Value, t.GetLoc(pce.LabelMapH).Value, t.Agent.Status.AgentVersion, targetVersion})
	}

	// Write CSV data
	writer := csv.NewWriter(outFile)
	writer.WriteAll(data)
	if err := writer.Error(); err != nil {
		utils.Log(1, fmt.Sprintf("writing CSV - %s\n", err))
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.Log(0, fmt.Sprintf("upgrade identified %d workloads requiring VEN update - see %s for details.", len(targetWklds), outFile.Name()))
		fmt.Printf("Upgrade identified %d workloads requiring VEN update. See %s for details. To do the upgrade, run again using --update-pce flag. The --auto flag will bypass the prompt if used with --update-pce.\r\n", len(targetWklds), outFile.Name())
		utils.Log(0, "completed running upgrade command")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("Upgrade identified %d workloads requiring VEN updates. See %s for details. Do you want to run the upgrade? (yes/no)? ", len(targetWklds), outFile.Name())
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.Log(0, fmt.Sprintf("upgrade identified %d workloads requiring VEN update - see %s for details. user denied prompt", len(targetWklds), outFile.Name()))
			fmt.Println("Prompt denied.")
			utils.Log(0, "completed running import command")
			return
		}
	}

	// We will only get here if we have need to run the upgrade
	for _, t := range targetWklds {
		// Log the current version
		utils.Log(0, fmt.Sprintf("%s to be upgraded from %s to %s.", t.Hostname, t.Agent.Status.AgentVersion, targetVersion))
		a, err := pce.WorkloadUpgrade(t.Href, targetVersion)
		if debug {
			utils.LogAPIResp("WorkloadUpgrade", a)
		}
		utils.Log(0, fmt.Sprintf("%s ven upgrade to %s received status code of %d", t.Hostname, targetVersion, a.StatusCode))
		if err != nil {
			utils.Log(1, err.Error())
		}
	}
}
