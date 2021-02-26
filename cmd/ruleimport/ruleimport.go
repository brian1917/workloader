package ruleimport

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

// Input is the data structure for the ImportRulesFromCSV command
type Input struct {
	PCE                                                                                                                                                                                                        illumioapi.PCE
	ImportFile                                                                                                                                                                                                 string
	ConsRoleIndex, ConsAppIndex, ConsEnvIndex, ConsLocIndex, ConsUserGroupIndex, ProvRoleIndex, ProvAppIndex, ProvEnvIndex, ProvLocIndex, ConsIPLIndex, ConsLabelGroupIndex, ProvLabelGroupIndex, ProvIPLIndex *int
	ConsWkldIndex, ProvWkldIndex, RulesetNameIndex, GlobalConsumerIndex, ServicesIndex, RuleEnabledIndex, MachineAuthIndex, SecureConnectIndex, RuleHrefIndex                                                  *int
	ProvsVSOnlyIndex, ProvsVSandWkldIndex, ConsVSOnlyIndex, ConsVSandWkldsIndex                                                                                                                                *int
	DoNotProvision, UpdatePCE, NoPrompt                                                                                                                                                                        bool
}

// Decluare a global iput and debug variable
var input Input
var debug bool

func init() {
	RuleImportCmd.Flags().BoolVar(&input.DoNotProvision, "x", false, "Do not provision changes.")
}

