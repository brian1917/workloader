package edgerulecopy

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debug, updatePCE, noPrompt, delete, doNotProvision bool
var csvFile, fromGroup, toGroup string
var pce illumioapi.PCE
var err error

func init() {
	EdgeRuleCopyCmd.Flags().StringVarP(&fromGroup, "from-group", "f", "", "Name of Endpoint group to copy rules from. Required")
	EdgeRuleCopyCmd.MarkFlagRequired("from-group")
	EdgeRuleCopyCmd.Flags().StringVarP(&toGroup, "to-group", "t", "", "Name of Endpoint group to create rules from rules copied from other Endpoint group. Required.")
	EdgeRuleCopyCmd.MarkFlagRequired("to-group")
	EdgeRuleCopyCmd.Flags().BoolVarP(&delete, "delete", "d", false, "Delete rules that were copied previously from the from-group to the to-group but are no longer in the from-group.")
	EdgeRuleCopyCmd.Flags().BoolVarP(&doNotProvision, "do-not-prov", "x", false, "Do not provision created Endpoint group rules. Will provision by default.")

	EdgeRuleCopyCmd.Flags().SortFlags = false
}

// EdgeRuleCopyCmd runs the rules copy command between 2 Illumio Edge groups
var EdgeRuleCopyCmd = &cobra.Command{
	Use:   "edge-rule-copy",
	Short: "Copy rules from one group to another.",
	Long: `
Copy all the rules for one group to another.  To select the rules from one endpoint group specify the group name by placing the name after --from-ruleset (-f) option.
The group name that will receive these rules should be entered after the --to-group (-t) option.  Both groups must exist on the PCE.  Exact match is required.
If Endpoint names have spaces in them, the entire group names needs to be encapsulated with \" (e.g. \"New Group\")

NOTE - All rules will be copied only.  Currently, you cannot update rules across multiple groups.  Running this more than once will copy the rules each time.
`,

	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		// Disable stdout
		viper.Set("output_format", "csv")
		if err := viper.WriteConfig(); err != nil {
			utils.LogError(err.Error())
		}

		edgerulescopy()
	},
}

