package edgeruleimport

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Global variables
var matchCol, roleCol, appCol, envCol, locCol, intCol, hostnameCol, nameCol, createdLabels int
var removeValue, csvFile string
var umwl, keepAllPCEInterfaces, fqdnToHostname, doNotProvision, debug, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error
var newLabels []illumioapi.Label

func init() {
	EdgeRuleImportCmd.Flags().BoolVar(&doNotProvision, "x", false, "Do not provision changes.")
}

// EdgeRuleImportCmd runs the upload command
var EdgeRuleImportCmd = &cobra.Command{
	Use:   "edge-rule-import [csv file to import]",
	Short: "Create and update rules in an Edge PCE from a CSV file.",
	Long: `
Create and update rules in an Edge PCE from a CSV file.

To get the input format, first run a workloader edge-rule-export. Example format is below:

+--------+-----------------+----------------+---------------------+----------+----------------+-----------------+--------------+--------------+----------------------------------------------------+---------------------------------------+
| group  | consumer_iplist | consumer_group | consumer_user_group |  service | provider_group | provider_iplist | rule_enabled | machine_auth |                     rule_href                      |             ruleset_href              |
+--------+-----------------+----------------+---------------------+----------+----------------+-----------------+--------------+--------------+----------------------------------------------------+---------------------------------------+
| Admins | VPN; HQ         |                |                     | RDP; SSH | LINAPP         |                 | TRUE         | FALSE        | /orgs/1/sec_policy/draft/rule_sets/15/sec_rules/79 | /orgs/1/sec_policy/draft/rule_sets/15 |
+--------+-----------------+----------------+---------------------+----------+----------------+-----------------+--------------+--------------+----------------------------------------------------+---------------------------------------+

With no rule_href (e.g., Sales example above), the rule will be created. If there is a rule_href provided (e.g., HR example above), the rule will be updated.

A ruleset_href is always needed.

Multiple values can be provided by using a semi-colon to separate as shown above in the service and consumer_iplist examples.

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		importEdgeRules()
	},
}

// FromCSV imports a CSV to label unmanaged workloads and create unmanaged workloads
func importEdgeRules() {

	// Log start of the command
	utils.LogStartCommand("edge-rule-import")

	// Set the column integers
	groupCol := 0
	conIPLCol := 1
	conGrpCol := 2
	conUserGrpCol := 3
	svcCol := 4
	provGroupCol := 5
	provIPLCol := 6
	ruleEnabledCol := 7
	maCol := 8
	rHrefCol := 9
	rsHrefCol := 10

	// Get all the rulesets and make a map
	allRS, a, err := pce.GetAllRuleSets("draft")
	utils.LogAPIResp("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	rsNameMap := make(map[string]illumioapi.RuleSet)
	for _, rs := range allRS {
		rsNameMap[rs.Name] = rs
	}
	rsHrefMap := make(map[string]illumioapi.RuleSet)
	for _, rs := range allRS {
		rsHrefMap[rs.Href] = rs
	}
	rHrefMap := make(map[string]illumioapi.Rule)
	for _, rs := range allRS {
		for _, r := range rs.Rules {
			rHrefMap[r.Href] = *r
		}

	}

	// Get all IPLists by name
	allIPLs, a, err := pce.GetAllDraftIPLists()
	utils.LogAPIResp("GetAllDraftIPLists", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	iplMap := make(map[string]illumioapi.IPList)
	for _, ipl := range allIPLs {
		iplMap[ipl.Name] = ipl
		iplMap[ipl.Href] = ipl
	}

	// Get all services by name
	allSvcs, a, err := pce.GetAllServices("draft")
	utils.LogAPIResp("GetAllServices", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	svcNameMap := make(map[string]illumioapi.Service)
	for _, svc := range allSvcs {
		svcNameMap[svc.Name] = svc
		svcNameMap[svc.Href] = svc
	}

	// Get the user groups
	userGroups, a, err := pce.GetAllADUserGroups()
	userGroupMapName := make(map[string]illumioapi.ConsumingSecurityPrincipals)
	for _, ug := range userGroups {
		userGroupMapName[ug.Name] = ug
		userGroupMapName[ug.Href] = ug
	}

	// Parse the CSV file
	csvInput, err := utils.ParseCSV(csvFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create a toAdd data struct
	type toAdd struct {
		ruleSetHref string
		rule        illumioapi.Rule
		csvLine     int
	}
	newRules := []toAdd{}
	updatedRules := []toAdd{}

	// Iterate through the CSV Data
	for i, l := range csvInput {

		// Skip the header row
		if i == 0 {
			continue
		}
		// Reset the update
		update := false

		/******************** Ruleset and Rule existence ********************/

		// Check  if the ruleset exists
		rs := illumioapi.RuleSet{}
		rsCheck := false

		// If a ruleset HREF is provided, we make sure it exists
		if rs, rsCheck = rsHrefMap[l[rsHrefCol]]; !rsCheck {
			utils.LogWarning(fmt.Sprintf("CSV line %d - the provided ruleset href does not exist. Skipping.", i+1), true)
			continue
		}

		// Check if the rule exists
		if l[rHrefCol] != "" {
			if _, rCheck := rHrefMap[l[rHrefCol]]; !rCheck {
				utils.LogWarning(fmt.Sprintf("CSV line %d - the provided rule href does not exist. Skipping.", i+1), true)
				continue
			}
		}

		// ******************** Consumers ********************
		consumers := []*illumioapi.Consumers{}

		// IP Lists
		consCSVipls := strings.Split(strings.ReplaceAll(l[conIPLCol], "; ", ";"), ";")
		if l[conIPLCol] == "" {
			consCSVipls = nil
		}
		iplChange, ipls := iplComparison(consCSVipls, rHrefMap[l[rHrefCol]], iplMap, i+1, false)
		if iplChange {
			update = true
		}
		for _, ipl := range ipls {
			consumers = append(consumers, &illumioapi.Consumers{IPList: ipl})
		}

		// Labels - parse the CSV entry to split by semicolon and remove spaces
		csvLabels := strings.Split(strings.ReplaceAll(l[conGrpCol], "; ", ";"), ";")
		if l[conGrpCol] == "" {
			csvLabels = nil
		}
		// Labels - check for All Workloads
		for _, l := range csvLabels {
			if strings.ToLower(l) == "all workloads" {
				consumers = append(consumers, &illumioapi.Consumers{Actors: "ams"})
			}
		}

		// Labels - run comparison
		labelUpdate, labels := labelComparison("role", csvLabels, pce, rHrefMap[l[rHrefCol]], i+1, false)
		if labelUpdate {
			update = true
		}
		for _, l := range labels {
			consumers = append(consumers, &illumioapi.Consumers{Label: &illumioapi.Label{Href: l.Href}})
		}

		// User Groups - parse and run comparison
		csvUserGroups := strings.Split(strings.ReplaceAll(l[conUserGrpCol], "; ", ";"), ";")
		if l[conUserGrpCol] == "" {
			csvUserGroups = nil
		}
		ugUpdate, consumingSecPrincipals := userGroupComaprison(csvUserGroups, rHrefMap[l[rHrefCol]], userGroupMapName, i+1)
		if ugUpdate {
			update = true
		}

		// ******************** Providers ********************

		// Labels - parse the CSV entry to split by semicolon and remove spaces
		csvLabels = strings.Split(strings.ReplaceAll(l[provGroupCol], "; ", ";"), ";")
		if l[provGroupCol] == "" {
			csvLabels = nil
		}

		providers := []*illumioapi.Providers{}

		// Labels - check for All Workloads
		for _, l := range csvLabels {
			if strings.ToLower(l) == "all workloads" {
				providers = append(providers, &illumioapi.Providers{Actors: "ams"})
			}
		}

		// Labels - run comparison
		labelUpdate, labels = labelComparison("role", csvLabels, pce, rHrefMap[l[rHrefCol]], i+1, true)
		if labelUpdate {
			update = true
		}
		for _, l := range labels {
			providers = append(providers, &illumioapi.Providers{Label: &illumioapi.Label{Href: l.Href}})
		}

		// IP Lists
		provCSVipls := strings.Split(strings.ReplaceAll(l[provIPLCol], "; ", ";"), ";")
		if l[provIPLCol] == "" {
			provCSVipls = nil
		}
		iplChange, ipls = iplComparison(provCSVipls, rHrefMap[l[rHrefCol]], iplMap, i+1, true)
		if iplChange {
			update = true
		}
		for _, ipl := range ipls {
			providers = append(providers, &illumioapi.Providers{IPList: ipl})
		}

		// ******************** Services ********************

		csvServices := strings.Split(strings.ReplaceAll(l[svcCol], "; ", ";"), ";")
		if l[svcCol] == "" {
			csvServices = nil
		}
		svcChange, ingressSvc := serviceComparison(csvServices, rHrefMap[l[rHrefCol]], svcNameMap, i+1)
		if svcChange {
			update = true
		}

		// ******************** Enabled ********************
		enabled, err := strconv.ParseBool(l[ruleEnabledCol])
		if err != nil {
			utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean", i+1, l[ruleEnabledCol]))
		}
		if l[rHrefCol] != "" {
			if rHrefMap[l[rHrefCol]].Enabled != enabled {
				update = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - enabled status needs to be updated.", i), false)
			}
		}

		// ******************** Machine Auth ********************/
		machineAuth, err := strconv.ParseBool(l[maCol])
		if err != nil {
			utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean", i+1, l[maCol]))
		}
		if l[rHrefCol] != "" {
			if rHrefMap[l[rHrefCol]].MachineAuth != machineAuth {
				update = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - machine auth status needs to be updated.", i), false)
			}
		}

		// Create the rule
		csvRule := illumioapi.Rule{Consumers: consumers, ConsumingSecurityPrincipals: consumingSecPrincipals, Providers: providers, IngressServices: ingressSvc, Enabled: enabled, MachineAuth: machineAuth, ResolveLabelsAs: &illumioapi.ResolveLabelsAs{Consumers: []string{"workloads"}, Providers: []string{"workloads"}}}

		// Add to our array
		if l[rHrefCol] == "" {
			newRules = append(newRules, toAdd{ruleSetHref: rs.Href, rule: csvRule, csvLine: i + 1})
			utils.LogInfo(fmt.Sprintf("CSV line %d - create new rule for group %s", i+1, l[groupCol]), true)
		} else {
			if update {
				csvRule.Href = l[rHrefCol]
				updatedRules = append(updatedRules, toAdd{ruleSetHref: rs.Href, rule: csvRule, csvLine: i + 1})
			}
		}
	}

	// End run if we have nothing to do
	if len(newRules) == 0 && len(updatedRules) == 0 {
		utils.LogInfo("nothing to be done", true)
		utils.LogEndCommand("edge-rule-import")
		return
	}

	// Log findings
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d rules to create and %d rules to update. See workloader.log for details. To do the import, run again using --update-pce flag.", len(newRules), len(updatedRules)), true)
		utils.LogEndCommand("edge-rule-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("\r\n[PROMPT] - workloader identified %d rules to create and %d rules to update in %s (%s). Do you want to run the import (yes/no)? ", len(newRules), len(updatedRules), viper.Get("default_pce_name").(string), viper.Get(viper.Get("default_pce_name").(string)+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied.", true)
			utils.LogEndCommand("edge-rule-import")
			return
		}
	}

	// Create the new rules
	provisionHrefs := make(map[string]bool)
	if len(newRules) > 0 {
		for _, newRule := range newRules {
			rule, a, err := pce.CreateRuleSetRule(newRule.ruleSetHref, newRule.rule)
			utils.LogAPIResp("CreateRuleSetRule", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			provisionHrefs[strings.Split(rule.Href, "/sec_rules")[0]] = true
			utils.LogInfo(fmt.Sprintf("CSV line %d - created rule %s - %d", newRule.csvLine, rule.Href, a.StatusCode), true)
		}
	}

	// Update the new rules
	if len(updatedRules) > 0 {
		for _, updatedRule := range updatedRules {
			a, err := pce.UpdateRuleSetRules(updatedRule.rule)
			utils.LogAPIResp("UpdateRuleSetRules", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			provisionHrefs[strings.Split(updatedRule.rule.Href, "/sec_rules")[0]] = true
			utils.LogInfo(fmt.Sprintf("CSV line %d - updated rule %s - %d", updatedRule.csvLine, updatedRule.rule.Href, a.StatusCode), true)
		}
	}

	// Provision any changes
	p := []string{}
	for a := range provisionHrefs {
		p = append(p, a)
	}
	if !doNotProvision {
		a, err := pce.ProvisionHref(p, "workloader edge-rule-import")
		utils.LogAPIResp("ProvisionHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning complete - status code %d", a.StatusCode), true)
	}

	// Log end
	utils.LogEndCommand("edge-rule-import")
}
