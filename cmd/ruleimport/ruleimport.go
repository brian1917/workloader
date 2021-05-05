package ruleimport

// Pending items
// Add virtual services
// Add virtual servers

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/cmd/ruleexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Input is the data structure for the ImportRulesFromCSV command
type Input struct {
	PCE                                          illumioapi.PCE
	ImportFile                                   string
	ProvisionComment                             string
	Headers                                      map[string]int
	Provision, UpdatePCE, NoPrompt, CreateLabels bool
}

// Decluare a global iput and debug variable
var globalInput Input

func init() {
	RuleImportCmd.Flags().BoolVar(&globalInput.CreateLabels, "create-labels", false, "Create labels if they do not exist.")
	RuleImportCmd.Flags().BoolVar(&globalInput.Provision, "provision", false, "Provision rule changes.")
	RuleImportCmd.Flags().StringVar(&globalInput.ProvisionComment, "provision-comment", "", "Comment for when provisioning changes.")
}

// RuleImportCmd runs the upload command
var RuleImportCmd = &cobra.Command{
	Use:   "rule-import [csv file to import]",
	Short: "Create and update rules from a CSV file.",
	Long: `
Create and update rules in the PCE from a CSV file.

An easy way to get the input format is to run the workloader rule-export command.

If a rule_href is provided, the existing rule will be updated. If it's not provided it will be created.

The order of the CSV columns do not matter. The input format accepts the following header values:
- ruleset_name (required. name of the target ruleset.)
- rule_enabled (required. true/false)
- unscoped_consumers (required. true/false. true is extra-scope and false is intra-scope.)
- consumer_all_workloads (true/false)
- consumer_roles (label value. multiple separated by ";")
- consumer_apps (label value. multiple separated by ";")
- consumer_envs (label value. multiple separated by ";")
- consumer_locs (label value. multiple separated by ";")
- consumer_iplists (names of IP lists. multiple separated by ";")
- consumer_label_groups (names of label groups. multiple separated by ";")
- consumer_user_groups (names of user groups. multiple separated by ";")
- consumer_workloads (names of workloads. multiple separated by ";")
- consumer_virtual_services
- consumer_resolve_labels_as (required. valid options are "workloads", "virtual_services", or "workloads;virtual_services")
- provider_all_workloads (true/false)
- provider_roles (label value. multiple separated by ";")
- provider_apps (label value. multiple separated by ";")
- provider_envs (label value. multiple separated by ";")
- provider_locs (label value. multiple separated by ";")
- provider_iplists (names of IP lists. multiple separated by ";")
- provider_workloads (names of workloads. multiple separated by ";")
- provider_virtual_services
- provider_resolve_labels_as (required. valid options are "workloads", "virtual_services", or "workloads;virtual_services")
- services (required. service name, port/proto, or port range/proto. multiple separated by ";")
- machine_auth_enabled (true/false)
- secure_connect_enabled (true/false)
- stateless (true/false)
- rule_href (if blank, a rule is created. if provided, the rule is updated.)

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		var err error
		globalInput.PCE, err = utils.GetTargetPCE(false)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		globalInput.ImportFile = args[0]

		// Get the debug value from viper
		globalInput.UpdatePCE = viper.Get("update_pce").(bool)
		globalInput.NoPrompt = viper.Get("no_prompt").(bool)

		ImportRulesFromCSV(globalInput)
	},
}

// ImportRulesFromCSV imports a CSV to modify/create rules
func ImportRulesFromCSV(input Input) {

	// Log start of the command
	utils.LogStartCommand("rule-import")

	// Set the global as the local for when it comes from other functions
	globalInput = input

	// Parse the CSV file
	csvInput, err := utils.ParseCSV(input.ImportFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Process headers and check if any entry in the CSV has workloads, virtual servers, or virtual services.
	var needWklds, needVirtualServices, needVirtualServers, needLabelGroups, needUserGroups bool
	csvRuleSetChecker := make(map[string]bool)

	// Create neededObjects for logging
	neededObjects := map[string]bool{"labels": true, "ip_lists": true, "services": true}

	for i, l := range csvInput {
		// Skip the header row
		if i == 0 {
			// Process the headers
			input.processHeaders(l)
			continue
		}
		// Add to the checker map
		csvRuleSetChecker[l[input.Headers[ruleexport.HeaderRulesetName]]] = true
		if index, ok := input.Headers[ruleexport.HeaderProviderWorkloads]; ok && l[index] != "" {
			neededObjects["workloads"] = true
			needWklds = true
		}
		if index, ok := input.Headers[ruleexport.HeaderConsumerWorkloads]; ok && l[index] != "" {
			neededObjects["workloads"] = true
			needWklds = true
		}
		if index, ok := input.Headers[ruleexport.HeaderProviderVirtualServices]; ok && l[index] != "" {
			neededObjects["virtual_services"] = true
			needVirtualServices = true
		}
		if index, ok := input.Headers[ruleexport.HeaderConsumerVirtualServices]; ok && l[index] != "" {
			neededObjects["virtual_services"] = true
			needVirtualServices = true
		}
		if index, ok := input.Headers[ruleexport.HeaderProviderVirtualServers]; ok && l[index] != "" {
			neededObjects["virtual_servers"] = true
			needVirtualServers = true
		}
		if index, ok := input.Headers[ruleexport.HeaderConsumerLabelGroup]; ok && l[index] != "" {
			neededObjects["label_groups"] = true
			needLabelGroups = true
		}
		if index, ok := input.Headers[ruleexport.HeaderProviderLabelGroups]; ok && l[index] != "" {
			neededObjects["label_groups"] = true
			needLabelGroups = true
		}
		if index, ok := input.Headers[ruleexport.HeaderConsumerUserGroups]; ok && l[index] != "" {
			neededObjects["consuming_security_principals"] = true
			needUserGroups = true
		}
	}
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
			// If the ruleset is in our CSV, check if it has label groups, workloads, virtual services, or virtual servers.
			if csvRuleSetChecker[rs.Name] {
				// Iterate through consumers to see if any consumers have virtual services or workloads
				for _, c := range r.Consumers {
					if c.VirtualService != nil {
						neededObjects["virtual_services"] = true
						needVirtualServices = true
					}
					if c.Workload != nil {
						neededObjects["workloads"] = true
						needWklds = true
					}
					if c.LabelGroup != nil {
						neededObjects["label_groups"] = true
						needLabelGroups = true
					}
					if r.ConsumingSecurityPrincipals != nil && len(r.ConsumingSecurityPrincipals) > 0 {
						neededObjects["consuming_security_principals"] = true
						needUserGroups = true
					}
				}
				// Iterate through providers to see if any providers have virtual servers, virtual services, or workloads
				for _, p := range r.Providers {
					if p.VirtualServer != nil {
						neededObjects["virtual_servers"] = true
						needVirtualServers = true
					}
					if p.VirtualService != nil {
						neededObjects["virtual_services"] = true
						needVirtualServices = true
					}
					if p.Workload != nil {
						neededObjects["workloads"] = true
						needWklds = true
					}
					if p.LabelGroup != nil {
						neededObjects["label_groups"] = true
						needLabelGroups = true
					}
				}
			}
		}

	}

	// Get the objects we need
	neededObjectsSlice := []string{}
	for n := range neededObjects {
		neededObjectsSlice = append(neededObjectsSlice, n)
	}
	utils.LogInfo(fmt.Sprintf("getting %s ...", strings.Join(neededObjectsSlice, ", ")), true)
	if err := input.PCE.Load(illumioapi.LoadInput{
		ProvisionStatus:             "draft",
		Labels:                      true,
		IPLists:                     true,
		Services:                    true,
		Workloads:                   needWklds,
		LabelGroups:                 needLabelGroups,
		VirtualServers:              needVirtualServers,
		VirtualServices:             needVirtualServices,
		ConsumingSecurityPrincipals: needUserGroups,
	}); err != nil {
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
CSVEntries:
	for i, l := range csvInput {

		// Skip the header row
		if i == 0 {
			// Process the headers
			continue CSVEntries
		}
		// Reset the update
		update := false

		// Set the rowRuleHref

		/******************** Ruleset and Rule existence ********************/

		// A ruleset name is required. Make sure it's provided in the CSV and exists in the PCE
		var rs illumioapi.RuleSet
		var rsCheck bool
		if rs, rsCheck = rsNameMap[l[input.Headers[ruleexport.HeaderRulesetName]]]; !rsCheck {
			utils.LogWarning(fmt.Sprintf("CSV line %d - %s ruleset_name does not exist. Skipping.", i+1, l[input.Headers[ruleexport.HeaderRulesetName]]), true)
			continue
		}

		// If a rule href is provided, it's updated. If not, it's created.
		// Verify if a rule is provided that it exsits.
		rowRuleHref := ""
		if c, ok := input.Headers[ruleexport.HeaderRuleHref]; ok && l[c] != "" {
			rowRuleHref = l[c]
			if _, rCheck := rHrefMap[l[input.Headers[ruleexport.HeaderRuleHref]]]; !rCheck {
				utils.LogWarning(fmt.Sprintf("CSV line %d - %s rule_href does not exist. Skipping.", i+1, l[input.Headers[ruleexport.HeaderRuleHref]]), true)
				continue
			}
		}

		// ******************** Consumers ********************
		consumers := []*illumioapi.Consumers{}

		// All workloads
		if c, ok := input.Headers[ruleexport.HeaderConsumerAllWorkloads]; ok {
			csvAllWorkloads, err := strconv.ParseBool(l[c])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for consumer_all_workloads", i+1, l[c]))
			}
			if rule, ok := rHrefMap[rowRuleHref]; ok {
				pceAllWklds := false
				for _, cons := range rule.Consumers {
					if cons.Actors == "ams" {
						pceAllWklds = true
					}
				}
				if pceAllWklds != csvAllWorkloads {
					utils.LogInfo(fmt.Sprintf("CSV line %d - consumer_all_workloads needs to be updated from %t to %t", i+1, pceAllWklds, csvAllWorkloads), false)
					update = true
				}
			}
			if csvAllWorkloads {
				consumers = append(consumers, &illumioapi.Consumers{Actors: "ams"})
			}
		}

		// IP Lists
		if c, ok := input.Headers[ruleexport.HeaderConsumerIplists]; ok {
			consCSVipls := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
			if l[c] == "" {
				consCSVipls = nil
			}
			iplChange, ipls := iplComparison(consCSVipls, rHrefMap[rowRuleHref], input.PCE.IPLists, i+1, false)
			if iplChange {
				update = true
			}
			for _, ipl := range ipls {
				consumers = append(consumers, &illumioapi.Consumers{IPList: ipl})
			}
		}

		// Workloads
		if c, ok := input.Headers[ruleexport.HeaderConsumerWorkloads]; ok {
			consCSVwklds := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
			if l[c] == "" {
				consCSVwklds = nil
			}
			wkldChange, wklds := wkldComparison(consCSVwklds, rHrefMap[rowRuleHref], input.PCE.Workloads, i+1, false)
			if wkldChange {
				update = true
			}
			for _, wkld := range wklds {
				consumers = append(consumers, &illumioapi.Consumers{Workload: wkld})
			}
		}

		// Virtual Services
		if c, ok := input.Headers[ruleexport.HeaderConsumerVirtualServices]; ok {
			consCSVVSs := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
			if l[c] == "" {
				consCSVVSs = nil
			}
			vsChange, virtualServices := virtualServiceCompare(consCSVVSs, rHrefMap[rowRuleHref], input.PCE.VirtualServices, i+1, false)
			if vsChange {
				update = true
			}
			for _, vs := range virtualServices {
				consumers = append(consumers, &illumioapi.Consumers{VirtualService: vs})
			}
		}

		// Label Groups
		if c, ok := input.Headers[ruleexport.HeaderConsumerLabelGroup]; ok {
			consCSVlgs := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
			if l[c] == "" {
				consCSVlgs = nil
			}
			lgChange, lgs := lgComparison(consCSVlgs, rHrefMap[rowRuleHref], input.PCE.LabelGroups, i+1, false)
			if lgChange {
				update = true
			}
			for _, lg := range lgs {
				consumers = append(consumers, &illumioapi.Consumers{LabelGroup: lg})
			}
		}

		// Labels - iterate through the role, app, env and loc.
		labelIndeces := []string{ruleexport.HeaderConsumerRole, ruleexport.HeaderConsumerApp, ruleexport.HeaderConsumerEnv, ruleexport.HeaderConsumerLoc}
		labelKeys := []string{"role", "app", "env", "loc"}
		for e, li := range labelIndeces {
			if c, ok := input.Headers[li]; ok {
				csvLabels := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
				if l[c] == "" {
					csvLabels = nil
				}

				// Labels - run comparison
				labelUpdate, labels := labelComparison(labelKeys[e], csvLabels, input.PCE, rHrefMap[rowRuleHref], i+1, false)
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
		if c, ok := input.Headers[ruleexport.HeaderConsumerUserGroups]; ok {
			csvUserGroups := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
			if l[c] == "" {
				csvUserGroups = nil
			}
			var ugUpdate bool
			ugUpdate, consumingSecPrincipals = userGroupComaprison(csvUserGroups, rHrefMap[rowRuleHref], input.PCE.ConsumingSecurityPrincipals, i+1)
			if ugUpdate {
				update = true
			}
		}

		// ******************** Providers ********************

		providers := []*illumioapi.Providers{}

		// All workloads
		if c, ok := input.Headers[ruleexport.HeaderProviderAllWorkloads]; ok {
			csvAllWorkloads, err := strconv.ParseBool(l[c])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for provider_all_workloads", i+1, l[c]))
			}
			if rule, ok := rHrefMap[rowRuleHref]; ok {
				pceAllWklds := false
				for _, prov := range rule.Providers {
					if prov.Actors == "ams" {
						pceAllWklds = true
					}
				}
				if pceAllWklds != csvAllWorkloads {
					utils.LogInfo(fmt.Sprintf("CSV line %d - provider_all_workloads needs to be updated from %t to %t", i+1, pceAllWklds, csvAllWorkloads), false)
					update = true
				}
			}
			if csvAllWorkloads {
				providers = append(providers, &illumioapi.Providers{Actors: "ams"})
			}
		}

		// Labels - parse the CSV entry to split by semicolon and remove spaces
		provLabelIndeces := []string{ruleexport.HeaderProviderRole, ruleexport.HeaderProviderApp, ruleexport.HeaderProviderEnv, ruleexport.HeaderProviderLoc}
		for e, li := range provLabelIndeces {
			if c, ok := input.Headers[li]; ok {
				csvLabels := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
				if l[c] == "" {
					csvLabels = nil
				}

				// Labels - run comparison
				labelUpdate, labels := labelComparison(labelKeys[e], csvLabels, input.PCE, rHrefMap[rowRuleHref], i+1, true)
				if labelUpdate {
					update = true
				}
				for _, l := range labels {
					providers = append(providers, &illumioapi.Providers{Label: &illumioapi.Label{Href: l.Href}})
				}
			}
		}

		// IP Lists
		if c, ok := input.Headers[ruleexport.HeaderProviderIplists]; ok {
			provCSVipls := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
			if l[c] == "" {
				provCSVipls = nil
			}
			iplChange, ipls := iplComparison(provCSVipls, rHrefMap[rowRuleHref], input.PCE.IPLists, i+1, true)
			if iplChange {
				update = true
			}
			for _, ipl := range ipls {
				providers = append(providers, &illumioapi.Providers{IPList: ipl})
			}
		}

		// Workloads
		if c, ok := input.Headers[ruleexport.HeaderProviderWorkloads]; ok {
			provsCSVwklds := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
			if l[c] == "" {
				provsCSVwklds = nil
			}
			wkldChange, wklds := wkldComparison(provsCSVwklds, rHrefMap[rowRuleHref], input.PCE.Workloads, i+1, true)
			if wkldChange {
				update = true
			}
			for _, wkld := range wklds {
				providers = append(providers, &illumioapi.Providers{Workload: wkld})
			}
		}

		// Virtual Services
		if c, ok := input.Headers[ruleexport.HeaderProviderVirtualServices]; ok {
			provCSVVSs := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
			if l[c] == "" {
				provCSVVSs = nil
			}
			vsChange, virtualServices := virtualServiceCompare(provCSVVSs, rHrefMap[rowRuleHref], input.PCE.VirtualServices, i+1, true)
			if vsChange {
				update = true
			}
			for _, vs := range virtualServices {
				providers = append(providers, &illumioapi.Providers{VirtualService: vs})
			}
		}

		// Label Groups
		if c, ok := input.Headers[ruleexport.HeaderProviderLabelGroups]; ok {
			provCSVlgs := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
			if l[c] == "" {
				provCSVlgs = nil
			}
			lgChange, lgs := lgComparison(provCSVlgs, rHrefMap[rowRuleHref], input.PCE.LabelGroups, i+1, true)
			if lgChange {
				update = true
			}
			for _, lg := range lgs {
				providers = append(providers, &illumioapi.Providers{LabelGroup: lg})
			}
		}

		// ******************** Services ********************
		var ingressSvc []*illumioapi.IngressServices
		var svcChange bool
		if c, ok := input.Headers[ruleexport.HeaderServices]; ok {
			csvServices := strings.Split(strings.ReplaceAll(l[c], "; ", ";"), ";")
			if l[c] == "" {
				csvServices = nil
			}
			svcChange, ingressSvc = serviceComparison(csvServices, rHrefMap[rowRuleHref], input.PCE.Services, i+1)
			if svcChange {
				update = true
			}
			if ingressSvc == nil {
				ingressSvc = append(ingressSvc, &illumioapi.IngressServices{})
			}
		}

		// ******************** Enabled ********************
		var enabled bool
		if c, ok := input.Headers[ruleexport.HeaderRuleEnabled]; ok {
			enabled, err = strconv.ParseBool(l[c])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for rule_enabled", i+1, l[c]))
			}
			if rowRuleHref != "" && *rHrefMap[rowRuleHref].Enabled != enabled {
				update = true
				utils.LogInfo(fmt.Sprintf("CSV line %d - rule_enabled needs to be updated from %t to %t.", i+1, !enabled, enabled), false)
			}

		}

		// ******************** Machine Auth ********************/
		var machineAuth bool
		if c, ok := input.Headers[ruleexport.HeaderMachineAuthEnabled]; ok {
			machineAuth, err = strconv.ParseBool(l[c])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for machine_auth_enabled", i+1, l[c]))
			}
			if rowRuleHref != "" {
				if *rHrefMap[rowRuleHref].MachineAuth != machineAuth {
					update = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - machine_auth_enabled needs to be updated from %t to %t.", i+1, !machineAuth, machineAuth), false)
				}
			}
		}

		// ******************** Secure Connect ********************/
		var secConnect bool
		if c, ok := input.Headers[ruleexport.HeaderSecureConnectEnabled]; ok {
			secConnect, err = strconv.ParseBool(l[c])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for secure_connect_enabled", i+1, l[c]))
			}
			if rowRuleHref != "" {
				if *rHrefMap[rowRuleHref].SecConnect != secConnect {
					update = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - secure_connect_enabled needs to be updated from %t to %t.", i+1, !secConnect, secConnect), false)
				}
			}
		}

		// ******************** Global Consumers ********************/
		var unscopedConsumers bool
		if c, ok := input.Headers[ruleexport.HeaderUnscopedConsumers]; ok {
			unscopedConsumers, err = strconv.ParseBool(l[c])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid boolean for unscoped_consumers", i+1, l[c]))
			}
			if rowRuleHref != "" {
				if *rHrefMap[rowRuleHref].UnscopedConsumers != unscopedConsumers {
					update = true
					utils.LogInfo(fmt.Sprintf("CSV line %d - unscoped_consumers needs to be updated from %t to %t.", i+1, !unscopedConsumers, unscopedConsumers), false)
				}
			}
		}

		// ******************** VS / Workload only ********************/

		headers := []string{ruleexport.HeaderConsumerResolveLabelsAs, ruleexport.HeaderProviderResolveLabelsAs}
		pceRuleResolveAs := [][]string{}
		if rule, ok := rHrefMap[rowRuleHref]; ok {
			pceRuleResolveAs = [][]string{rule.ResolveLabelsAs.Consumers, rule.ResolveLabelsAs.Providers}
		}
		var consResolveAs, provResolveAs []string
		targets := []*[]string{&consResolveAs, &provResolveAs}
		for z, h := range headers {
			csvResolveAs := strings.ToLower(strings.Replace(l[input.Headers[h]], " ", "", -1))
			csvResolveAsSlc := strings.Split(csvResolveAs, ";")
			// Make sure the provided values are valid
			for _, r := range csvResolveAsSlc {
				if r != "workloads" && r != "virtual_services" {
					utils.LogWarning(fmt.Sprintf("CSV line %d - %s is an invalid %s. Value must be workloads, virtual_services, or workloads;virtual_services", i+1, r, h), true)
					continue CSVEntries
				}
			}
			// Log if we need to make changes
			if rowRuleHref != "" {
				// If one length is 2 and other is not, we need to update
				if len(pceRuleResolveAs[z]) == 2 && len(csvResolveAsSlc) != 2 {
					utils.LogInfo(fmt.Sprintf("CSV line %d - %s needs to be updated from %s to %s", i+1, h, strings.Join(pceRuleResolveAs[z], ";"), csvResolveAs), false)
					update = true
				} else {
					// Here, both lengths are one so we can just compare values
					if pceRuleResolveAs[z][0] != csvResolveAs {
						utils.LogInfo(fmt.Sprintf("CSV line %d - %s needs to be updated from %s to %s", i+1, h, pceRuleResolveAs[z][0], csvResolveAs), false)
						update = true
					}
				}
			}
			// Populate target
			*targets[z] = csvResolveAsSlc
		}

		// Create the rule
		csvRule := illumioapi.Rule{UnscopedConsumers: &unscopedConsumers, Consumers: consumers, ConsumingSecurityPrincipals: consumingSecPrincipals, Providers: providers, IngressServices: &ingressSvc, Enabled: &enabled, MachineAuth: &machineAuth, SecConnect: &secConnect, ResolveLabelsAs: &illumioapi.ResolveLabelsAs{Consumers: consResolveAs, Providers: provResolveAs}}

		// Add to our array
		// Option 1 - No rule HREF provided, so it's a new rule
		if rowRuleHref == "" {
			newRules = append(newRules, toAdd{ruleSetHref: rs.Href, rule: csvRule, csvLine: i + 1})
			utils.LogInfo(fmt.Sprintf("CSV line %d - create new rule for %s ruleset", i+1, l[input.Headers[ruleexport.HeaderRulesetName]]), false)
		} else {
			// Option 2 - No rule href and update set, add to updated rules
			if update {
				csvRule.Href = rowRuleHref
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
		fmt.Printf("\r\n[PROMPT] - workloader identified %d rules to create and %d rules to update in %s (%s). Do you want to run the import (yes/no)? ", len(newRules), len(updatedRules), input.PCE.FriendlyName, viper.Get(input.PCE.FriendlyName+".fqdn").(string))
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
	if input.Provision {
		a, err := input.PCE.ProvisionHref(p, input.ProvisionComment)
		utils.LogAPIResp("ProvisionHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning complete - status code %d", a.StatusCode), true)
	}

	// Log end
	utils.LogEndCommand("rule-import")
}