//
func edgerulescopy() {

	// Log start of run
	utils.LogStartCommand("edge-rule-copy")

	// Get all rulesets
	rulesets, a, err := pce.GetRuleSetMapName("draft")
	utils.LogAPIResp("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Check matches for provided from and to groups
	var fromRuleSet, toRuleSet illumioapi.RuleSet
	var ok bool
	if fromRuleSet, ok = rulesets[fromGroup]; !ok {
		utils.LogError("Command requires from-Group Name to match a Group Name. If Endpoint group name has spaces ecapsulate the group with \" \". See usage help.")
	}
	if toRuleSet, ok = rulesets[toGroup]; !ok {
		utils.LogError("Command requires to-Group Name to match a Group Name. If Endpoint group name has spaces ecapsulate the group with \" \". See usage help.")
	}

	// Create map for fromRuleSet to check for rules in delete
	fromRuleSetRules := make(map[string]illumioapi.Rule)
	for _, r := range fromRuleSet.Rules {
		fromRuleSetRules[r.Href] = *r
	}

	// Create an internal struct to store to rules
	type toRuleMapEntry struct {
		href                 string
		createdAt, updatedAt time.Time
	}

	// Iterate through each toGroup rules. If there is an ExternalDataReference, calculate times, and put into map.
	// By having the refence data we can then compare the existing rules to new ones and skip over existing ones.
	rulemap := make(map[string]toRuleMapEntry)
	deleteRules := []string{}
	for _, rule := range toRuleSet.Rules {

		// Populate ruleMap if ExternalDataReference is not blank
		if rule.ExternalDataReference != "" {

			// Convert CreatedAt and UpdatedAt strings to time variables and add new toRuleMapEntry to map
			ct, err := time.Parse(time.RFC3339, rule.CreatedAt)
			if err != nil {
				utils.LogError(err.Error())
			}
			ut, err := time.Parse(time.RFC3339, rule.UpdatedAt)
			if err != nil {
				utils.LogError(err.Error())
			}
			tmpToRuleMap := toRuleMapEntry{
				href:      rule.Href,
				createdAt: ct,
				updatedAt: ut}
			rulemap[rule.ExternalDataReference] = tmpToRuleMap
		}

		// If the delete flag is set and the rule external data reference has the from ruleset href, check if the rule is still in the from ruleset. If it's not, add to the delete slice.
		if delete && strings.Contains(rule.ExternalDataReference, fromRuleSet.Href) {
			if _, ok := fromRuleSetRules[rule.ExternalDataReference]; !ok {
				fmt.Println(fromRuleSetRules)
				fmt.Println(rule.ExternalDataReference)
				deleteRules = append(deleteRules, rule.Href)
				utils.LogInfo(fmt.Sprintf("rule %s to be deleted based on %s", rule.Href, fromRuleSet.Href), false)
			}
		}
	}

	// Check to see there are rules to copy before iterating
	if len(fromRuleSet.Rules) == 0 {
		utils.LogInfo("no rules to copy", true)
		utils.LogEndCommand("edge-rule-copy")
		return
	}

	// Set variables for processing rules
	newRules := []illumioapi.Rule{}
	updatedRules := []illumioapi.Rule{}

	// Get href to replace every copied rule provider with.  Necessary for Edge.
	toLabel, a, err := pce.GetLabelbyKeyValue("role", toGroup)
	utils.LogAPIResp("GetLabelbyKeyValue", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Iterate through the fromRules
	for i, fromRule := range fromRuleSet.Rules {

		// Create the copy rule struct
		copiedRule := illumioapi.Rule{Href: "",
			Consumers:                   fromRule.Consumers,
			ConsumingSecurityPrincipals: fromRule.ConsumingSecurityPrincipals,
			Description:                 fromRule.Description,
			Enabled:                     fromRule.Enabled,
			IngressServices:             fromRule.IngressServices,
			ResolveLabelsAs:             fromRule.ResolveLabelsAs,
			Providers:                   []*illumioapi.Providers{&illumioapi.Providers{Label: &illumioapi.Label{Href: toLabel.Href}}},
			SecConnect:                  fromRule.SecConnect,
			Stateless:                   fromRule.Stateless,
			MachineAuth:                 fromRule.MachineAuth,
			UnscopedConsumers:           fromRule.UnscopedConsumers,
			ExternalDataReference:       fromRule.Href, //Store rule that this rule comes from.
		}

		// Turn UpdatedAt string into time object
		fromUpdatedTime, err := time.Parse(time.RFC3339, fromRule.UpdatedAt)
		if err != nil {
			utils.LogError(err.Error())
		}

		// If the href doesn't exist, create the rule.
		if rulemap[fromRule.Href].href == "" {
			utils.LogInfo(fmt.Sprintf("rule %d - rule to be created based on %s", i+1, fromRule.Href), false)
			newRules = append(newRules, copiedRule)
			// If the fromUpdatedTime is UpdatedAt time is before the fromUpdatedTime, replace the HREF and update the rule
		} else if rulemap[fromRule.Href].updatedAt.Before(fromUpdatedTime) {
			copiedRule.Href = rulemap[fromRule.Href].href
			utils.LogInfo(fmt.Sprintf("rule %d - %s to be updated base on %s", i+1, copiedRule.Href, fromRule.Href), false)
			updatedRules = append(updatedRules, copiedRule)
			// Otherwise, no changes
		} else {
			utils.LogInfo(fmt.Sprintf("rule %d - no change to %s with %s", i+1, rulemap[fromRule.Href].href, fromRule.Href), false)
		}
	}

	// Log the total number of rules copied
	utils.LogInfo(fmt.Sprintf("%d Rules found in group %s.", len(fromRuleSet.Rules), fromGroup), true)

	// Return if there is nothing to process
	if len(newRules)+len(updatedRules)+len(deleteRules) == 0 {
		utils.LogInfo("nothing to be done.", true)
		utils.LogEndCommand("edge-rule-copy")
		return
	}

	// If updatePCE is disabled, we are return
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("edge-rule-copy identified %d rules to create and %d rules to update. See workloader.log for details. To do the import, run agai useing --update-pce flag.", len(newRules), len(updatedRules)), true)
		utils.LogEndCommand("edge-rule-copy")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("[INFO] - edge-rule-copy will create %d rules and update %d rules. Do you want to run the import (yes/no)? ", len(newRules), len(updatedRules))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("Prompt denied for creating %d rules and updating %d rules.", len(newRules), len(updatedRules)), true)
			utils.LogEndCommand("edge-rule-copy")
			return
		}
	}

	// Create rules
	for _, nr := range newRules {
		newRule, a, err := pce.CreateRuleSetRule(toRuleSet.Href, nr)
		utils.LogAPIResp("CreateRuleSetRule", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("created new rule %s from %s - status code %d", newRule.Href, newRule.ExternalDataReference, a.StatusCode), true)
	}

	// Updated rules
	for _, ur := range updatedRules {
		a, err := pce.UpdateRuleSetRules(ur)
		utils.LogAPIResp("UpdateRuleSetRules", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("updated rule %s from %s - status code %d", ur.Href, ur.ExternalDataReference, a.StatusCode), true)
	}

	// Delete Rules
	for _, d := range deleteRules {
		a, err := pce.DeleteHref(d)
		utils.LogAPIResp("DeleteHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("delete rule %s from %s - status code %d", d, toRuleSet.Href, a.StatusCode), true)
	}

	// Provision any changes
	if !doNotProvision {
		a, err := pce.ProvisionHref([]string{toRuleSet.Href}, "workloader edge-rules-copy")
		utils.LogAPIResp("ProvisionHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning complete - status code %d", a.StatusCode), true)
	}

	utils.LogEndCommand("edge-rule-copy")
}
