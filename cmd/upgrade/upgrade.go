package upgrade

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Set global variables for flags
var targetVersion, hostFile, loc, env, app, role string
var noPrompt, logOnly bool
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
	UpgradeCmd.Flags().BoolVar(&logOnly, "logonly", false, "Will only log changes that would occur. No VENS will be upgraded.")
	UpgradeCmd.Flags().BoolVar(&noPrompt, "noprompt", false, "Will run the upgrades with no confirmation prompts.")
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

You will be prompted with the number of VENs that will be upgraded before upgrading in the PCE unless the --noprompt flag is set.`,

	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetPCE()
		if err != nil {
			utils.Log(1, err.Error())
		}

		wkldUpgrade()
	},
}

func wkldUpgrade() {

	// Get all workloads
	wklds, _, err := pce.GetAllWorkloads()
	if err != nil {
		utils.Log(1, err.Error())
	}

	// Get label map
	labelMap, _, err := pce.GetLabelMapH()
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
			if app != "" && w.GetApp(labelMap).Value != app {
				continue
			}
			if role != "" && w.GetRole(labelMap).Value != role {
				continue
			}
			if env != "" && w.GetEnv(labelMap).Value != env {
				continue
			}
			if loc != "" && w.GetLoc(labelMap).Value != loc {
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

	// Get our confirmation
	proceed := false
	if !noPrompt && !logOnly {
		fmt.Printf("This will update %d VENs. Please type \"yes\" to continue: ", len(targetWklds))
		var promptStr string
		fmt.Scanln(&promptStr)
		if promptStr == "yes" {
			proceed = true
		}
	}

	// If we have permission, cycle through our target workloads and run the upgrade
	if proceed {
		for _, t := range targetWklds {

			// Log the current version
			utils.Log(0, fmt.Sprintf("%s to be upgraded from %s to %s.", t.Hostname, t.Agent.Status.AgentVersion, targetVersion))

			// Run the upgrade if log only is not set
			if !logOnly {
				apiResp, err := pce.WorkloadUpgrade(t.Href, targetVersion)
				if err != nil {
					utils.Log(1, err.Error())
				}
				utils.Log(0, fmt.Sprintf("%s ven upgrade to %s received status code of %d", t.Hostname, targetVersion, apiResp.StatusCode))
			}
		}
	}
}
