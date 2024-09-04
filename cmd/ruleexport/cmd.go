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

	ia "github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var err error

// RuleExport is the input format for the rule-export command
type RuleExport struct {
	PCE                                                                       *ia.PCE
	Debug, Edge, ExpandServices, TrafficCount, SkipWkldDetailCheck            bool
	OutputFileName, ExplorerStart, ExplorerEnd, ExclServiceCSV, PolicyVersion string
	ExplorerMax, TrafficRuleLimit                                             int
	NoHref                                                                    bool
	RulesetHrefs                                                              *[]string
}

var input RuleExport
var userProvidedRulesetHrefs string

// Init handles flags
func init() {
	RuleExportCmd.Flags().BoolVar(&input.NoHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	RuleExportCmd.Flags().StringVar(&userProvidedRulesetHrefs, "ruleset-hrefs", "", "a file with list of ruleset hrefs to filter. use workloader ruleset-export to get a list of rulesets and build the list of hrefs. header optional.")
	RuleExportCmd.Flags().StringVar(&input.PolicyVersion, "policy-version", "draft", "Policy version. Must be active or draft.")
	RuleExportCmd.Flags().BoolVar(&input.ExpandServices, "expand-svcs", false, "expand service objects to show ports/protocols (not compatible in rule-import format).")
	RuleExportCmd.Flags().BoolVar(&input.TrafficCount, "traffic-count", false, "include the traffic summaries for flows that meet the rule criteria. an explorer query is executed per rule, which will take some time.")
	RuleExportCmd.Flags().IntVar(&input.ExplorerMax, "traffic-max-results", 10000, "maximum results on an explorer query. only applicable if used with traffic-count flag.")
	RuleExportCmd.Flags().IntVar(&input.TrafficRuleLimit, "traffic-rule-limit", 500, "maximum number of rules to be processed for traffic. default is 500 for performance.")
	RuleExportCmd.Flags().StringVar(&input.ExplorerStart, "traffic-start", time.Now().AddDate(0, 0, -7).In(time.UTC).Format("2006-01-02"), "start date in the format of yyyy-mm-dd. only applicable if used with traffic-count flag.")
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
Create a CSV export of all rules in the input.PCE.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Validate the policy version
		input.PolicyVersion = strings.ToLower(input.PolicyVersion)
		if input.PolicyVersion != "active" && input.PolicyVersion != "draft" {
			utils.LogError("policy-version must be active or draft.")
		}

		// Get the PCE
		input.PCE = &ia.PCE{}
		*input.PCE, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		input.ExportToCsv()
	},
}

