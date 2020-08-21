package rulesetcopy

import (
	"fmt"
	"os"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debug, updatePCE, noPrompt, doNotProvision bool
var csvFile, fromRuleset, toRuleset string
var pce illumioapi.PCE
var err error

func init() {
	RuleSetCopyCmd.Flags().StringVarP(&fromRuleset, "from-ruleset", "f", "", "Name of Ruleset to copy. Required")
	RuleSetCopyCmd.MarkFlagRequired("from-ruleset")
	RuleSetCopyCmd.Flags().StringVarP(&toRuleset, "to-ruleset", "t", "", "Name of Ruleset to update with copied rules")
	RuleSetCopyCmd.MarkFlagRequired("to-ruleset")
	RuleSetCopyCmd.Flags().BoolVarP(&doNotProvision, "do-not-prov", "x", false, "Do not provision created/updated IP Lists. Will provision by default.")
}

// RuleSetCopyCmd runs the upload command
var RuleSetCopyCmd = &cobra.Command{
	Use:   "rulesetcopy",
	Short: "Copy rules from one ruleset to another.",
	Long: `
Copy all the rules for one ruleset to another.  To select the rules from one ruleset specify the ruleset name by placing the name after --from-ruleset or -f option.
 The ruleset you want to copy those rules into will be specified by the ruleset name after the --to-ruleset or -t option.  Both must exist on the PCE.

`,

	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		// if len(args) !=  {
		// 	fmt.Println("Command requires 1 argument for the csv file. See usage help.")
		// 	os.Exit(0)
		// }
		// csvFile = args[0]

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		// Disable stdout
		viper.Set("output_format", "csv")
		if err := viper.WriteConfig(); err != nil {
			utils.LogError(err.Error())
		}

		copyruleset()
	},
}

func copyruleset() {

	// Log start of run
	utils.LogStartCommand("copyruletset")

	// Check if we have destination PCE if we need it
	// if updatePCE && toPCE == "" {
	// 	utils.LogError("need --to-pce (-t) flag set if using update-pce")
	// }

	// // Get the source pce
	// sPce, err := utils.GetPCEbyName(fromPCE, true)
	// if err != nil {
	// 	utils.LogError(err.Error())
	// }

	// Get all rulesets from the source PCE
	rulesets, a, err := pce.GetAllRuleSets("draft")
	utils.LogAPIResp("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// // Parse the input csv
	// csvData, err := utils.ParseCSV(csvFile)
	// if err != nil {
	// 	utils.LogError(err.Error())
	// }

	// Create a slice to hold IP Lists
	//ipls := []illumioapi.IPList{}

	savefromHref, savetoHref := "", ""
	foundfrom, foundto := false, false
	var fromRules []*illumioapi.Rule

	// Iterate the CSV file
	for _, ruleset := range rulesets {

		if !(foundfrom && foundto) {
			switch ruleset.Name {
			case fromRuleset:
				savefromHref = ruleset.Href
				fromRules = ruleset.Rules
				foundfrom = true
			case toRuleset:
				savetoHref = ruleset.Href
				foundto = true
			}
		} else {
			break
		}

	}
	if !(foundfrom || foundto) {
		fmt.Println("Command requires 1 argument for the csv file. See usage help.")
		utils.LogInfo("NAME was not matched.", true)
		os.Exit(0)
	}
	for _, fromrule := range fromRules {

		copyRule := illumioapi.Rule{Href: "",
			Consumers:                   fromrule.Consumers,
			ConsumingSecurityPrincipals: fromrule.ConsumingSecurityPrincipals,
			Description:                 fromrule.Description,
			Enabled:                     fromrule.Enabled,
			IngressServices:             fromrule.IngressServices,
			ResolveLabelsAs:             fromrule.ResolveLabelsAs,
			Providers:                   fromrule.Providers,
			SecConnect:                  fromrule.SecConnect,
			Stateless:                   fromRules.Stateless,
			MachineAuth:                 fromRules.MachineAuth,
			UnscopedConsumers:           fromrule.UnscopedConsumers,
		}

		fmt.Print(copyRule)
		rules, a, err := pce.CreateRuleSetRule(savetoHref, copyRule)
		utils.LogAPIResp("CreateRuleSetRule", a)
		if err != nil {
			utils.LogError(err.Error())
		}

	}

}
