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

// Input is the input format for the rule-export command
type Input struct {
	PCE                                                                       illumioapi.PCE
	Debug, Edge, ExpandServices, TrafficCount, SkipWkldDetailCheck            bool
	OutputFileName, ExplorerStart, ExplorerEnd, ExclServiceCSV, PolicyVersion string
	ExplorerMax                                                               int
	NoHref                                                                    bool
	RulesetHrefs                                                              []string
}

var input Input
var userProvidedRulesetHrefs string

// Init handles flags
func init() {
	RuleExportCmd.Flags().BoolVar(&input.NoHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	RuleExportCmd.Flags().StringVar(&userProvidedRulesetHrefs, "ruleset-hrefs", "", "a file with list of ruleset hrefs to filter. use workloader ruleset-export to get a list of rulesets and build the list of hrefs. header optional.")
	RuleExportCmd.Flags().StringVar(&input.PolicyVersion, "policy-version", "draft", "Policy version. Must be active or draft.")
	RuleExportCmd.Flags().BoolVar(&input.ExpandServices, "expand-svcs", false, "expand service objects to show ports/protocols (not compatible in rule-import format).")
	RuleExportCmd.Flags().BoolVar(&input.TrafficCount, "traffic-count", false, "include the traffic summaries for flows that meet the rule criteria. an explorer query is executed per rule, which will take some time.")
	RuleExportCmd.Flags().IntVar(&input.ExplorerMax, "traffic-max-results", 10000, "maximum results on an explorer query. only applicable if used with traffic-count flag.")
	RuleExportCmd.Flags().StringVar(&input.ExplorerStart, "traffic-start", time.Now().AddDate(0, 0, -88).In(time.UTC).Format("2006-01-02"), "start date in the format of yyyy-mm-dd. only applicable if used with traffic-count flag.")
	RuleExportCmd.Flags().StringVar(&input.ExplorerEnd, "traffic-end", time.Now().Add(time.Hour*24).Format("2006-01-02"), "end date in the format of yyyy-mm-dd. only applicable if used with traffic-count flag.")
	RuleExportCmd.Flags().StringVar(&input.ExclServiceCSV, "traffic-excl-svc-file", "", "file location of csv with port/protocols to exclude. Port number in column 1 and IANA numeric protocol in column 2. headers optional. only applicable if used with traffic-count flag.")
	RuleExportCmd.Flags().BoolVarP(&input.SkipWkldDetailCheck, "skip-wkld-detail-check", "s", false, "do not check for enforced workloads with low detail or no logging, which can skew traffic results since allowed (low detail) or all (no detail) flows are not reported. this can save time by not checking each workload enforcement state.")
	RuleExportCmd.Flags().StringVar(&input.OutputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
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
	utils.LogStartCommand("rule-export")

	// Get version
	version, api, err := input.PCE.GetVersion()
	utils.LogAPIResp("GetVersion", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	pceVersionIncludesUseSubnets := false
	if version.Major > 22 || (version.Major == 22 && version.Minor >= 2) {
		pceVersionIncludesUseSubnets = true
	}

	// GetAllRulesets
	utils.LogInfo("getting all rulesets...", true)
	allPCERulesets, a, err := input.PCE.GetRulesets(nil, input.PolicyVersion)
	utils.LogAPIResp("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Filter down the rulesets if we are given a slice
	if userProvidedRulesetHrefs != "" {
		data, err := utils.ParseCSV(userProvidedRulesetHrefs)
		if err != nil {
			utils.LogError(err.Error())
		}
		for _, row := range data {
			if strings.Contains(row[0], "/orgs/") {
				input.RulesetHrefs = append(input.RulesetHrefs, row[0])
			}
		}
	}

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

	// Load the PCE with the relevant obects (save unnecessary expensive potentially large GETs)
	neededObjectsSlice := []string{}
	for n := range neededObjects {
		neededObjectsSlice = append(neededObjectsSlice, n)
	}
	utils.LogInfo(fmt.Sprintf("getting %s ...", strings.Join(neededObjectsSlice, ", ")), true)
	apiResps, err := input.PCE.Load(illumioapi.LoadInput{
		Labels:                      true,
		IPLists:                     true,
		Services:                    true,
		ConsumingSecurityPrincipals: needUserGroups,
		LabelGroups:                 needLabelGroups,
		Workloads:                   needWklds,
		VirtualServices:             needVirtualServices,
		VirtualServers:              needVirtualServers,
		ProvisionStatus:             input.PolicyVersion,
	})
	utils.LogMultiAPIResp(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Check if we need workloads for checking detail
	lowCount := 0
	noCount := 0
	if input.TrafficCount && !input.SkipWkldDetailCheck {
		if !needWklds {
			w, api, err := input.PCE.GetWklds(map[string]string{"visibility_level": "flow_off"})
			utils.LogAPIResp("GetWklds?visibility_level=flow_off", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			noCount = len(w)

			w, api, err = input.PCE.GetWklds(map[string]string{"visibility_level": "flow_drops"})
			utils.LogAPIResp("GetWklds?visibility_level=flow_drops", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			lowCount = len(w)
		} else {
			for _, wkld := range input.PCE.Workloads {
				if wkld.GetMode() == "enforced-low" {
					lowCount++
				}
				if wkld.GetMode() == "enforced-no" {
					noCount++
				}
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

	// Start the data slice with headers
	csvData := [][]string{}
	if input.TrafficCount {
		csvData = append(csvData, append(getCSVHeaders(input.NoHref), []string{"async_query_href", "async_query_status", "flows", "flows_by_port"}...))
	} else {
		csvData = append(csvData, getCSVHeaders(input.NoHref))
	}

	// Remove workloadsubnets from headers based on PCE version

	if !pceVersionIncludesUseSubnets {
		tempHeaders := []string{}
		for _, header := range csvData[0] {
			if header == HeaderConsumerUseWorkloadSubnets || header == HeaderProviderUseWorkloadSubnets {
				continue
			}
			tempHeaders = append(tempHeaders, header)
		}
		csvData = nil
		csvData = append(csvData, tempHeaders)
	}

	// Iterate each ruleset
	var i int
	var rs illumioapi.RuleSet

	for i, rs = range allRuleSets {
		// Reset the matchedRules and filters
		matchedRules := 0

		// Log ruleset processing
		utils.LogInfo(fmt.Sprintf("processing ruleset %s with %d rules", rs.Name, len(rs.Rules)), false)

		// Set scope
		scopeSlice := []string{}

		// Iterate through each scope
		for _, scope := range rs.Scopes {
			scopeMap := make(map[string]string)
			for _, scopeMember := range scope {
				if scopeMember.Label != nil {
					scopeMap[input.PCE.Labels[scopeMember.Label.Href].Key] = input.PCE.Labels[scopeMember.Label.Href].Value
				}
				if scopeMember.LabelGroup != nil {
					scopeMap[input.PCE.LabelGroups[scopeMember.LabelGroup.Href].Key] = fmt.Sprintf("%s (label_group)", input.PCE.LabelGroups[scopeMember.LabelGroup.Href].Name)
				}
			}
		}

		// Process each rule
		for _, r := range rs.Rules {
			csvEntryMap := make(map[string]string)
			// Populate the map with basic info
			csvEntryMap[HeaderRuleSetScope] = strings.Join(scopeSlice, ";")
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

			// Consumers
			consumerLabels := []string{}
			for _, c := range r.Consumers {
				if c.Actors == "ams" {
					csvEntryMap[HeaderConsumerAllWorkloads] = "true"
					continue
				}

				// IP List
				if c.IPList != nil {
					if val, ok := csvEntryMap[HeaderConsumerIplists]; ok {
						csvEntryMap[HeaderConsumerIplists] = fmt.Sprintf("%s;%s", val, input.PCE.IPLists[c.IPList.Href].Name)
					} else {
						csvEntryMap[HeaderConsumerIplists] = input.PCE.IPLists[c.IPList.Href].Name
					}
				}
				// Labels
				if c.Label != nil {
					consumerLabels = append(consumerLabels, fmt.Sprintf("%s:%s", input.PCE.Labels[c.Label.Href].Key, input.PCE.Labels[c.Label.Href].Value))
				}

				// Label Groups
				if c.LabelGroup != nil {
					if val, ok := csvEntryMap[HeaderConsumerLabelGroup]; ok {
						csvEntryMap[HeaderConsumerLabelGroup] = fmt.Sprintf("%s;%s", val, input.PCE.LabelGroups[c.LabelGroup.Href].Name)
					} else {
						csvEntryMap[HeaderConsumerLabelGroup] = input.PCE.LabelGroups[c.LabelGroup.Href].Name
					}
				}
				// Virtual Services
				if c.VirtualService != nil {
					if val, ok := csvEntryMap[HeaderConsumerVirtualServices]; ok {
						csvEntryMap[HeaderConsumerVirtualServices] = fmt.Sprintf("%s;%s", val, input.PCE.VirtualServices[c.VirtualService.Href].Name)
					} else {
						csvEntryMap[HeaderConsumerVirtualServices] = input.PCE.VirtualServices[c.VirtualService.Href].Name
					}
				}
				if c.Workload != nil {
					// Get the hostname
					pceHostname := ""
					if pceWorkload, ok := input.PCE.Workloads[c.Workload.Href]; ok {
						if pceWorkload.Hostname != "" {
							pceHostname = pceWorkload.Hostname
						} else {
							pceHostname = pceWorkload.Name
						}
					} else {
						pceHostname = "DELETED-WORKLOAD"
					}
					if val, ok := csvEntryMap[HeaderConsumerWorkloads]; ok {
						csvEntryMap[HeaderConsumerWorkloads] = fmt.Sprintf("%s;%s", val, pceHostname)
					} else {
						csvEntryMap[HeaderConsumerWorkloads] = pceHostname
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
			providerLabels := []string{}
			for _, p := range r.Providers {

				if p.Actors == "ams" {
					csvEntryMap[HeaderProviderAllWorkloads] = "true"
					continue
				}
				// IP List
				if p.IPList != nil {
					if val, ok := csvEntryMap[HeaderProviderIplists]; ok {
						csvEntryMap[HeaderProviderIplists] = fmt.Sprintf("%s;%s", val, input.PCE.IPLists[p.IPList.Href].Name)
					} else {
						csvEntryMap[HeaderProviderIplists] = input.PCE.IPLists[p.IPList.Href].Name
					}
				}
				// Labels
				if p.Label != nil {
					providerLabels = append(providerLabels, fmt.Sprintf("%s:%s", input.PCE.Labels[p.Label.Href].Key, input.PCE.Labels[p.Label.Href].Value))
				}

				// Label Groups
				if p.LabelGroup != nil {
					if val, ok := csvEntryMap[HeaderProviderLabelGroups]; ok {
						csvEntryMap[HeaderProviderLabelGroups] = fmt.Sprintf("%s;%s", val, input.PCE.LabelGroups[p.LabelGroup.Href].Name)
					} else {
						csvEntryMap[HeaderProviderLabelGroups] = input.PCE.LabelGroups[p.LabelGroup.Href].Name
					}
				}
				// Virtual Services
				if p.VirtualService != nil {
					if val, ok := csvEntryMap[HeaderProviderVirtualServices]; ok {
						csvEntryMap[HeaderProviderVirtualServices] = fmt.Sprintf("%s;%s", val, input.PCE.VirtualServices[p.VirtualService.Href].Name)
					} else {
						csvEntryMap[HeaderProviderVirtualServices] = input.PCE.VirtualServices[p.VirtualService.Href].Name
					}
				}
				// Workloads
				if p.Workload != nil {
					// Get the hostname
					pceHostname := ""
					if pceWorkload, ok := input.PCE.Workloads[p.Workload.Href]; ok {
						if pceWorkload.Hostname != "" {
							pceHostname = pceWorkload.Hostname
						} else {
							pceHostname = pceWorkload.Name
						}
					} else {
						pceHostname = "DELETED-WORKLOAD"
					}
					if val, ok := csvEntryMap[HeaderProviderWorkloads]; ok {
						csvEntryMap[HeaderProviderWorkloads] = fmt.Sprintf("%s;%s", val, pceHostname)
					} else {
						csvEntryMap[HeaderProviderWorkloads] = pceHostname
					}
				}
				// Virtual Servers
				if p.VirtualServer != nil {
					if val, ok := csvEntryMap[HeaderProviderVirtualServers]; ok {
						csvEntryMap[HeaderProviderVirtualServers] = fmt.Sprintf("%s;%s", val, input.PCE.VirtualServers[p.VirtualServer.Href].Name)
					} else {
						csvEntryMap[HeaderProviderVirtualServers] = input.PCE.VirtualServers[p.VirtualServer.Href].Name
					}
				}
			}

			// Append the labels
			csvEntryMap[HeaderConsumerLabels] = strings.Join(consumerLabels, ";")
			csvEntryMap[HeaderProviderLabels] = strings.Join(providerLabels, ";")

			// Services
			services := []string{}
			// Iterate through ingress service
			for _, s := range *r.IngressServices {
				// Windows Services
				if s.Href != nil && input.PCE.Services[*s.Href].WindowsServices != nil {
					a := input.PCE.Services[*s.Href]
					b, _ := a.ParseService()
					if !input.ExpandServices {
						services = append(services, input.PCE.Services[*s.Href].Name)
					} else {
						services = append(services, fmt.Sprintf("%s (%s)", input.PCE.Services[*s.Href].Name, strings.Join(b, ";")))
					}
				}
				// Port/Proto Services
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

				// Port or port ranges
				if s.Href == nil {
					if s.ToPort == nil || *s.ToPort == 0 {
						services = append(services, fmt.Sprintf("%d %s", *s.Port, illumioapi.ProtocolList()[*s.Protocol]))
					} else {
						services = append(services, fmt.Sprintf("%d-%d %s", *s.Port, *s.ToPort, illumioapi.ProtocolList()[*s.Protocol]))
					}
				}
			}
			csvEntryMap[HeaderServices] = strings.Join(services, ";")

			// Resolve As
			csvEntryMap[HeaderConsumerResolveLabelsAs] = strings.Join(r.ResolveLabelsAs.Consumers, ";")
			csvEntryMap[HeaderProviderResolveLabelsAs] = strings.Join(r.ResolveLabelsAs.Providers, ";")

			// Use Workload Subnets
			if pceVersionIncludesUseSubnets {
				csvEntryMap[HeaderConsumerUseWorkloadSubnets] = "false"
				csvEntryMap[HeaderProviderUseWorkloadSubnets] = "false"
				for _, u := range r.UseWorkloadSubnets {
					if u == "consumers" {
						csvEntryMap[HeaderConsumerUseWorkloadSubnets] = "true"
					}
					if u == "providers" {
						csvEntryMap[HeaderProviderUseWorkloadSubnets] = "true"
					}
				}
			}

			// Append to output if there are no filters or if we pass the filter checks

			// Adjust some blanks
			if csvEntryMap[HeaderConsumerAllWorkloads] == "" {
				csvEntryMap[HeaderConsumerAllWorkloads] = "false"
			}
			if csvEntryMap[HeaderProviderAllWorkloads] == "" {
				csvEntryMap[HeaderProviderAllWorkloads] = "false"
			}

			if input.TrafficCount {
				csvData = append(csvData, append(createEntrySlice(csvEntryMap, input.NoHref, pceVersionIncludesUseSubnets), trafficCounter(input, rs, *r)...))
			} else {
				csvData = append(csvData, createEntrySlice(csvEntryMap, input.NoHref, pceVersionIncludesUseSubnets))
			}

			matchedRules++

		}
		utils.LogInfo(fmt.Sprintf("%d rules exported.", matchedRules), false)
	}

	// Output the CSV Data
	if len(csvData) > 1 {
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