// ExportRules exports rules from the PCE
func (r *RuleExport) ExportToCsv() {

	// Initialize Slice
	input.RulesetHrefs = &[]string{}

	// Get version
	version, api, err := input.PCE.GetVersion()
	utils.LogAPIRespV2("GetVersion", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	pceVersionIncludesUseSubnets := false
	if version.Major > 22 || (version.Major == 22 && version.Minor >= 2) {
		pceVersionIncludesUseSubnets = true
	}

	// GetAllRulesets first to see what objects we need.
	utils.LogInfo("getting all rulesets...", true)
	a, err := input.PCE.GetRulesets(nil, input.PolicyVersion)
	utils.LogAPIRespV2("GetAllRuleSets", a)
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
				*input.RulesetHrefs = append(*input.RulesetHrefs, row[0])
			}
		}
	}

	allRuleSets := []ia.RuleSet{}
	if len(*input.RulesetHrefs) == 0 {
		allRuleSets = input.PCE.RuleSetsSlice
	} else {
		// Create a map
		targetRuleSets := make(map[string]bool)
		for _, h := range *input.RulesetHrefs {
			targetRuleSets[h] = true
		}
		for _, rs := range input.PCE.RuleSetsSlice {
			if targetRuleSets[rs.Href] {
				allRuleSets = append(allRuleSets, rs)
			}
		}
	}

	// Get total number of rules
	totalNumRules := 0
	for _, rs := range allRuleSets {
		for range ia.PtrToVal(rs.Rules) {
			totalNumRules++
		}
	}

	// If rules is more than 500 with traffic
	if input.TrafficCount && totalNumRules > input.TrafficRuleLimit {
		utils.LogError(fmt.Sprintf("traffic-rule-limit set to %d and total rules is %d. either use --rulset-hrefs flag to limit rules in analysis or increase limit with --traffic-rule-limit flag (potential performance impacts).", input.TrafficRuleLimit, totalNumRules))
	}

	// Run through rulesets to see what we need
	var needWklds, needLabelGroups, needVirtualServices, needVirtualServers, needUserGroups bool
	// Needed objects is for logging. Only add to this slice first time (when value is false and switches to true)
	neededObjects := map[string]bool{"labels": true, "ip_lists": true, "services": true}
	for _, rs := range allRuleSets {
		for _, scopes := range ia.PtrToVal(rs.Scopes) {
			for _, scopeEntity := range scopes {
				if scopeEntity.LabelGroup != nil {
					neededObjects["label_groups"] = true
					needLabelGroups = true
				}
			}
		}
		for _, rule := range ia.PtrToVal(rs.Rules) {
			for _, c := range ia.PtrToVal(rule.Consumers) {
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
			for _, p := range ia.PtrToVal(rule.Providers) {
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
			if rule.ConsumingSecurityPrincipals != nil && len(ia.PtrToVal(rule.ConsumingSecurityPrincipals)) > 0 {
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
	apiResps, err := input.PCE.Load(ia.LoadInput{
		Labels:                      true,
		IPLists:                     true,
		Services:                    true,
		ConsumingSecurityPrincipals: needUserGroups,
		LabelGroups:                 needLabelGroups,
		Workloads:                   needWklds,
		VirtualServices:             needVirtualServices,
		VirtualServers:              needVirtualServers,
		ProvisionStatus:             input.PolicyVersion,
	}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Check if we need workloads for checking detail
	lowCount := 0
	noCount := 0
	if input.TrafficCount && !input.SkipWkldDetailCheck {
		if !needWklds {
			api, err := input.PCE.GetWklds(map[string]string{"visibility_level": "flow_off"})
			utils.LogAPIRespV2("GetWklds?visibility_level=flow_off", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			noCount = len(input.PCE.WorkloadsSlice)

			api, err = input.PCE.GetWklds(map[string]string{"visibility_level": "flow_drops"})
			utils.LogAPIRespV2("GetWklds?visibility_level=flow_drops", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			lowCount = len(input.PCE.WorkloadsSlice)
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

				return
			}
		}

	}

	// Start the headers
	var headerSlice []string
	if input.TrafficCount {
		headerSlice = append(getCSVHeaders(input.NoHref), []string{"async_query_href", "async_query_status", "flows", "flows_by_port", "query_body"}...)
	} else {
		headerSlice = getCSVHeaders(input.NoHref)
	}

	// Remove workloadsubnets from headers based on PCE version
	if !pceVersionIncludesUseSubnets {
		tempHeaders := []string{}
		for _, header := range headerSlice {
			if header == HeaderSrcUseWorkloadSubnets || header == HeaderDstUseWorkloadSubnets {
				continue
			}
			tempHeaders = append(tempHeaders, header)
		}
		headerSlice = tempHeaders
	}

	// Start the otuput file
	if input.OutputFileName == "" {
		input.OutputFileName = fmt.Sprintf("workloader-rule-export-%s.csv", time.Now().Format("20060102_150405"))
	}
	utils.WriteLineOutput(headerSlice, input.OutputFileName)

	// Iterate each ruleset
	totalRules := 0
	totalRulesets := 0
	skippedRules := 0
	for _, rs := range allRuleSets {
		totalRulesets++

		// Log ruleset processing
		utils.LogInfo(fmt.Sprintf("processing ruleset %s with %d rules", rs.Name, len(ia.PtrToVal(rs.Rules))), false)

		// Set scope
		scopes := []string{}

		// Iterate through each scope
		for _, scope := range ia.PtrToVal(rs.Scopes) {
			scopeStrSlice := []string{}
			for _, scopeMember := range scope {
				if scopeMember.Label != nil {
					scopeStrSlice = append(scopeStrSlice, fmt.Sprintf("%s:%s", input.PCE.Labels[scopeMember.Label.Href].Key, input.PCE.Labels[scopeMember.Label.Href].Value))
				}
				if scopeMember.LabelGroup != nil {
					scopeStrSlice = append(scopeStrSlice, fmt.Sprintf("%s:%s", input.PCE.LabelGroups[scopeMember.LabelGroup.Href].Key, input.PCE.LabelGroups[scopeMember.LabelGroup.Href].Name))
				}
			}
			scopes = append(scopes, strings.Join(scopeStrSlice, ";"))
		}

		// Process each rule
		for _, rule := range ia.PtrToVal(rs.Rules) {
			totalRules++
			csvEntryMap := make(map[string]string)
			// Populate the map with basic info
			csvEntryMap[HeaderRuleSetScope] = strings.Join(scopes, ";")
			csvEntryMap[HeaderRulesetHref] = rs.Href
			csvEntryMap[HeaderRulesetEnabled] = strconv.FormatBool(ia.PtrToVal(rs.Enabled))
			csvEntryMap[HeaderRulesetDescription] = ia.PtrToVal(rs.Description)
			csvEntryMap[HeaderRulesetName] = rs.Name
			csvEntryMap[HeaderRuleHref] = rule.Href
			csvEntryMap[HeaderRuleDescription] = ia.PtrToVal(rule.Description)
			csvEntryMap[HeaderRuleEnabled] = strconv.FormatBool(ia.PtrToVal(rule.Enabled))
			csvEntryMap[HeaderUnscopedConsumers] = strconv.FormatBool(ia.PtrToVal(rule.UnscopedConsumers))
			csvEntryMap[HeaderStateless] = strconv.FormatBool(ia.PtrToVal(rule.Stateless))
			csvEntryMap[HeaderMachineAuthEnabled] = strconv.FormatBool(ia.PtrToVal(rule.MachineAuth))
			csvEntryMap[HeaderSecureConnectEnabled] = strconv.FormatBool(ia.PtrToVal(rule.SecConnect))
			csvEntryMap[HeaderNetworkType] = rule.NetworkType
			csvEntryMap[HeaderExternalDataSet] = ia.PtrToVal(rule.ExternalDataSet)
			csvEntryMap[HeaderExternalDataReference] = ia.PtrToVal(rule.ExternalDataReference)
			if rule.UpdateType == "update" {
				csvEntryMap[HeaderUpdateType] = "Modification Pending"
			} else if rule.UpdateType == "delete" {
				csvEntryMap[HeaderUpdateType] = "Deletion Pending"
			} else if rule.UpdateType == "create" {
				csvEntryMap[HeaderUpdateType] = "Addition Pending"
			} else {
				csvEntryMap[HeaderUpdateType] = rule.UpdateType
			}

			// Consumers
			consumerLabels := []string{}
			consumerLabelsExcusions := []string{}
			for _, c := range ia.PtrToVal(rule.Consumers) {
				if ia.PtrToVal(c.Actors) == "ams" {
					csvEntryMap[HeaderSrcAllWorkloads] = "true"
					continue
				}

				// IP List
				if c.IPList != nil {
					if val, ok := csvEntryMap[HeaderSrcIplists]; ok {
						csvEntryMap[HeaderSrcIplists] = fmt.Sprintf("%s;%s", val, input.PCE.IPLists[c.IPList.Href].Name)
					} else {
						csvEntryMap[HeaderSrcIplists] = input.PCE.IPLists[c.IPList.Href].Name
					}
				}
				// Labels
				if c.Label != nil {
					if c.Exclusion != nil && *c.Exclusion {
						consumerLabelsExcusions = append(consumerLabelsExcusions, fmt.Sprintf("%s:%s", input.PCE.Labels[c.Label.Href].Key, input.PCE.Labels[c.Label.Href].Value))
					} else {
						consumerLabels = append(consumerLabels, fmt.Sprintf("%s:%s", input.PCE.Labels[c.Label.Href].Key, input.PCE.Labels[c.Label.Href].Value))
					}
				}

				// Label Groups
				if c.LabelGroup != nil {
					if c.Exclusion != nil && *c.Exclusion {
						if val, ok := csvEntryMap[HeaderSrcLabelGroup]; ok {
							csvEntryMap[HeaderSrcLabelGroupExclusions] = fmt.Sprintf("%s;%s", val, input.PCE.LabelGroups[c.LabelGroup.Href].Name)
						} else {
							csvEntryMap[HeaderSrcLabelGroupExclusions] = input.PCE.LabelGroups[c.LabelGroup.Href].Name
						}
					} else {
						if val, ok := csvEntryMap[HeaderSrcLabelGroup]; ok {
							csvEntryMap[HeaderSrcLabelGroup] = fmt.Sprintf("%s;%s", val, input.PCE.LabelGroups[c.LabelGroup.Href].Name)
						} else {
							csvEntryMap[HeaderSrcLabelGroup] = input.PCE.LabelGroups[c.LabelGroup.Href].Name
						}
					}
				}
				// Virtual Services
				if c.VirtualService != nil {
					if val, ok := csvEntryMap[HeaderSrcVirtualServices]; ok {
						csvEntryMap[HeaderSrcVirtualServices] = fmt.Sprintf("%s;%s", val, input.PCE.VirtualServices[c.VirtualService.Href].Name)
					} else {
						csvEntryMap[HeaderSrcVirtualServices] = input.PCE.VirtualServices[c.VirtualService.Href].Name
					}
				}
				if c.Workload != nil {
					// Get the hostname
					pceHostname := ""
					if pceWorkload, ok := input.PCE.Workloads[c.Workload.Href]; ok {
						if ia.PtrToVal(pceWorkload.Hostname) != "" {
							pceHostname = ia.PtrToVal(pceWorkload.Hostname)
						} else {
							pceHostname = ia.PtrToVal(pceWorkload.Name)
						}
					} else {
						pceHostname = "DELETED-WORKLOAD"
					}
					if val, ok := csvEntryMap[HeaderSrcWorkloads]; ok {
						csvEntryMap[HeaderSrcWorkloads] = fmt.Sprintf("%s;%s", val, pceHostname)
					} else {
						csvEntryMap[HeaderSrcWorkloads] = pceHostname
					}
				}
			}

			// Consuming Security Principals
			consumingSecPrincipals := []string{}
			for _, csp := range ia.PtrToVal(rule.ConsumingSecurityPrincipals) {
				consumingSecPrincipals = append(consumingSecPrincipals, input.PCE.ConsumingSecurityPrincipals[csp.Href].Name)
			}
			csvEntryMap[HeaderSrcUserGroups] = strings.Join(consumingSecPrincipals, ";")

			// Providers
			providerLabels := []string{}
			providerLabelsExclusions := []string{}
			for _, p := range ia.PtrToVal(rule.Providers) {

				if ia.PtrToVal(p.Actors) == "ams" {
					csvEntryMap[HeaderDstAllWorkloads] = "true"
					continue
				}
				// IP List
				if p.IPList != nil {
					if val, ok := csvEntryMap[HeaderDstIplists]; ok {
						csvEntryMap[HeaderDstIplists] = fmt.Sprintf("%s;%s", val, input.PCE.IPLists[p.IPList.Href].Name)
					} else {
						csvEntryMap[HeaderDstIplists] = input.PCE.IPLists[p.IPList.Href].Name
					}
				}
				// Labels
				if p.Label != nil {
					if p.Exclusion != nil && *p.Exclusion {
						providerLabelsExclusions = append(providerLabelsExclusions, fmt.Sprintf("%s:%s", input.PCE.Labels[p.Label.Href].Key, input.PCE.Labels[p.Label.Href].Value))
					} else {
						providerLabels = append(providerLabels, fmt.Sprintf("%s:%s", input.PCE.Labels[p.Label.Href].Key, input.PCE.Labels[p.Label.Href].Value))
					}
				}

				// Label Groups
				if p.LabelGroup != nil {
					if p.Exclusion != nil && *p.Exclusion {
						if val, ok := csvEntryMap[HeaderDstLabelGroups]; ok {
							csvEntryMap[HeaderDstLabelGroupsExclusions] = fmt.Sprintf("%s;%s", val, input.PCE.LabelGroups[p.LabelGroup.Href].Name)
						} else {
							csvEntryMap[HeaderDstLabelGroupsExclusions] = input.PCE.LabelGroups[p.LabelGroup.Href].Name
						}
					} else {
						if val, ok := csvEntryMap[HeaderDstLabelGroups]; ok {
							csvEntryMap[HeaderDstLabelGroups] = fmt.Sprintf("%s;%s", val, input.PCE.LabelGroups[p.LabelGroup.Href].Name)
						} else {
							csvEntryMap[HeaderDstLabelGroups] = input.PCE.LabelGroups[p.LabelGroup.Href].Name
						}
					}
				}
				// Virtual Services
				if p.VirtualService != nil {
					if val, ok := csvEntryMap[HeaderDstVirtualServices]; ok {
						csvEntryMap[HeaderDstVirtualServices] = fmt.Sprintf("%s;%s", val, input.PCE.VirtualServices[p.VirtualService.Href].Name)
					} else {
						csvEntryMap[HeaderDstVirtualServices] = input.PCE.VirtualServices[p.VirtualService.Href].Name
					}
				}
				// Workloads
				if p.Workload != nil {
					// Get the hostname
					pceHostname := ""
					if pceWorkload, ok := input.PCE.Workloads[p.Workload.Href]; ok {
						if ia.PtrToVal(pceWorkload.Hostname) != "" {
							pceHostname = ia.PtrToVal(pceWorkload.Hostname)
						} else {
							pceHostname = ia.PtrToVal(pceWorkload.Name)
						}
					} else {
						pceHostname = "DELETED-WORKLOAD"
					}
					if val, ok := csvEntryMap[HeaderDstWorkloads]; ok {
						csvEntryMap[HeaderDstWorkloads] = fmt.Sprintf("%s;%s", val, pceHostname)
					} else {
						csvEntryMap[HeaderDstWorkloads] = pceHostname
					}
				}
				// Virtual Servers
				if p.VirtualServer != nil {
					if val, ok := csvEntryMap[HeaderDstVirtualServers]; ok {
						csvEntryMap[HeaderDstVirtualServers] = fmt.Sprintf("%s;%s", val, input.PCE.VirtualServers[p.VirtualServer.Href].Name)
					} else {
						csvEntryMap[HeaderDstVirtualServers] = input.PCE.VirtualServers[p.VirtualServer.Href].Name
					}
				}
			}

			// Append the labels
			csvEntryMap[HeaderSrcLabels] = strings.Join(consumerLabels, ";")
			csvEntryMap[HeaderDstLabels] = strings.Join(providerLabels, ";")
			csvEntryMap[HeaderSrcLabelsExclusions] = strings.Join(consumerLabelsExcusions, ";")
			csvEntryMap[HeaderDstLabelsExclusions] = strings.Join(providerLabelsExclusions, ";")

			// Services
			services := []string{}
			// Iterate through ingress service
			for _, s := range ia.PtrToVal(rule.IngressServices) {
				// Windows Services
				if input.PCE.Services[s.Href].WindowsServices != nil {
					a := input.PCE.Services[s.Href]
					b, _ := a.ParseService()
					if !input.ExpandServices {
						services = append(services, input.PCE.Services[s.Href].Name)
					} else {
						services = append(services, fmt.Sprintf("%s (%s)", input.PCE.Services[s.Href].Name, strings.Join(b, ";")))
					}
				}
				// Port/Proto Services
				if input.PCE.Services[s.Href].ServicePorts != nil {
					a := input.PCE.Services[s.Href]
					_, b := a.ParseService()
					if input.PCE.Services[s.Href].Name == "All Services" {
						services = append(services, "All Services")
					} else {
						if !input.ExpandServices {
							services = append(services, input.PCE.Services[s.Href].Name)
						} else {
							services = append(services, fmt.Sprintf("%s (%s)", input.PCE.Services[s.Href].Name, strings.Join(b, ";")))
						}
					}
				}

				// Port or port ranges
				if s.Href == "" {
					if ia.PtrToVal(s.ToPort) == 0 {
						services = append(services, fmt.Sprintf("%d %s", ia.PtrToVal(s.Port), ia.ProtocolList()[ia.PtrToVal(s.Protocol)]))
					} else {
						services = append(services, fmt.Sprintf("%d-%d %s", ia.PtrToVal(s.Port), ia.PtrToVal(s.ToPort), ia.ProtocolList()[ia.PtrToVal(s.Protocol)]))
					}
				}
				csvEntryMap[HeaderServices] = strings.Join(services, ";")
			}

			// Resolve As
			csvEntryMap[HeaderSrcResolveLabelsAs] = strings.Join(ia.PtrToVal(rule.ResolveLabelsAs.Consumers), ";")
			csvEntryMap[HeaderDstResolveLabelsAs] = strings.Join(ia.PtrToVal(rule.ResolveLabelsAs.Providers), ";")

			// Use Workload Subnets
			if pceVersionIncludesUseSubnets {
				csvEntryMap[HeaderSrcUseWorkloadSubnets] = "false"
				csvEntryMap[HeaderDstUseWorkloadSubnets] = "false"
				for _, u := range ia.PtrToVal(rule.UseWorkloadSubnets) {
					if u == "consumers" {
						csvEntryMap[HeaderSrcUseWorkloadSubnets] = "true"
					}
					if u == "providers" {
						csvEntryMap[HeaderDstUseWorkloadSubnets] = "true"
					}
				}
			}

			// Append to output if there are no filters or if we pass the filter checks

			// Adjust some blanks
			if csvEntryMap[HeaderSrcAllWorkloads] == "" {
				csvEntryMap[HeaderSrcAllWorkloads] = "false"
			}
			if csvEntryMap[HeaderDstAllWorkloads] == "" {
				csvEntryMap[HeaderDstAllWorkloads] = "false"
			}

			if input.TrafficCount {
				data, skipped := input.TrafficCounter(&rs, &rule, fmt.Sprintf("%d of %d", totalRules, totalNumRules))
				if skipped {
					skippedRules++
				}
				utils.WriteLineOutput(append(createEntrySlice(csvEntryMap, input.NoHref, pceVersionIncludesUseSubnets), data...), input.OutputFileName)
			} else {
				utils.WriteLineOutput(createEntrySlice(csvEntryMap, input.NoHref, pceVersionIncludesUseSubnets), input.OutputFileName)
			}

		}
	}

	utils.LogInfo(fmt.Sprintf("%d rules from %d rulesets exported", totalRules, totalRulesets), true)
	if skippedRules > 0 {
		utils.LogWarning(fmt.Sprintf("%d rules skipped because could not create valid traffic query", skippedRules), true)
	}
	utils.LogInfo(fmt.Sprintf("output file: %s", input.OutputFileName), true)

}