// RuleImportCmd runs the upload command
var RuleImportCmd = &cobra.Command{
	Use:   "rule-import [csv file to import]",
	Short: "Create and update rules from a CSV file.",
	Long: `
Create and update rules in the PCE from a CSV file.

The input format accepts the following header values:
- ruleset_name (required. name of the target ruleset.)
- global_consumers (required. true/false. true is extra-scope and false is intra-scope.)
- consumer_role (label value. multiple separated by ";")
- consumer_app (label value. multiple separated by ";")
- consumer_env (label value. multiple separated by ";")
- consumer_loc (label value. multiple separated by ";")
- consumer_iplists (names of IP lists. multiple separated by ";")
- consumer_label_groups (names of label groups. multiple separated by ";")
- consumer_user_group (names of user groups. multiple separated by ";")
- consumer_workloads (names of workloads. multiple separated by ";")
- consumers_virtual_services_only (required. true/false)
- consumers_virtual_services_and_workloads (required. true/false)
- provider_role (label value. multiple separated by ";")
- provider_app (label value. multiple separated by ";")
- provider_env (label value. multiple separated by ";")
- provider_loc (label value. multiple separated by ";")
- provider_iplists (names of IP lists. multiple separated by ";")
- provider_workloads (names of workloads. multiple separated by ";")
- providers_virtual_services_only (required. true/false)
- providers_virtual_services_and_workloads (required. true/false)
- services (required. service name, port/proto, or port range/proto. multiple separated by ";")
- rule_enabled (required. true/false)
- machine_auth_enabled (true/false)
- secure_connect_enabled (true/false)
- rule_href (if blank, a rule is created. if provided, the rule is updated.)

The only required header value is ruleset_name. All others are optional.

If a rule_href is provided, the existing rule will be updated. If it's not provided it will be created.

An easy way to get the input format is to run the workloader ruleset-export command.

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		var err error
		input.PCE, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		input.ImportFile = args[0]

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)
		input.UpdatePCE = viper.Get("update_pce").(bool)
		input.NoPrompt = viper.Get("no_prompt").(bool)

		ImportRulesFromCSV(input)
	},
}

// ImportRulesFromCSV imports a CSV to modify/create rules
func ImportRulesFromCSV(input Input) {

	// Log start of the command
	utils.LogStartCommand("rule-import")

	// Get all the rulesets and make a map
	utils.LogInfo("Getting all rulesets...", true)
	allRS, a, err := input.PCE.GetAllRuleSets("draft")
	utils.LogAPIResp("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	rsNameMap := make(map[string]illumioapi.RuleSet)
	rsHrefMap := make(map[string]illumioapi.RuleSet)
	for _, rs := range allRS {
		rsHrefMap[rs.Href] = rs
		rsNameMap[rs.Name] = rs
	}
	rHrefMap := make(map[string]illumioapi.Rule)
	for _, rs := range allRS {
		for _, r := range rs.Rules {
			rHrefMap[r.Href] = *r
		}

	}

	// Get all IPLists by name
	utils.LogInfo("Getting all IP lists...", true)
	allIPLs, a, err := input.PCE.GetAllDraftIPLists()
	utils.LogAPIResp("GetAllDraftIPLists", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	iplMap := make(map[string]illumioapi.IPList)
	for _, ipl := range allIPLs {
		iplMap[ipl.Name] = ipl
		iplMap[ipl.Href] = ipl
	}

	// Get all label groups by name
	utils.LogInfo("Getting all label groups...", true)
	allLGs, a, err := input.PCE.GetAllLabelGroups("draft")
	utils.LogAPIResp("GetAllLabelGroups", a)
	lgMap := make(map[string]illumioapi.LabelGroup)
	for _, lg := range allLGs {
		lgMap[lg.Name] = lg
		lgMap[lg.Href] = lg
	}

	// Get all workloads by name
	utils.LogInfo("Getting all workloads...", true)
	allWklds, a, err := input.PCE.GetAllWorkloadsQP(nil)
	utils.LogAPIResp("GetAllWorkloadsQP", a)
	wkldMap := make(map[string]illumioapi.Workload)
	for _, wkld := range allWklds {
		wkldMap[wkld.Name] = wkld
		wkldMap[wkld.Href] = wkld
	}

	// Get all services by name
	utils.LogInfo("Getting all services...", true)
	allSvcs, a, err := input.PCE.GetAllServices("draft")
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
	utils.LogInfo("Getting all usergroups...", true)
	userGroups, a, err := input.PCE.GetAllADUserGroups()
	userGroupMapName := make(map[string]illumioapi.ConsumingSecurityPrincipals)
	for _, ug := range userGroups {
		userGroupMapName[ug.Name] = ug
		userGroupMapName[ug.Href] = ug
	}

	// Create a toAdd data struct
	type toAdd struct {
		ruleSetHref string
		rule        illumioapi.Rule
		csvLine     int
	}
	newRules := []toAdd{}
	updatedRules := []toAdd{}

	// Parse the CSV file
	csvInput, err := utils.ParseCSV(input.ImportFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Iterate through the CSV Data
	for i, l := range csvInput {

		// Skip the header row
		if i == 0 {
			// Process the headers
			input.processHeaders(l)
			// Log the input
			input.log()
			continue
		}
		// Reset the update
		update := false

		/******************** Ruleset and Rule existence ********************/

		// A ruleset name is required. Make sure it's provided in the CSV and exists in the PCE
		if input.RulesetNameIndex == nil {
			utils.LogWarning(fmt.Sprintf("CSV line %d - no ruleset provided. Skipping.", i+1), true)
			continue
		}
		var rs illumioapi.RuleSet
		var rsCheck bool
		if rs, rsCheck = rsNameMap[l[*input.RulesetNameIndex]]; !rsCheck {
			fmt.Println(l[*input.RulesetNameIndex])
			utils.LogWarning(fmt.Sprintf("CSV line %d - the provided ruleset name does not exist. Skipping.", i+1), true)
			continue
		}

		// If a rule href is provided, it's updated.If not, it's created.
		// Verify if a rule is provided that it exsits.
		if input.RuleHrefIndex != nil && l[*input.RuleHrefIndex] != "" {
			if _, rCheck := rHrefMap[l[*input.RuleHrefIndex]]; !rCheck {
				utils.LogWarning(fmt.Sprintf("CSV line %d - the provided rule href does not exist. Skipping.", i+1), true)
				continue
			}
		}

		// ******************** Consumers ********************
		consumers := []*illumioapi.Consumers{}

		// IP Lists
		if input.ConsIPLIndex != nil {
			consCSVipls := strings.Split(strings.ReplaceAll(l[*input.ConsIPLIndex], "; ", ";"), ";")
			if l[*input.ConsIPLIndex] == "" {
				consCSVipls = nil
			}
			iplChange, ipls := iplComparison(consCSVipls, rHrefMap[l[*input.RuleHrefIndex]], iplMap, i+1, false)
			if iplChange {
				update = true
			}
			for _, ipl := range ipls {
				consumers = append(consumers, &illumioapi.Consumers{IPList: ipl})
			}
		}

		// Workloads
		if input.ConsWkldIndex != nil {
			consCSVwklds := strings.Split(strings.ReplaceAll(l[*input.ConsWkldIndex], "; ", ";"), ";")
			if l[*input.ConsWkldIndex] == "" {
				consCSVwklds = nil
			}
			wkldChange, wklds := wkldComparison(consCSVwklds, rHrefMap[l[*input.RuleHrefIndex]], wkldMap, i+1, false)
			if wkldChange {
				update = true
			}
			for _, wkld := range wklds {
				consumers = append(consumers, &illumioapi.Consumers{Workload: wkld})
			}
		}

		// Label Groups
		if input.ConsLabelGroupIndex != nil {
			consCSVlgs := strings.Split(strings.ReplaceAll(l[*input.ConsLabelGroupIndex], "; ", ";"), ";")
			if l[*input.ConsLabelGroupIndex] == "" {
				consCSVlgs = nil
			}
			lgChange, lgs := lgComparison(consCSVlgs, rHrefMap[l[*input.RuleHrefIndex]], lgMap, i+1, false)
			if lgChange {
				update = true
			}
			for _, lg := range lgs {
				consumers = append(consumers, &illumioapi.Consumers{LabelGroup: lg})
			}
		}

		// Labels - iterate through the role, app, env and loc.
		labelIndeces := []*int{input.ConsRoleIndex, input.ConsAppIndex, input.ConsEnvIndex, input.ConsLocIndex}
		labelKeys := []string{"role", "app", "env", "loc"}
		for e, li := range labelIndeces {
			if li != nil {
				csvLabels := strings.Split(strings.ReplaceAll(l[*li], "; ", ";"), ";")
				if l[*li] == "" {
					csvLabels = nil
				}
				// Labels - check for All Workloads
				for _, l := range csvLabels {
					if strings.ToLower(l) == "all workloads" {
						consumers = append(consumers, &illumioapi.Consumers{Actors: "ams"})
					}
				}

				// Labels - run comparison
				labelUpdate, labels := labelComparison(labelKeys[e], csvLabels, input.PCE, rHrefMap[l[*input.RuleHrefIndex]], i+1, false)
				if labelUpdate {
					update = true
				}
				for _, l := range labels {
					consumers = append(consumers, &illumioapi.Consumers{Label: &illumioapi.Label{Href: l.Href}})
				}
			}
		}
		// User Groups - parse and run comparison
		var consumingSecPrincipals []*illumioapi.ConsumingSecurityPrincipals
		if input.ConsUserGroupIndex != nil {
			csvUserGroups := strings.Split(strings.ReplaceAll(l[*input.ConsUserGroupIndex], "; ", ";"), ";")
			if l[*input.ConsUserGroupIndex] == "" {
				csvUserGroups = nil
			}
			var ugUpdate bool
			ugUpdate, consumingSecPrincipals = userGroupComaprison(csvUserGroups, rHrefMap[l[*input.RuleHrefIndex]], userGroupMapName, i+1)
			if ugUpdate {
				update = true
			}
		}

		// ******************** Providers ********************

		providers := []*illumioapi.Providers{}

		// Labels - parse the CSV entry to split by semicolon and remove spaces
		provLabelIndeces := []*int{input.ProvRoleIndex, input.ProvAppIndex, input.ProvEnvIndex, input.ProvLocIndex}
		for e, li := range provLabelIndeces {
			if li != nil {
				csvLabels := strings.Split(strings.ReplaceAll(l[*li], "; ", ";"), ";")
				if l[*li] == "" {
					csvLabels = nil
				}

				// Labels - check for All Workloads
				for _, l := range csvLabels {
					if strings.ToLower(l) == "all workloads" {
						providers = append(providers, &illumioapi.Providers{Actors: "ams"})
					}
				}

				// Labels - run comparison
				labelUpdate, labels := labelComparison(labelKeys[e], csvLabels, input.PCE, rHrefMap[l[*input.RuleHrefIndex]], i+1, true)
				if labelUpdate {
					update = true
				}
				for _, l := range labels {
					providers = append(providers, &illumioapi.Providers{Label: &illumioapi.Label{Href: l.Href}})
				}
			}
		}

		// IP Lists
		if input.ProvIPLIndex != nil {
			provCSVipls := strings.Split(strings.ReplaceAll(l[*input.ProvIPLIndex], "; ", ";"), ";")
			if l[*input.ProvIPLIndex] == "" {
				provCSVipls = nil
			}
			iplChange, ipls := iplComparison(provCSVipls, rHrefMap[l[*input.RuleHrefIndex]], iplMap, i+1, true)
			if iplChange {
				update = true
			}
			for _, ipl := range ipls {
				providers = append(providers, &illumioapi.Providers{IPList: ipl})
			}
		}

		// Workloads
		if input.ProvWkldIndex != nil {
			provsCSVwklds := strings.Split(strings.ReplaceAll(l[*input.ProvWkldIndex], "; ", ";"), ";")
			if l[*input.ProvWkldIndex] == "" {
				provsCSVwklds = nil
			}
			wkldChange, wklds := wkldComparison(provsCSVwklds, rHrefMap[l[*input.RuleHrefIndex]], wkldMap, i+1, false)
			if wkldChange {
				update = true
			}
			for _, wkld := range wklds {
				providers = append(providers, &illumioapi.Providers{Workload: wkld})
			}
		}

		// Label Groups
		if input.ProvLabelGroupIndex != nil {
			provCSVlgs := strings.Split(strings.ReplaceAll(l[*input.ProvLabelGroupIndex], "; ", ";"), ";")
			if l[*input.ProvLabelGroupIndex] == "" {
				provCSVlgs = nil
			}
			lgChange, lgs := lgComparison(provCSVlgs, rHrefMap[l[*input.RuleHrefIndex]], lgMap, i+1, true)
			if lgChange {
				update = true
			}
			for _, lg := range lgs {
				providers = append(providers, &illumioapi.Providers{LabelGroup: lg})
			}
		}

		// ******************** Services ********************
		csvServices := strings.Split(strings.ReplaceAll(l[*input.ServicesIndex], "; ", ";"), ";")
		if l[*input.ServicesIndex] == "" {
			csvServices = nil
		}
		svcChange, ingressSvc := serviceComparison(csvServices, rHrefMap[l[*input.RuleHrefIndex]], svcNameMap, i+1)
		if svcChange {
			update = true
		}
		if ingressSvc == nil {
			ingressSvc = append(ingressSvc, &illumioapi.IngressServices{})
		}

		// ******************** Enabled ********************
		enabled, err := strconv.ParseBool(l[*input.RuleEnabledIndex])
		if err != nil {
			utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for rule_enabled", i+1, l[*input.RuleEnabledIndex]))
		}
		if l[*input.RuleHrefIndex] != "" {
			if *rHrefMap[l[*input.RuleHrefIndex]].Enabled != enabled {
				update = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - enabled status needs to be updated.", i), false)
			}
		}

		// ******************** Machine Auth ********************/
		var machineAuth bool
		if input.MachineAuthIndex != nil {
			machineAuth, err = strconv.ParseBool(l[*input.MachineAuthIndex])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for machine_auth_enabled", i+1, l[*input.MachineAuthIndex]))
			}
			if l[*input.RuleHrefIndex] != "" {
				if *rHrefMap[l[*input.RuleHrefIndex]].MachineAuth != machineAuth {
					update = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - machine auth status needs to be updated.", i), false)
				}
			}
		}

		// ******************** Machine Auth ********************/
		var secConnect bool
		if input.SecureConnectIndex != nil {
			secConnect, err = strconv.ParseBool(l[*input.SecureConnectIndex])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for secure_connect_enabled", i+1, l[*input.SecureConnectIndex]))
			}
			if l[*input.RuleHrefIndex] != "" {
				if *rHrefMap[l[*input.RuleHrefIndex]].SecConnect != secConnect {
					update = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - secure connect status needs to be updated.", i), false)
				}
			}
		}

		// ******************** Global Consumers ********************/
		var globalConsumers bool
		if input.GlobalConsumerIndex != nil {
			globalConsumers, err = strconv.ParseBool(l[*input.GlobalConsumerIndex])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for global_consumers", i+1, l[*input.SecureConnectIndex]))
			}
			if l[*input.RuleHrefIndex] != "" {
				if *rHrefMap[l[*input.RuleHrefIndex]].UnscopedConsumers != globalConsumers {
					update = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - unscoped_consumers needs to be updated from %t to %t.", i, !globalConsumers, globalConsumers), false)
				}
			}
		}

		// ******************** VS / Workload only ********************/

		// Consumers
		consResolve := []string{}
		if input.ConsVSOnlyIndex != nil && l[*input.ConsVSOnlyIndex] != "" {
			consVSOnly, err := strconv.ParseBool(l[*input.ConsVSOnlyIndex])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for consumer virtual services only", i+1, l[*input.ConsVSOnlyIndex]))
			}
			consVSandWkld, err := strconv.ParseBool(l[*input.ConsVSandWkldsIndex])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for consumer and workloads", i+1, l[*input.ConsVSandWkldsIndex]))
			}
			// If the rule href is provided it's an update
			if l[*input.RuleHrefIndex] != "" {
				// Populate map
				consPCE := make(map[string]bool)
				for _, t := range rHrefMap[l[*input.RuleHrefIndex]].ResolveLabelsAs.Consumers {
					consPCE[t] = true
				}
				if consVSOnly != consPCE["virtual_services"] {
					update = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - consumer_virtual_services_only needs to be update from %t to %t.", i, consPCE["virtual_services"], consVSOnly), false)
				}
				if (consVSandWkld != consPCE["virtual_services"]) && (consVSandWkld != consPCE["workloads"]) {
					update = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - consumer_workloads_and_virtual_services needs to be update from %t to %t.", i, !consVSandWkld, consVSandWkld), false)
				}
			}

			if consVSOnly {
				consResolve = append(consResolve, "virtual_services")
			} else if consVSandWkld {
				consResolve = append(consResolve, "virtual_services")
				consResolve = append(consResolve, "workloads")
			} else {
				consResolve = append(consResolve, "workloads")
			}
		}

		// Providers
		provsResolve := []string{}
		provsVSOnly, err := strconv.ParseBool(l[*input.ProvsVSOnlyIndex])
		if err != nil {
			utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for provsumer virtual services only", i+1, l[*input.ProvsVSOnlyIndex]))
		}
		provsVSandWkld, err := strconv.ParseBool(l[*input.ProvsVSandWkldIndex])
		if err != nil {
			utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for provsumer and workloads", i+1, l[*input.ProvsVSandWkldIndex]))
		}
		// If the rule href is provided it's an update
		if l[*input.RuleHrefIndex] != "" {
			// Populate map
			provsPCE := make(map[string]bool)
			for _, t := range rHrefMap[l[*input.RuleHrefIndex]].ResolveLabelsAs.Providers {
				provsPCE[t] = true
			}
			if provsVSOnly != provsPCE["virtual_services"] {
				update = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - provsumer_virtual_services_only needs to be update from %t to %t.", i, provsPCE["virtual_services"], provsVSOnly), false)
			}
			if (provsVSandWkld != provsPCE["virtual_services"]) && (provsVSandWkld != provsPCE["workloads"]) {
				update = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - provsumer_workloads_and_virtual_services needs to be update from %t to %t.", i, !provsVSandWkld, provsVSandWkld), false)
			}
		}

		if provsVSOnly {
			provsResolve = append(provsResolve, "virtual_services")
		} else if provsVSandWkld {
			provsResolve = append(provsResolve, "virtual_services")
			provsResolve = append(provsResolve, "workloads")
		} else {
			provsResolve = append(provsResolve, "workloads")
		}

		// Create the rule
		csvRule := illumioapi.Rule{UnscopedConsumers: &globalConsumers, Consumers: consumers, ConsumingSecurityPrincipals: consumingSecPrincipals, Providers: providers, IngressServices: &ingressSvc, Enabled: &enabled, MachineAuth: &machineAuth, ResolveLabelsAs: &illumioapi.ResolveLabelsAs{Consumers: consResolve, Providers: provsResolve}}

		// Add to our array
		// Option 1 - No rule HREF provided, so it's a new rule
		if l[*input.RuleHrefIndex] == "" {
			newRules = append(newRules, toAdd{ruleSetHref: rs.Href, rule: csvRule, csvLine: i + 1})
			utils.LogInfo(fmt.Sprintf("CSV line %d - create new rule for %s ruleset", i+1, l[*input.RulesetNameIndex]), true)
		} else {
			// Option 2 - No rule href and update set, add to updated rules
			if update {
				csvRule.Href = l[*input.RuleHrefIndex]
				updatedRules = append(updatedRules, toAdd{ruleSetHref: rs.Href, rule: csvRule, csvLine: i + 1})
			}
		}
	}

	// End run if we have nothing to do
	if len(newRules) == 0 && len(updatedRules) == 0 {
		utils.LogInfo("nothing to be done", true)
		utils.LogEndCommand("rule-import")
		return
	}

	// Log findings
	if !input.UpdatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d rules to create and %d rules to update. See workloader.log for details. To do the import, run again using --update-pce flag.", len(newRules), len(updatedRules)), true)
		utils.LogEndCommand("rule-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if input.UpdatePCE && !input.NoPrompt {
		var prompt string
		fmt.Printf("\r\n[PROMPT] - workloader identified %d rules to create and %d rules to update in %s (%s). Do you want to run the import (yes/no)? ", len(newRules), len(updatedRules), viper.Get("default_pce_name").(string), viper.Get(viper.Get("default_pce_name").(string)+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied.", true)
			utils.LogEndCommand("rule-import")
			return
		}
	}

	// Create the new rules
	provisionHrefs := make(map[string]bool)
	if len(newRules) > 0 {
		for _, newRule := range newRules {
			rule, a, err := input.PCE.CreateRuleSetRule(newRule.ruleSetHref, newRule.rule)
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
			a, err := input.PCE.UpdateRuleSetRules(updatedRule.rule)
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
	if !input.DoNotProvision {
		a, err := input.PCE.ProvisionHref(p, "workloader edge-rule-import")
		utils.LogAPIResp("ProvisionHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning complete - status code %d", a.StatusCode), true)
	}

	// Log end
	utils.LogEndCommand("rule-import")
}
