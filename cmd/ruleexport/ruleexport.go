package ruleexport

// Pending items:
// - Handle resolve as VS for inheriting the services.
// - Handle workload and/or VS rules directly.
// - Not fully load the PCE to save API calls for all workloads specifically.

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var err error

//Input is the input format for the rule-export command
type Input struct {
	PCE                                                                                            illumioapi.PCE
	Debug, Edge, ExpandServices, TrafficCount, SkipWkldDetailCheck                                 bool
	Role, App, Env, Loc, OutputFileName, ExplorerStart, ExplorerEnd, ExclServiceCSV, PolicyVersion string
	ExplorerMax                                                                                    int
	TemplateFormat                                                                                 bool
	RulesetHrefs                                                                                   []string
}

var input Input

// Init handles flags
func init() {

	RuleExportCmd.Flags().StringVar(&input.PolicyVersion, "policy-version", "draft", "Policy version. Must be active or draft.")
	RuleExportCmd.Flags().BoolVar(&input.ExpandServices, "expand-svcs", false, "Expand service objects to show ports/protocols (not compatible in rule-import format).")
	RuleExportCmd.Flags().StringVarP(&input.App, "app", "a", "", "Only include rules with app label (directly or via a label group) in the rule or scope.")
	RuleExportCmd.Flags().StringVarP(&input.Env, "env", "e", "", "Only include rules with env label (directly or via a label group) in the rule or scope.")
	RuleExportCmd.Flags().StringVarP(&input.Loc, "loc", "l", "", "Only include rules with loc label (directly or via a label group) in the rule or scope.")
	RuleExportCmd.Flags().BoolVar(&input.TrafficCount, "traffic-count", false, "Include the traffic summaries for flows that meet the rule criteria. An explorer query is executed per rule, which will take some time.")
	RuleExportCmd.Flags().IntVar(&input.ExplorerMax, "explorer-max-results", 10000, "Maximum results on an explorer query. Only applicable if used with traffic-count flag.")
	RuleExportCmd.Flags().StringVar(&input.ExplorerStart, "explorer-start", time.Date(time.Now().Year()-5, time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02"), "Start date in the format of yyyy-mm-dd.")
	RuleExportCmd.Flags().StringVar(&input.ExplorerEnd, "explorer-end", time.Now().Add(time.Hour*24).Format("2006-01-02"), "End date in the format of yyyy-mm-dd.")
	RuleExportCmd.Flags().StringVar(&input.ExclServiceCSV, "explorer-excl-svc-file", "", "File location of csv with port/protocols to exclude. Port number in column 1 and IANA numeric protocol in Col 2. Headers optional.")
	RuleExportCmd.Flags().BoolVarP(&input.SkipWkldDetailCheck, "skip-wkld-detail-check", "s", false, "Do not check for enforced workloads with low detail or no logging, which can skew traffic results since allowed (low detail) or all (no detail) flows are not reported. This can save time by not checking each workload enforcement state.")
	RuleExportCmd.Flags().StringVar(&input.OutputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	RuleExportCmd.Flags().BoolVar(&input.Edge, "edge", false, "Edge rule format")
	RuleExportCmd.Flags().MarkHidden("edge")
	RuleExportCmd.Flags().SortFlags = false

}

// RuleExportCmd runs the workload identifier
var RuleExportCmd = &cobra.Command{
	Use:   "rule-export",
	Short: "Create a CSV export of all rules in the input.PCE.",
	Long: `
Create a CSV export of all rules in the input.PCE. The app, env, and location flags (one label per key) will filter the results.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Validate the policy version
		input.PolicyVersion = strings.ToLower(input.PolicyVersion)
		if input.PolicyVersion != "active" && input.PolicyVersion != "draft" {
			utils.LogError("policy-version must be active or draft.")
		}

		// Get the PCE
		input.PCE, err = utils.GetTargetPCE(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		ExportRules(input)
	},
}

// ExportRules exports rules from the PCE
func ExportRules(input Input) {

	// Log command execution
	if input.Edge {
		utils.LogStartCommand("edge-rule-export")
	} else {
		utils.LogStartCommand("rule-export")
	}

	// GetAllRulesets
	utils.LogInfo("getting all rulesets...", true)
	allPCERulesets, a, err := input.PCE.GetAllRuleSets(input.PolicyVersion)
	utils.LogAPIResp("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Filter down the rulesets if we are given a slice
	allRuleSets := []illumioapi.RuleSet{}
	if len(input.RulesetHrefs) == 0 {
		allRuleSets = allPCERulesets
	} else {
		// Create a map
		targetRuleSets := make(map[string]bool)
		for _, h := range input.RulesetHrefs {
			targetRuleSets[h] = true
		}
		for _, rs := range allPCERulesets {
			if targetRuleSets[rs.Href] {
				allRuleSets = append(allRuleSets, rs)
			}
		}
	}

	// Run through rulesets to see what we need
	var needWklds, needLabelGroups, needVirtualServices, needVirtualServers, needUserGroups bool
	// Needed objects is for logging. Only add to this slice first time (when value is false and switches to true)
	neededObjects := map[string]bool{"labels": true, "ip_lists": true, "services": true}
	for _, rs := range allRuleSets {
		for _, scopes := range rs.Scopes {
			for _, scopeEntity := range scopes {
				if scopeEntity.LabelGroup != nil {
					neededObjects["label_group"] = true
					needLabelGroups = true
				}
			}
		}
		for _, r := range rs.Rules {
			for _, c := range r.Consumers {
				if c.Workload != nil {
					neededObjects["workloads"] = true
					needWklds = true
				}
				if c.VirtualService != nil {
					neededObjects["virtual_services"] = true
					needVirtualServices = true
				}
				if c.LabelGroup != nil {
					neededObjects["label_groups"] = true
					needLabelGroups = true
				}
			}
			for _, p := range r.Providers {
				if p.Workload != nil {
					neededObjects["workloads"] = true
					needWklds = true
				}
				if p.VirtualService != nil {
					neededObjects["virtual_services"] = true
					needVirtualServices = true
				}
				if p.VirtualServer != nil {
					neededObjects["virtual_servers"] = true
					needVirtualServers = true
				}
				if p.LabelGroup != nil {
					neededObjects["label_groups"] = true
					needLabelGroups = true
				}
			}
			if r.ConsumingSecurityPrincipals != nil && len(r.ConsumingSecurityPrincipals) > 0 {
				neededObjects["consuming_security_principals"] = true
				needUserGroups = true
			}
		}
	}

	// Check if we need workloads for checking detail
	if input.TrafficCount && !input.SkipWkldDetailCheck {
		neededObjects["workloads"] = true
		needWklds = true
	}

	// Load the PCE with the relevant obects (save unnecessary expensive potentially large GETs)
	neededObjectsSlice := []string{}
	for n := range neededObjects {
		neededObjectsSlice = append(neededObjectsSlice, n)
	}
	utils.LogInfo(fmt.Sprintf("getting %s ...", strings.Join(neededObjectsSlice, ", ")), true)
	if err = input.PCE.Load(illumioapi.LoadInput{
		Labels:                      true,
		IPLists:                     true,
		Services:                    true,
		ConsumingSecurityPrincipals: needUserGroups,
		LabelGroups:                 needLabelGroups,
		Workloads:                   needWklds,
		VirtualServices:             needVirtualServices,
		VirtualServers:              needVirtualServers,
		ProvisionStatus:             input.PolicyVersion,
	}); err != nil {
		utils.LogError(err.Error())
	}

	if input.TrafficCount && !input.SkipWkldDetailCheck {
		lowCount := 0
		noCount := 0
		for _, wkld := range input.PCE.Workloads {
			if wkld.GetMode() == "enforced-low" {
				lowCount++
			}
			if wkld.GetMode() == "enforced-no" {
				noCount++
			}
		}
		if lowCount+noCount > 0 {
			var prompt string
			fmt.Printf("\r\n%s [PROMPT] - there are workloads with low (%d) or no (%d) logging. Low-detail logging does not report allowed flows and no detail does not report any flows. The traffic-count comes from reported flows. If this command is being used to find stale rules (i.e., rules not being hit) low or no logging can cause false positives. Do you want to continue? (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), lowCount, noCount)
			fmt.Scanln(&prompt)
			if strings.ToLower(prompt) != "yes" {
				utils.LogInfo("prompt denied to continue after workload logging warning.", true)
				utils.LogEndCommand("rule-export")
				return
			}
		}

	}

	// Build the filer list
	filter := make(map[string]int)
	keys := []string{"app", "env", "loc"}
	values := []string{input.App, input.Env, input.Loc}
	for i, k := range keys {
		if values[i] == "" {
			continue
		}
		if val, ok := input.PCE.Labels[k+values[i]]; !ok {
			utils.LogError(fmt.Sprintf("%s does not exist as a %s label", values[i], k))

		} else {
			filter[val.Href] = 0
		}
	}

	// Log the filter list
	lf := []string{}
	for f := range filter {
		lf = append(lf, fmt.Sprintf("%s (%s)", input.PCE.Labels[f].Value, input.PCE.Labels[f].Key))
	}
	if len(lf) > 0 {
		utils.LogInfo(fmt.Sprintf("filter list: %s", strings.Join(lf, ", ")), false)
	} else {
		utils.LogInfo("no filters", false)
	}

	// Start the data slice with headers
	csvData := [][]string{}
	if input.TrafficCount {
		csvData = append(csvData, append(getCSVHeaders(input.TemplateFormat), []string{"flows", "flows_by_port"}...))
	} else {
		csvData = append(csvData, getCSVHeaders(input.TemplateFormat))
	}

	edgeCSVData := [][]string{[]string{"group", "consumer_iplist", "consumer_group", "consumer_user_group", "service", "provider_group", "provider_iplist", "rule_enabled", "machine_auth", "rule_href", "ruleset_href"}}

	// Iterate each ruleset
	var i int
	var rs illumioapi.RuleSet

	for i, rs = range allRuleSets {
		// Reset the matchedRules and filters
		matchedRules := 0
		scopeFilter := make(map[string]int)
		for s := range filter {
			scopeFilter[s] = 0
		}

		// Log ruleset processing
		utils.LogInfo(fmt.Sprintf("processing ruleset %s with %d rules", rs.Name, len(rs.Rules)), false)

		// Check for custom iptables rules
		customIPTables := false
		if len(rs.IPTablesRules) != 0 {
			customIPTables = true
		}
		utils.LogInfo(fmt.Sprintf("custom iptables rules: %t", customIPTables), false)

		// Get the scopes
		scopesSlice := []string{}

		// Iterate through each scope
		for _, scope := range rs.Scopes {
			scopeMap := make(map[string]string)
			for _, scopeMember := range scope {
				if scopeMember.Label != nil {
					scopeMap[input.PCE.Labels[scopeMember.Label.Href].Key] = input.PCE.Labels[scopeMember.Label.Href].Value
					// Check if we hit a filter
					if val, ok := scopeFilter[scopeMember.Label.Href]; ok {
						scopeFilter[scopeMember.Label.Href] = val + 1
						//scopeFilterCheck = true
						utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is in the scope", input.PCE.Labels[scopeMember.Label.Href].Value, input.PCE.Labels[scopeMember.Label.Href].Key), false)
					}
				}
				if scopeMember.LabelGroup != nil {
					scopeMap[input.PCE.LabelGroups[scopeMember.LabelGroup.Href].Key] = fmt.Sprintf("%s (label_group)", input.PCE.LabelGroups[scopeMember.LabelGroup.Href].Name)
					// Expand the label group
					labels := input.PCE.ExpandLabelGroup(scopeMember.LabelGroup.Href)
					// Check if we hit a filter
					for _, l := range labels {
						if val, ok := scopeFilter[l]; ok {
							scopeFilter[l] = val + 1
							// scopeFilterCheck = true
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is in %s (label group) which is in the scope", input.PCE.Labels[l].Value, input.PCE.Labels[l].Key, input.PCE.LabelGroups[scopeMember.LabelGroup.Href].Name), false)
						}
					}
				}
			}
			var scopeString string
			if scopeMap["role"] == "" {
				scopeString = "ALL | "
				if input.Role != "" {
					if val, ok := scopeFilter[input.PCE.Labels["app"+input.App].Href]; ok {
						scopeFilter[input.PCE.Labels["role"+input.Role].Href] = val + 1
					}
					utils.LogInfo("filter match - scope includes all roles.", false)
				}
			} else {
				scopeString = scopeMap["role"] + " | "
			}
			if scopeMap["app"] == "" {
				scopeString = "ALL | "
				if input.App != "" {
					if val, ok := scopeFilter[input.PCE.Labels["app"+input.App].Href]; ok {
						scopeFilter[input.PCE.Labels["app"+input.App].Href] = val + 1
					}
					utils.LogInfo("filter match - scope includes all applications.", false)
				}
			} else {
				scopeString = scopeMap["app"] + " | "
			}
			if scopeMap["env"] == "" {
				scopeString = scopeString + "ALL | "
				if input.Env != "" {
					if val, ok := scopeFilter[input.PCE.Labels["env"+input.Env].Href]; ok {
						scopeFilter[input.PCE.Labels["env"+input.Env].Href] = val + 1
					}
					utils.LogInfo("filter match - scope includes all environments.", false)
				}
			} else {
				scopeString = scopeString + scopeMap["env"] + " | "
			}
			if scopeMap["loc"] == "" {
				scopeString = scopeString + "ALL"
				if input.Loc != "" {
					if val, ok := scopeFilter[input.PCE.Labels["loc"+input.Loc].Href]; ok {
						scopeFilter[input.PCE.Labels["loc"+input.Loc].Href] = val + 1
					}
					utils.LogInfo("filter match - scope includes all locations.", false)
				}
			} else {
				scopeString = scopeString + scopeMap["loc"]
			}
			scopesSlice = append(scopesSlice, scopeString)
		}

		// Process each rule
		for _, r := range rs.Rules {
			csvEntryMap := make(map[string]string)
			// Populate the map with basic info
			csvEntryMap[HeaderRuleSetScope] = strings.Join(scopesSlice, ";")
			csvEntryMap[HeaderRulesetHref] = rs.Href
			csvEntryMap[HeaderRulesetEnabled] = strconv.FormatBool(*rs.Enabled)
			csvEntryMap[HeaderRulesetDescription] = rs.Description
			csvEntryMap[HeaderRulesetName] = rs.Name
			csvEntryMap[HeaderRuleHref] = r.Href
			csvEntryMap[HeaderRuleDescription] = r.Description
			csvEntryMap[HeaderRuleEnabled] = strconv.FormatBool(*r.Enabled)
			csvEntryMap[HeaderUnscopedConsumers] = strconv.FormatBool(*r.UnscopedConsumers)
			csvEntryMap[HeaderStateless] = strconv.FormatBool(*r.Stateless)
			csvEntryMap[HeaderMachineAuthEnabled] = strconv.FormatBool(*r.MachineAuth)
			csvEntryMap[HeaderSecureConnectEnabled] = strconv.FormatBool(*r.SecConnect)
			if r.UpdateType == "update" {
				csvEntryMap[HeaderUpdateType] = "Modification Pending"
			} else if r.UpdateType == "delete" {
				csvEntryMap[HeaderUpdateType] = "Deletion Pending"
			} else if r.UpdateType == "create" {
				csvEntryMap[HeaderUpdateType] = "Addition Pending"
			} else {
				csvEntryMap[HeaderUpdateType] = r.UpdateType
			}

			// Reset the filter
			ruleFilter := make(map[string]int)
			for s := range filter {
				ruleFilter[s] = 0
			}
			// Consumers
			for _, c := range r.Consumers {
				if c.Actors == "ams" {
					csvEntryMap[HeaderConsumerAllWorkloads] = "true"
					if *r.UnscopedConsumers && len(filter) > 0 {
						for rf := range ruleFilter {
							ruleFilter[rf] = ruleFilter[rf] + 1
						}
						utils.LogInfo(fmt.Sprintf("filter match - global consumers include all workloads - rule %s", r.Href), false)
					}
					continue
				}

				// IP List
				if c.IPList != nil {
					// If we are exporting for templates, all IP Lists should be set to Any
					if input.TemplateFormat {
						c.IPList.Name = "Any (0.0.0.0/0 and ::/0)"
					}
					if val, ok := csvEntryMap[HeaderConsumerIplists]; ok {
						csvEntryMap[HeaderConsumerIplists] = fmt.Sprintf("%s;%s", val, input.PCE.IPLists[c.IPList.Href].Name)
					} else {
						csvEntryMap[HeaderConsumerIplists] = input.PCE.IPLists[c.IPList.Href].Name
					}
				}
				// Labels
				if c.Label != nil {
					keys := []string{"role", "app", "env", "loc"}
					target := []string{HeaderConsumerRole, HeaderConsumerApp, HeaderConsumerEnv, HeaderConsumerLoc}
					for i, k := range keys {
						if input.PCE.Labels[c.Label.Href].Key != k {
							continue
						}
						if val, ok := csvEntryMap[target[i]]; ok {
							csvEntryMap[target[i]] = fmt.Sprintf("%s;%s", val, input.PCE.Labels[c.Label.Href].Value)
						} else {
							csvEntryMap[target[i]] = input.PCE.Labels[c.Label.Href].Value
						}
					}
					if val, ok := ruleFilter[c.Label.Href]; ok {
						ruleFilter[c.Label.Href] = val + 1
						//ruleFilterCheck = true
						utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a consumer label - rule %s", input.PCE.Labels[c.Label.Href].Value, input.PCE.Labels[c.Label.Href].Key, r.Href), false)
					}
				}
				// Label Groups
				if c.LabelGroup != nil {
					if val, ok := csvEntryMap[HeaderConsumerLabelGroup]; ok {
						csvEntryMap[HeaderConsumerLabelGroup] = fmt.Sprintf("%s;%s", val, input.PCE.LabelGroups[c.LabelGroup.Href].Name)
					} else {
						csvEntryMap[HeaderConsumerLabelGroup] = input.PCE.LabelGroups[c.LabelGroup.Href].Name
					}
					// Expand the label group and check each
					labels := input.PCE.ExpandLabelGroup(c.LabelGroup.Href)
					for _, l := range labels {
						if val, ok := ruleFilter[l]; ok {
							ruleFilter[l] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is in %s (label group) which is a consumer - rule %s", input.PCE.Labels[l].Value, input.PCE.Labels[l].Key, input.PCE.LabelGroups[c.LabelGroup.Href].Name, r.Href), false)
						}
					}
				}
				// Virtual Services
				if c.VirtualService != nil {
					if val, ok := csvEntryMap[HeaderConsumerVirtualServices]; ok {
						csvEntryMap[HeaderConsumerVirtualServices] = fmt.Sprintf("%s;%s", val, input.PCE.VirtualServices[c.VirtualService.Href].Name)
					} else {
						csvEntryMap[HeaderConsumerVirtualServices] = input.PCE.VirtualServices[c.VirtualService.Href].Name
					}
					// Check the labels
					for _, l := range input.PCE.VirtualServices[c.VirtualService.Href].Labels {
						if val, ok := ruleFilter[l.Href]; ok {
							ruleFilter[l.Href] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a consumer label on %s virtual service - rule %s", input.PCE.Labels[l.Href].Value, input.PCE.Labels[l.Href].Key, input.PCE.VirtualServices[c.VirtualService.Href].Name, r.Href), false)
						}
					}
				}
				if c.Workload != nil {
					if val, ok := csvEntryMap[HeaderConsumerWorkloads]; ok {
						csvEntryMap[HeaderConsumerWorkloads] = fmt.Sprintf("%s;%s", val, input.PCE.Workloads[c.Workload.Href].Hostname)
					} else {
						csvEntryMap[HeaderConsumerWorkloads] = input.PCE.Workloads[c.Workload.Href].Hostname
					}
					// Check the labels
					for _, l := range *input.PCE.Workloads[c.Workload.Href].Labels {
						if val, ok := ruleFilter[l.Href]; ok {
							ruleFilter[l.Href] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a consumer label on %s workload - rule %s", input.PCE.Labels[l.Href].Value, input.PCE.Labels[l.Href].Key, input.PCE.Workloads[c.Workload.Href].Name, r.Href), false)
						}
					}
				}
			}

			// Consuming Security Principals
			consumingSecPrincipals := []string{}
			for _, csp := range r.ConsumingSecurityPrincipals {
				consumingSecPrincipals = append(consumingSecPrincipals, input.PCE.ConsumingSecurityPrincipals[csp.Href].Name)
			}
			csvEntryMap[HeaderConsumerUserGroups] = strings.Join(consumingSecPrincipals, ";")

			// Providers
			for _, p := range r.Providers {

				if p.Actors == "ams" {
					csvEntryMap[HeaderProviderAllWorkloads] = "true"
					continue
				}
				// IP List
				if p.IPList != nil {
					// If we are exporting for templates, all IP Lists should be set to Any
					if input.TemplateFormat {
						p.IPList.Name = "Any (0.0.0.0/0 and ::/0)"
					}
					if val, ok := csvEntryMap[HeaderProviderIplists]; ok {
						csvEntryMap[HeaderProviderIplists] = fmt.Sprintf("%s;%s", val, input.PCE.IPLists[p.IPList.Href].Name)
					} else {
						csvEntryMap[HeaderProviderIplists] = input.PCE.IPLists[p.IPList.Href].Name
					}
				}
				// Labels
				if p.Label != nil {
					keys := []string{"role", "app", "env", "loc"}
					target := []string{HeaderProviderRole, HeaderProviderApp, HeaderProviderEnv, HeaderProviderLoc}
					for i, k := range keys {
						if input.PCE.Labels[p.Label.Href].Key != k {
							continue
						}
						if val, ok := csvEntryMap[target[i]]; ok {
							csvEntryMap[target[i]] = fmt.Sprintf("%s;%s", val, input.PCE.Labels[p.Label.Href].Value)
						} else {
							csvEntryMap[target[i]] = input.PCE.Labels[p.Label.Href].Value
						}
					}
					if val, ok := ruleFilter[p.Label.Href]; ok {
						ruleFilter[p.Label.Href] = val + 1
						//ruleFilterCheck = true
						utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a provider label - rule %s", input.PCE.Labels[p.Label.Href].Value, input.PCE.Labels[p.Label.Href].Key, r.Href), false)
					}
				}
				// Label Groups
				if p.LabelGroup != nil {
					if val, ok := csvEntryMap[HeaderProviderLabelGroups]; ok {
						csvEntryMap[HeaderProviderLabelGroups] = fmt.Sprintf("%s;%s", val, input.PCE.Labels[p.LabelGroup.Href].Value)
					} else {
						csvEntryMap[HeaderProviderLabelGroups] = input.PCE.Labels[p.LabelGroup.Href].Value
					}
					// Expand the label group and check each
					labels := input.PCE.ExpandLabelGroup(p.LabelGroup.Href)
					for _, l := range labels {
						if val, ok := ruleFilter[l]; ok {
							ruleFilter[l] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is in %s (label group) which is a provider - rule %s", input.PCE.Labels[l].Value, input.PCE.Labels[l].Key, input.PCE.LabelGroups[p.LabelGroup.Href].Name, r.Href), false)
						}
					}
				}
				// Virtual Services
				if p.VirtualService != nil {
					if val, ok := csvEntryMap[HeaderProviderVirtualServices]; ok {
						csvEntryMap[HeaderProviderVirtualServices] = fmt.Sprintf("%s;%s", val, input.PCE.VirtualServices[p.VirtualService.Href].Name)
					} else {
						csvEntryMap[HeaderProviderVirtualServices] = input.PCE.VirtualServices[p.VirtualService.Href].Name
					}
					for _, l := range input.PCE.VirtualServices[p.VirtualService.Href].Labels {
						if val, ok := ruleFilter[l.Href]; ok {
							ruleFilter[l.Href] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a provider label on %s virtual service - rule %s", input.PCE.Labels[l.Href].Value, input.PCE.Labels[l.Href].Key, input.PCE.VirtualServices[p.VirtualService.Href].Name, r.Href), false)
						}
					}
				}
				// Workloads
				if p.Workload != nil {
					if val, ok := csvEntryMap[HeaderProviderWorkloads]; ok {
						csvEntryMap[HeaderProviderWorkloads] = fmt.Sprintf("%s;%s", val, input.PCE.Workloads[p.Workload.Href].Hostname)
					} else {
						csvEntryMap[HeaderProviderWorkloads] = input.PCE.Workloads[p.Workload.Href].Hostname
					}
					// Check the labels
					for _, l := range *input.PCE.Workloads[p.Workload.Href].Labels {
						if val, ok := ruleFilter[l.Href]; ok {
							ruleFilter[l.Href] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a provider label on %s workload - rule %s", input.PCE.Labels[l.Href].Value, input.PCE.Labels[l.Href].Key, input.PCE.Workloads[p.Workload.Href].Name, r.Href), false)
						}
					}
				}
				// Virtual Servers
				if p.VirtualServer != nil {
					if val, ok := csvEntryMap[HeaderProviderVirtualServers]; ok {
						csvEntryMap[HeaderProviderVirtualServers] = fmt.Sprintf("%s;%s", val, input.PCE.VirtualServers[p.VirtualServer.Href].Name)
					} else {
						csvEntryMap[HeaderProviderVirtualServers] = input.PCE.VirtualServers[p.VirtualServer.Href].Name
					}
					for _, l := range input.PCE.VirtualServers[p.VirtualServer.Href].Labels {
						if val, ok := ruleFilter[l.Href]; ok {
							ruleFilter[l.Href] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a provider label on %s workload - rule %s", input.PCE.Labels[l.Href].Value, input.PCE.Labels[l.Href].Key, input.PCE.VirtualServers[p.VirtualServer.Href].Name, r.Href), false)
						}
					}
				}
			}

			// Services
			services := []string{}
			for _, s := range *r.IngressServices {
				if s.Href != nil && input.PCE.Services[*s.Href].WindowsServices != nil {
					a := input.PCE.Services[*s.Href]
					b, _ := a.ParseService()
					if !input.ExpandServices {
						services = append(services, input.PCE.Services[*s.Href].Name)
					} else {
						services = append(services, fmt.Sprintf("%s (%s)", input.PCE.Services[*s.Href].Name, strings.Join(b, ";")))
					}
				}
				if s.Href != nil && input.PCE.Services[*s.Href].ServicePorts != nil {
					a := input.PCE.Services[*s.Href]
					_, b := a.ParseService()
					if input.PCE.Services[*s.Href].Name == "All Services" {
						services = append(services, "All Services")
					} else {
						if !input.ExpandServices {
							services = append(services, input.PCE.Services[*s.Href].Name)
						} else {
							services = append(services, fmt.Sprintf("%s (%s)", input.PCE.Services[*s.Href].Name, strings.Join(b, ";")))
						}
					}
				}
				if s.Href == nil {
					services = append(services, fmt.Sprintf("%d %s", *s.Port, illumioapi.ProtocolList()[*s.Protocol]))
				}
			}
			csvEntryMap[HeaderServices] = strings.Join(services, ";")

			// Resolve As
			csvEntryMap[HeaderConsumerResolveLabelsAs] = strings.Join(r.ResolveLabelsAs.Consumers, ";")
			csvEntryMap[HeaderProviderResolveLabelsAs] = strings.Join(r.ResolveLabelsAs.Providers, ";")

			// Append to output if there are no filters or if we pass the filter checks
			skip := false
			for f := range filter {
				if scopeFilter[f]+ruleFilter[f] == 0 {
					skip = true
				}
			}
			// Adjust some blanks
			if csvEntryMap[HeaderConsumerAllWorkloads] == "" {
				csvEntryMap[HeaderConsumerAllWorkloads] = "false"
			}
			if csvEntryMap[HeaderProviderAllWorkloads] == "" {
				csvEntryMap[HeaderProviderAllWorkloads] = "false"
			}

			if len(filter) == 0 || !skip {
				if input.TrafficCount {
					csvData = append(csvData, append(createEntrySlice(csvEntryMap, input.TemplateFormat), trafficCounter(input, rs, *r)...))
				} else {
					csvData = append(csvData, createEntrySlice(csvEntryMap, input.TemplateFormat))
				}

				matchedRules++
			}

		}
		utils.LogInfo(fmt.Sprintf("%d rules exported.", matchedRules), false)
	}

	// Output the CSV Data
	if len(csvData) > 1 {
		if input.Edge {
			csvData = edgeCSVData
		}
		if input.OutputFileName == "" {
			input.OutputFileName = fmt.Sprintf("workloader-rule-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, input.OutputFileName)
		utils.LogInfo(fmt.Sprintf("%d rules from %d rulesets exported", len(csvData)-1, i+1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no rulesets in input.PCE.", true)
	}

	if input.Edge {
		utils.LogEndCommand("edge-rule-export")
	} else {
		utils.LogEndCommand("rule-export")
	}

}
