package edgerulecopy

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var updatePCE, noPrompt, delete, doNotProvision, ignoreRuleUpdate bool
var fromGroup, toGroup, toGroupFile string
var pce illumioapi.PCE
var err error

func init() {
	EdgeRuleCopyCmd.Flags().StringVarP(&fromGroup, "from-group", "f", "", "Name of Endpoint group to copy rules from. Required")
	EdgeRuleCopyCmd.MarkFlagRequired("from-group")
	EdgeRuleCopyCmd.Flags().StringVarP(&toGroup, "to-group", "t", "", "Name of Endpoint group to create rules from rules copied from other Endpoint group.")
	EdgeRuleCopyCmd.Flags().StringVarP(&toGroupFile, "to-group-file", "l", "", "Name of file with list of groups to copy rules to.")
	// EdgeRuleCopyCmd.MarkFlagRequired("to-group")
	EdgeRuleCopyCmd.Flags().BoolVarP(&delete, "delete", "d", false, "Delete rules that were copied previously from the from-group to the to-group but are no longer in the from-group.")
	EdgeRuleCopyCmd.Flags().BoolVarP(&ignoreRuleUpdate, "ignore-rule-update", "i", false, "Delete rules that were copied previously from the from-group to the to-group but are no longer in the from-group.")
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
If Endpoint names have spaces in them, the entire group names needs to be encapsulated with "" (e.g. "New Group")

NOTE - All rules will be copied only.  Currently, you cannot update rules across multiple groups.  Running this more than once will copy the rules each time.
`,

	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the debug value from viper
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

// This is used for generating a UniqueID in the external data reference
func generateUniqueID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		utils.LogError(err.Error())
	}
	uuid := fmt.Sprintf("%x-%x-%x-%x-%x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
	return uuid
}

func trimUniqueID(s string) string {
	return strings.Split(s, ":::")[0]
}

func edgerulescopy() {

	// Log start of run
	utils.LogStartCommand("edge-rule-copy")

	// Build the toGroupList
	toGroupList := []string{}

	// Add the to group to it if we are not using the file
	if toGroup != "" {
		toGroupList = append(toGroupList, toGroup)
	}

	// Pasrse the from group file
	if toGroupFile != "" {
		data, err := utils.ParseCSV(toGroupFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		for _, l := range data {
			toGroupList = append(toGroupList, l[0])
		}
	}

	// Check to make sure we have at least one entry in the to group
	if len(toGroupList) < 1 {
		utils.LogError("Either --to-group (-t) or --to-group-file (-l) must be used.")
	}

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

	// Log the total number of rules copied
	utils.LogInfo(fmt.Sprintf("%d Rules found in group %s.", len(fromRuleSet.Rules), fromGroup), true)

	// Create map for fromRuleSet to check for rules in delete
	fromRuleSetRules := make(map[string]illumioapi.Rule)
	for _, r := range fromRuleSet.Rules {
		fromRuleSetRules[r.Href] = *r
	}

	// Create an internal struct to store to rules
	type toRuleMapEntry struct {
		href      string
		updatedAt time.Time
	}

	// newRule struct
	type newRule struct {
		rule        illumioapi.Rule
		rulesetHref string
	}

	// Set variables for processing rules
	newRules := []newRule{}
	updatedRules := []illumioapi.Rule{}
	deleteRules := []string{}

	// Iterate through each to group
	for n, t := range toGroupList {
		// Reset values
		nDelete, nUpdate, nCreate := 0, 0, 0

		// Check to make sure the toRuleSet exists
		if toRuleSet, ok = rulesets[t]; !ok {
			utils.LogWarning(fmt.Sprintf("%s is not a group. If Endpoint group name has spaces ecapsulate the group with \"\"", t), true)
			continue
		}

		// toRuleMap will have a key of fromRuleSet.Href (populated by the external data reference)
		toRuleMap := make(map[string]toRuleMapEntry)

		// Iterate through each toGroup rules. If there is an ExternalDataReference, calculate times, and put into map.
		// By having the refence data we can then compare the existing rules to new ones and skip over existing ones.
		for _, rule := range toRuleSet.Rules {

			// If the delete flag is set and the rule external data reference has the from ruleset href, check if the rule is still in the from ruleset. If it's not, add to the delete slice.
			if delete && strings.Contains(rule.ExternalDataReference, fromRuleSet.Href) {
				if _, ok := fromRuleSetRules[trimUniqueID(rule.ExternalDataReference)]; !ok {
					deleteRules = append(deleteRules, rule.Href)
					nDelete++
					utils.LogInfo(fmt.Sprintf("rule %s to be deleted based on %s", rule.Href, fromRuleSet.Href), false)
				}
			}

			// If the rule external data reference is blank, do nothing else.
			if rule.ExternalDataReference == "" {
				continue
			}

			// Convert UpdatedAt string to time variables and add new toRuleMapEntry to map
			ut, err := time.Parse(time.RFC3339, rule.UpdatedAt)
			if err != nil {
				utils.LogError(err.Error())
			}
			toRuleMap[trimUniqueID(rule.ExternalDataReference)] = toRuleMapEntry{href: rule.Href, updatedAt: ut}
		}

		// Check to see there are rules to copy before iterating
		if len(fromRuleSet.Rules) == 0 {
			utils.LogInfo(fmt.Sprintf("no rules to copy from %s", t), true)
			continue
		}

		// Get href to replace every copied rule provider with. Edge ruleset names are based on the group (role label).
		toLabel, a, err := pce.GetLabelbyKeyValue("role", t)
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
				ExternalDataReference:       fmt.Sprintf("%s:::%s", fromRule.Href, generateUniqueID()), //Store rule that this rule comes from.
			}

			// Turn UpdatedAt string into time object
			fromUpdatedTime, err := time.Parse(time.RFC3339, fromRule.UpdatedAt)
			if err != nil {
				utils.LogError(err.Error())
			}

			// If the href doesn't exist, create the rule.
			if toRuleMap[fromRule.Href].href == "" {
				utils.LogInfo(fmt.Sprintf("rule %d - rule to be created based on %s", i+1, fromRule.Href), false)
				newRules = append(newRules, newRule{rule: copiedRule, rulesetHref: toRuleSet.Href})
				nCreate++
				// If the fromUpdatedTime is UpdatedAt time is before the fromUpdatedTime, replace the HREF and update the rule
			} else if toRuleMap[fromRule.Href].updatedAt.Before(fromUpdatedTime) || !ignoreRuleUpdate {
				copiedRule.Href = toRuleMap[fromRule.Href].href
				utils.LogInfo(fmt.Sprintf("rule %d - %s to be updated base on %s", i+1, copiedRule.Href, fromRule.Href), false)
				updatedRules = append(updatedRules, copiedRule)
				nUpdate++
				// Otherwise, no changes
			} else {
				utils.LogInfo(fmt.Sprintf("rule %d - no change to %s with %s", i+1, toRuleMap[fromRule.Href].href, fromRule.Href), false)
			}
		}
		utils.LogInfo(fmt.Sprintf("%s (group %d of %d) - %d rules to be created, %d rules to updated, %d rules to be deleted", t, n+1, len(toGroupList), nCreate, nUpdate, nDelete), true)

	}

	// Return if there is nothing to process
	if len(newRules)+len(updatedRules)+len(deleteRules) == 0 {
		utils.LogInfo("nothing to be done.", true)
		utils.LogEndCommand("edge-rule-copy")
		return
	}

	// If updatePCE is disabled, we are return
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d rules to create and %d rules to update. See workloader.log for details. To do the import, run agai useing --update-pce flag.", len(newRules), len(updatedRules)), true)
		utils.LogEndCommand("edge-rule-copy")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("[PROMPT] - workloader will create %d rules and update %d rules in %s (%s). Do you want to run the import (yes/no)? ", len(newRules), len(updatedRules), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("Prompt denied for creating %d rules and updating %d rules.", len(newRules), len(updatedRules)), true)
			utils.LogEndCommand("edge-rule-copy")
			return
		}
	}

	// Create a provision slice
	provisionHrefs := make(map[string]bool)

	// Create rules
	for _, nr := range newRules {
		newRule, a, err := pce.CreateRuleSetRule(nr.rulesetHref, nr.rule)
		utils.LogAPIResp("CreateRuleSetRule", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		provisionHrefs[strings.Split(newRule.Href, "/sec_rules")[0]] = true
		utils.LogInfo(fmt.Sprintf("created new rule %s from %s - status code %d", newRule.Href, trimUniqueID(newRule.ExternalDataReference), a.StatusCode), true)
	}

	// Updated rules
	for _, ur := range updatedRules {
		a, err := pce.UpdateRuleSetRules(ur)
		utils.LogAPIResp("UpdateRuleSetRules", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		provisionHrefs[strings.Split(ur.Href, "/sec_rules")[0]] = true
		utils.LogInfo(fmt.Sprintf("updated rule %s from %s - status code %d", ur.Href, trimUniqueID(ur.ExternalDataReference), a.StatusCode), true)
	}

	// Delete Rules
	for _, d := range deleteRules {
		a, err := pce.DeleteHref(d)
		utils.LogAPIResp("DeleteHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		provisionHrefs[strings.Split(d, "/sec_rules")[0]] = true
		utils.LogInfo(fmt.Sprintf("delete rule %s from %s - status code %d", d, toRuleSet.Href, a.StatusCode), true)
	}

	// Provision any changes
	p := []string{}
	for a := range provisionHrefs {
		p = append(p, a)
	}
	if !doNotProvision {
		a, err := pce.ProvisionHref(p, "workloader edge-rules-copy")
		utils.LogAPIResp("ProvisionHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning complete - status code %d", a.StatusCode), true)
	}

	utils.LogEndCommand("edge-rule-copy")
}
