package increasevenupdaterate

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var role, app, env, loc string
var forMinutes int
var pce illumioapi.PCE
var err error
var updatePCE, noPrompt bool

func init() {
	IncreaseVENUpdateRateCmd.Flags().StringVarP(&role, "role", "r", "", "Role Label. Blank means all roles.")
	IncreaseVENUpdateRateCmd.Flags().StringVarP(&app, "app", "a", "", "Application Label. Blank means all applications.")
	IncreaseVENUpdateRateCmd.Flags().StringVarP(&env, "env", "e", "", "Environment Label. Blank means all environments.")
	IncreaseVENUpdateRateCmd.Flags().StringVarP(&loc, "loc", "l", "", "Location Label. Blank means all locations.")
	IncreaseVENUpdateRateCmd.Flags().IntVarP(&forMinutes, "for-minutes", "f", 0, "Minutes to issue increase command every 10 minutes (e.g., 60 will run the process for 60 minutes with the command running 6 total times.")

}

// IncreaseVENUpdateRateCmd runs the workload identifier
var IncreaseVENUpdateRateCmd = &cobra.Command{
	Use:   "increase-ven-rate",
	Short: "Increase the VEN update rate to every 30 seconds for a period of 10 minutes.",
	Long: `
Increase the VEN update rate to every 30 seconds for a period of 10 minutes.

Use the role, app, env, and loc labels to specify workloads. One label can be provided for each key and they are combined with the "AND" operator.

The forMinutes flag can be used to have workloader run the command every 10 minutes for the specified forMinutes value. You'll need to keep your shell open (or run in the background).`,

	Example: `# Increase frequency for all workloads in the CRM (app) PROD (env) app group for the default 10 mins:
  workloader increase-ven-rate --app CRM --env PROD

  # Increase frequency for all workloads in the CRM (app) PROD (env) app group for an hour:
  workloader increase-ven-rate --app CRM --env PROD --for-minutes 60`,

	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(fmt.Sprintf("getting PCE for mode command - %s", err))
		}

		// Get Viper configuration
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		increaseVENUpdateRate()
	},
}

func increaseVENUpdateRate() {
	// Log start of execution
	utils.LogStartCommand("increase-ven-rate")

	// Get the labels
	if err := pce.Load(illumioapi.LoadInput{Labels: true}); err != nil {
		utils.LogError(err.Error())
	}

	// Set up the map for the workload query and process labels
	var qp = (map[string]string{"managed": "true"})
	qp["online"] = "true"
	values := []string{role, app, env, loc}
	keys := []string{"role", "app", "env", "loc"}
	labelHrefs := []string{}
	for i, v := range values {
		// If the flag wasn't provided, continue
		if v == "" {
			continue
		}
		if val, ok := pce.Labels[keys[i]+v]; !ok {
			utils.LogError(fmt.Sprintf("%s does not exist as %s label", v, keys[i]))
		} else {
			labelHrefs = append(labelHrefs, fmt.Sprintf("\"%s\"", val.Href))
		}
	}
	if len(labelHrefs) > 0 {
		qp["labels"] = fmt.Sprintf("[[%s]]", strings.Join(labelHrefs, ","))
	}

	// Get the workloads
	pce.Load(illumioapi.LoadInput{Workloads: true, WorkloadsQueryParameters: qp})
	utils.LogInfo(fmt.Sprintf("%d workloads identified", len(pce.WorkloadsSlice)), true)

	// If we have zero workloads, we are done.
	if len(pce.WorkloadsSlice) == 0 {
		utils.LogEndCommand("increase-ven-rate")
		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d workloads requiring VEN update rate incease in %s (%s). To update, run again using --update-pce flag. The --no-prompt flag will bypass the prompt if used with --update-pce.", len(pce.WorkloadsSlice), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string)), true)
		utils.LogEndCommand("increase-ven-rate")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("\r\n%s [PROMPT] - workloader will increase the VEN update rate for %d workloads. Do you want to run the change (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), len(pce.WorkloadsSlice))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("prompt denied for %d workloads.", len(pce.WorkloadsSlice)), true)
			utils.LogEndCommand("increase-ven-rate")
			return
		}
	}

	iterations := 0
	requiredIterations := forMinutes / 10
	for iterations <= requiredIterations {
		a, err := pce.IncreaseTrafficUpdateRate(pce.WorkloadsSlice)
		utils.LogAPIResp("IncreaseTrafficUpdateRate", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("Increase VEN update rate status: %d", a.StatusCode), true)
		iterations++

		if iterations <= requiredIterations {
			utils.LogInfo(fmt.Sprintf("%d iterations remaining. Running another in 10 minutes", requiredIterations-iterations+1), true)
			time.Sleep(600 * time.Second)
		}
	}
	utils.LogEndCommand("increase-ven-rate")

}
