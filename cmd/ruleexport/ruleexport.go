package ruleexport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug, useActive, edge, expandServices bool
var outFormat, role, app, env, loc, outputFileName string

// Init handles flags
func init() {

	RuleExportCmd.Flags().BoolVar(&useActive, "active", false, "Use active policy versus draft. Draft is default.")
	RuleExportCmd.Flags().BoolVar(&expandServices, "expand-svcs", false, "Expand service objects to show ports/protocols (not allowed in rule-import format).")
	RuleExportCmd.Flags().StringVarP(&app, "app", "a", "", "Only include rules with app label (directly or via a label group) in the rule or scope.")
	RuleExportCmd.Flags().StringVarP(&env, "env", "e", "", "Only include rules with env label (directly or via a label group) in the rule or scope.")
	RuleExportCmd.Flags().StringVarP(&loc, "loc", "l", "", "Only include rules with loc label (directly or via a label group) in the rule or scope.")
	RuleExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	RuleExportCmd.Flags().BoolVar(&edge, "edge", false, "Edge rule format")
	RuleExportCmd.Flags().MarkHidden("edge")
	RuleExportCmd.Flags().SortFlags = false

}

// RuleExportCmd runs the workload identifier
var RuleExportCmd = &cobra.Command{
	Use:   "rule-export",
	Short: "Create a CSV export of all rules in the PCE.",
	Long: `
Create a CSV export of all rules in the PCE. The app, env, and location flags (one label per key) will filter the results.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the viper values
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		ExportRules(pce, useActive, app, env, loc, edge, expandServices, outputFileName, debug)
	},
}

// ExportRules exports rules from the PCE
func ExportRules(pce illumioapi.PCE, useActive bool, app, env, loc string, edge, expandSVCs bool, outputFileName string, debug bool) {

	// Log command execution
	if edge {
		utils.LogStartCommand("edge-rule-export")
	} else {
		utils.LogStartCommand("rule-export")
	}

	// Check active/draft
	provisionStatus := "draft"
	if useActive {
		provisionStatus = "active"
	}
	utils.LogInfo(fmt.Sprintf("provision status: %s", provisionStatus), false)

	// Load the pce
	if err := pce.Load(provisionStatus); err != nil {
		utils.LogError(err.Error())
	}
	utils.LogInfo("successfulyl loaded PCE with all policy objects", false)

	// Build the filer list
	filter := make(map[string]int)
	keys := []string{"app", "env", "loc"}
	values := []string{app, env, loc}
	for i, k := range keys {
		if values[i] == "" {
			continue
		}
		if val, ok := pce.LabelMapKV[k+values[i]]; !ok {
			utils.LogError(fmt.Sprintf("%s does not exist as a %s label", values[i], k))

		} else {
			filter[val.Href] = 0
		}
	}

	// Log the filter list
	lf := []string{}
	for f := range filter {
		lf = append(lf, fmt.Sprintf("%s (%s)", pce.LabelMapH[f].Value, pce.LabelMapH[f].Key))
	}
	if len(lf) > 0 {
		utils.LogInfo(fmt.Sprintf("filter list: %s", strings.Join(lf, ", ")), false)
	} else {
		utils.LogInfo("no filters", false)
	}

	// Check if we need to get all workloads
	wkldHrefMap := make(map[string]illumioapi.Workload)
	var a illumioapi.APIResponse
	if len(filter) > 0 {
		wkldHrefMap, a, err = pce.GetWkldHrefMap()
		utils.LogAPIResp("GetWkldHrefMap", a)
		if err != nil {
			utils.LogError(err.Error())
		}

	}

	// Start the data slice with headers
	csvData := [][]string{getCSVHeaders()}

	edgeCSVData := [][]string{[]string{"group", "consumer_iplist", "consumer_group", "consumer_user_group", "service", "provider_group", "provider_iplist", "rule_enabled", "machine_auth", "rule_href", "ruleset_href"}}

	// GetAllRulesets
	allRuleSets, a, err := pce.GetAllRuleSets(provisionStatus)
	utils.LogAPIResp("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}

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
					scopeMap[pce.LabelMapH[scopeMember.Label.Href].Key] = pce.LabelMapH[scopeMember.Label.Href].Value
					// Check if we hit a filter
					if val, ok := scopeFilter[scopeMember.Label.Href]; ok {
						scopeFilter[scopeMember.Label.Href] = val + 1
						//scopeFilterCheck = true
						utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is in the scope", pce.LabelMapH[scopeMember.Label.Href].Value, pce.LabelMapH[scopeMember.Label.Href].Key), false)
					}
				}
				if scopeMember.LabelGroup != nil {
					scopeMap[pce.LabelGroupMapH[scopeMember.LabelGroup.Href].Key] = fmt.Sprintf("%s (label_group)", pce.LabelGroupMapH[scopeMember.LabelGroup.Href].Name)
					// Expand the label group
					labels := pce.ExpandLabelGroup(scopeMember.LabelGroup.Href)
					// Check if we hit a filter
					for _, l := range labels {
						if val, ok := scopeFilter[l]; ok {
							scopeFilter[l] = val + 1
							// scopeFilterCheck = true
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is in %s (label group) which is in the scope", pce.LabelMapH[l].Value, pce.LabelMapH[l].Key, pce.LabelGroupMapH[scopeMember.LabelGroup.Href].Name), false)
						}
					}
				}
			}
			var scopeString string
			if scopeMap["role"] == "" {
				scopeString = "ALL | "
				if role != "" {
					if val, ok := scopeFilter[pce.LabelMapKV["app"+app].Href]; ok {
						scopeFilter[pce.LabelMapKV["role"+role].Href] = val + 1
					}
					utils.LogInfo("filter match - scope includes all roles.", false)
				}
			} else {
				scopeString = scopeMap["role"] + " | "
			}
			if scopeMap["app"] == "" {
				scopeString = "ALL | "
				if app != "" {
					if val, ok := scopeFilter[pce.LabelMapKV["app"+app].Href]; ok {
						scopeFilter[pce.LabelMapKV["app"+app].Href] = val + 1
					}
					utils.LogInfo("filter match - scope includes all applications.", false)
				}
			} else {
				scopeString = scopeMap["app"] + " | "
			}
			if scopeMap["env"] == "" {
				scopeString = scopeString + "ALL | "
				if env != "" {
					if val, ok := scopeFilter[pce.LabelMapKV["env"+env].Href]; ok {
						scopeFilter[pce.LabelMapKV["env"+env].Href] = val + 1
					}
					utils.LogInfo("filter match - scope includes all environments.", false)
				}
			} else {
				scopeString = scopeString + scopeMap["env"] + " | "
			}
			if scopeMap["loc"] == "" {
				scopeString = scopeString + "ALL"
				if loc != "" {
					if val, ok := scopeFilter[pce.LabelMapKV["loc"+loc].Href]; ok {
						scopeFilter[pce.LabelMapKV["loc"+loc].Href] = val + 1
					}
					utils.LogInfo("filter match - scope includes all locations.", false)
				}
			} else {
				scopeString = scopeString + scopeMap["loc"]
			}
			scopesSlice = append(scopesSlice, scopeString)
		}

		// If there are no rules here, add the csv entry
		if len(rs.Rules) == 0 {
			csvData = append(csvData, []string{rs.Name, strconv.FormatBool(*rs.Enabled), rs.Description, strings.Join(scopesSlice, ";"), "no rules", "no rules", "no rules", "no rules", "no rules", "no rules", "no rules", "no rules", "no rules", "no rules", "no rules", "no rules", rs.Href, "no rules"})
			edgeCSVData = append(edgeCSVData, []string{rs.Name, "no rules", "no rules", "no rules", "no rules", "no rules", "no rules", "no rules", rs.Href})

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
					if val, ok := csvEntryMap[HeaderConsumerIplists]; ok {
						csvEntryMap[HeaderConsumerIplists] = fmt.Sprintf("%s;%s", val, pce.IPListMapH[c.IPList.Href].Name)
					} else {
						csvEntryMap[HeaderConsumerIplists] = pce.IPListMapH[c.IPList.Href].Name
					}
				}
				// Labels
				if c.Label != nil {
					keys := []string{"role", "app", "env", "loc"}
					target := []string{HeaderConsumerRole, HeaderConsumerApp, HeaderConsumerEnv, HeaderConsumerLoc}
					for i, k := range keys {
						if pce.LabelMapH[c.Label.Href].Key != k {
							continue
						}
						if val, ok := csvEntryMap[target[i]]; ok {
							csvEntryMap[target[i]] = fmt.Sprintf("%s;%s", val, pce.LabelMapH[c.Label.Href].Value)
						} else {
							csvEntryMap[target[i]] = pce.LabelMapH[c.Label.Href].Value
						}
					}
					if val, ok := ruleFilter[c.Label.Href]; ok {
						ruleFilter[c.Label.Href] = val + 1
						//ruleFilterCheck = true
						utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a consumer label - rule %s", pce.LabelMapH[c.Label.Href].Value, pce.LabelMapH[c.Label.Href].Key, r.Href), false)
					}
				}
				// Label Groups
				if c.LabelGroup != nil {
					if val, ok := csvEntryMap[HeaderConsumerLabelGroup]; ok {
						csvEntryMap[HeaderConsumerLabelGroup] = fmt.Sprintf("%s;%s", val, pce.LabelGroupMapH[c.LabelGroup.Href].Name)
					} else {
						csvEntryMap[HeaderConsumerLabelGroup] = pce.LabelGroupMapH[c.LabelGroup.Href].Name
					}
					// Expand the label group and check each
					labels := pce.ExpandLabelGroup(c.LabelGroup.Href)
					for _, l := range labels {
						if val, ok := ruleFilter[l]; ok {
							ruleFilter[l] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is in %s (label group) which is a consumer - rule %s", pce.LabelMapH[l].Value, pce.LabelMapH[l].Key, pce.LabelGroupMapH[c.LabelGroup.Href].Name, r.Href), false)
						}
					}
				}
				// Virtual Services
				if c.VirtualService != nil {
					if val, ok := csvEntryMap[HeaderConsumerVirtualServices]; ok {
						csvEntryMap[HeaderConsumerVirtualServices] = fmt.Sprintf("%s;%s", val, pce.VirtualServiceMapH[c.VirtualService.Href].Name)
					} else {
						csvEntryMap[HeaderConsumerVirtualServices] = pce.VirtualServiceMapH[c.VirtualService.Href].Name
					}
					// Check the labels
					for _, l := range pce.VirtualServiceMapH[c.VirtualService.Href].Labels {
						if val, ok := ruleFilter[l.Href]; ok {
							ruleFilter[l.Href] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a consumer label on %s virtual service - rule %s", pce.LabelMapH[l.Href].Value, pce.LabelMapH[l.Href].Key, pce.VirtualServiceMapH[c.VirtualService.Href].Name, r.Href), false)
						}
					}
				}
				if c.Workload != nil {
					if val, ok := csvEntryMap[HeaderConsumerWorkloads]; ok {
						csvEntryMap[HeaderConsumerWorkloads] = fmt.Sprintf("%s;%s", val, pce.WorkloadMapH[c.Workload.Href].Hostname)
					} else {
						csvEntryMap[HeaderConsumerWorkloads] = pce.WorkloadMapH[c.Workload.Href].Hostname
					}
					// Check the labels
					for _, l := range wkldHrefMap[c.Workload.Href].Labels {
						if val, ok := ruleFilter[l.Href]; ok {
							ruleFilter[l.Href] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a consumer label on %s workload - rule %s", pce.LabelMapH[l.Href].Value, pce.LabelMapH[l.Href].Key, wkldHrefMap[c.Workload.Href].Name, r.Href), false)
						}
					}
				}
			}

			// Consuming Security Principals
			consumingSecPrincipals := []string{}
			for _, csp := range r.ConsumingSecurityPrincipals {
				consumingSecPrincipals = append(consumingSecPrincipals, csp.Name)
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
					if val, ok := csvEntryMap[HeaderProviderIplists]; ok {
						csvEntryMap[HeaderProviderIplists] = fmt.Sprintf("%s;%s", val, pce.IPListMapH[p.IPList.Href].Name)
					} else {
						csvEntryMap[HeaderProviderIplists] = pce.IPListMapH[p.IPList.Href].Name
					}
				}
				// Labels
				if p.Label != nil {
					keys := []string{"role", "app", "env", "loc"}
					target := []string{HeaderProviderRole, HeaderProviderApp, HeaderProviderEnv, HeaderProviderLoc}
					for i, k := range keys {
						if pce.LabelMapH[p.Label.Href].Key != k {
							continue
						}
						if val, ok := csvEntryMap[target[i]]; ok {
							csvEntryMap[target[i]] = fmt.Sprintf("%s;%s", val, pce.LabelMapH[p.Label.Href].Value)
						} else {
							csvEntryMap[target[i]] = pce.LabelMapH[p.Label.Href].Value
						}
					}
					if val, ok := ruleFilter[p.Label.Href]; ok {
						ruleFilter[p.Label.Href] = val + 1
						//ruleFilterCheck = true
						utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a provider label - rule %s", pce.LabelMapH[p.Label.Href].Value, pce.LabelMapH[p.Label.Href].Key, r.Href), false)
					}
				}
				// Label Groups
				if p.LabelGroup != nil {
					if val, ok := csvEntryMap[HeaderProviderLabelGroups]; ok {
						csvEntryMap[HeaderProviderLabelGroups] = fmt.Sprintf("%s;%s", val, pce.LabelMapH[p.LabelGroup.Href].Value)
					} else {
						csvEntryMap[HeaderProviderLabelGroups] = pce.LabelMapH[p.LabelGroup.Href].Value
					}
					// Expand the label group and check each
					labels := pce.ExpandLabelGroup(p.LabelGroup.Href)
					for _, l := range labels {
						if val, ok := ruleFilter[l]; ok {
							ruleFilter[l] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is in %s (label group) which is a provider - rule %s", pce.LabelMapH[l].Value, pce.LabelMapH[l].Key, pce.LabelGroupMapH[p.LabelGroup.Href].Name, r.Href), false)
						}
					}
				}
				// Virtual Services
				if p.VirtualService != nil {
					if val, ok := csvEntryMap[HeaderProviderVirtualServices]; ok {
						csvEntryMap[HeaderProviderVirtualServices] = fmt.Sprintf("%s;%s", val, pce.VirtualServiceMapH[p.VirtualService.Href].Name)
					} else {
						csvEntryMap[HeaderProviderVirtualServices] = pce.VirtualServiceMapH[p.VirtualService.Href].Name
					}
					for _, l := range pce.VirtualServiceMapH[p.VirtualService.Href].Labels {
						if val, ok := ruleFilter[l.Href]; ok {
							ruleFilter[l.Href] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a provider label on %s virtual service - rule %s", pce.LabelMapH[l.Href].Value, pce.LabelMapH[l.Href].Key, pce.VirtualServiceMapH[p.VirtualService.Href].Name, r.Href), false)
						}
					}
				}
				// Workloads
				if p.Workload != nil {
					if val, ok := csvEntryMap[HeaderProviderWorkloads]; ok {
						csvEntryMap[HeaderProviderWorkloads] = fmt.Sprintf("%s;%s", val, pce.WorkloadMapH[p.Workload.Href].Hostname)
					} else {
						csvEntryMap[HeaderProviderWorkloads] = pce.WorkloadMapH[p.Workload.Href].Hostname
					}
					// Check the labels
					for _, l := range wkldHrefMap[p.Workload.Href].Labels {
						if val, ok := ruleFilter[l.Href]; ok {
							ruleFilter[l.Href] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a provider label on %s workload - rule %s", pce.LabelMapH[l.Href].Value, pce.LabelMapH[l.Href].Key, wkldHrefMap[p.Workload.Href].Name, r.Href), false)
						}
					}
				}
				// Virtual Servers
				if p.VirtualServer != nil {
					if val, ok := csvEntryMap[HeaderProviderVirtualServers]; ok {
						csvEntryMap[HeaderProviderVirtualServers] = fmt.Sprintf("%s;%s", val, pce.VirtualServerMapH[p.VirtualServer.Href].Name)
					} else {
						csvEntryMap[HeaderProviderVirtualServers] = pce.VirtualServerMapH[p.VirtualServer.Href].Name
					}
					for _, l := range pce.VirtualServerMapH[p.VirtualServer.Href].Labels {
						if val, ok := ruleFilter[l.Href]; ok {
							ruleFilter[l.Href] = val + 1
							utils.LogInfo(fmt.Sprintf("filter match - %s (%s) is a provider label on %s workload - rule %s", pce.LabelMapH[l.Href].Value, pce.LabelMapH[l.Href].Key, pce.VirtualServerMapH[p.VirtualServer.Href].Name, r.Href), false)
						}
					}
				}
			}

			// Services
			services := []string{}
			for _, s := range *r.IngressServices {
				if s.Href != "" && pce.ServiceMapH[s.Href].WindowsServices != nil {
					a := pce.ServiceMapH[s.Href]
					b, _ := a.ParseService()
					if !expandSVCs {
						services = append(services, pce.ServiceMapH[s.Href].Name)
					} else {
						services = append(services, fmt.Sprintf("%s (%s)", pce.ServiceMapH[s.Href].Name, strings.Join(b, ";")))
					}
				}
				if s.Href != "" && pce.ServiceMapH[s.Href].ServicePorts != nil {
					a := pce.ServiceMapH[s.Href]
					_, b := a.ParseService()
					if pce.ServiceMapH[s.Href].Name == "All Services" {
						services = append(services, "All Services")
					} else {
						if !expandSVCs {
							services = append(services, pce.ServiceMapH[s.Href].Name)
						} else {
							services = append(services, fmt.Sprintf("%s (%s)", pce.ServiceMapH[s.Href].Name, strings.Join(b, ";")))
						}
					}
				}
				if s.Href == "" {
					services = append(services, fmt.Sprintf("%d %s", s.Port, illumioapi.ProtocolList()[s.Protocol]))
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
				csvData = append(csvData, createEntrySlice(csvEntryMap))
				matchedRules++
			}
		}
		utils.LogInfo(fmt.Sprintf("%d rules exported.", matchedRules), false)
	}

	// Output the CSV Data
	if len(csvData) > 1 {
		if edge {
			csvData = edgeCSVData
		}
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-rule-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d rules from %d rulesets exported", len(csvData)-1, i), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no rulesets in PCE.", true)
	}

	if edge {
		utils.LogEndCommand("edge-rule-export")
	} else {
		utils.LogEndCommand("rule-export")
	}

}
