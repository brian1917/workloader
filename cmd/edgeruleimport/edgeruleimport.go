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

The input file requires:
  - Header row (first line is always skipped)
  - The seven columns below in the same order.

+-------+-----------------+----------------+---------+--------------+--------------+------------------------------------------------------+
| group | consumer_iplist | consumer_group | service | rule_enabled | machine_auth |                      rule_href                       |
+-------+-----------------+----------------+---------+--------------+--------------+------------------------------------------------------+
| Sales | Private         |                | Zoom    | true         | true         |                                                      |
| HR    | Private         |                | Skype   | trye         | false        | /orgs/22/sec_policy/draft/rule_sets/76/sec_rules/135 |
+-------+-----------------+----------------+---------+--------------+--------------+------------------------------------------------------+

With no rule_href (e.g., Sales example above), the rule will be created. If there is a rule_href provided (e.g., HR example above), the rule will be updated.

Note - consumer_group field and machine_auth should not be generally used.

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetDefaultPCE(true)
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
	svcCol := 3
	ruleEnabledCol := 4
	maCol := 5
	rHrefCol := 6

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
	iplNameMap := make(map[string]illumioapi.IPList)
	for _, ipl := range allIPLs {
		iplNameMap[ipl.Name] = ipl
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
	}

	// Get LabelMaps
	a, err = pce.GetLabelMaps()
	utils.LogAPIResp("GetLabelMaps", a)
	if err != nil {
		utils.LogError(err.Error())
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

		// Check  if the ruleset exists
		if rs, rsCheck := rsNameMap[l[groupCol]]; rsCheck {

			// Check if the rule exists
			if _, rCheck := rHrefMap[l[rHrefCol]]; !rCheck {
				utils.LogError(fmt.Sprintf("CSV line - %d - the provided rule href does not exist", i+1))
			}

			// Consumers
			consumers := []*illumioapi.Consumers{}
			if l[conIPLCol] != "" {
				if ipl, iplCheck := iplNameMap[l[conIPLCol]]; !iplCheck {
					utils.LogError(fmt.Sprintf("CSV line - %d - %s does not exist as an IP List.", i+1, l[conIPLCol]))
				} else {
					// Check if we need to update
					if l[rHrefCol] != "" {
						if len(rHrefMap[l[rHrefCol]].Consumers) > 1 || rHrefMap[l[rHrefCol]].Consumers[0].IPList.Href != ipl.Href {
							update = true
							utils.LogInfo(fmt.Sprintf("CSV Line - %d - consumer IP list needs to be updated.", i), false)
						}
					}
					consumers = append(consumers, &illumioapi.Consumers{IPList: &illumioapi.IPList{Href: ipl.Href}})
				}
			}
			if l[conGrpCol] != "" {
				if label, labelCheck := pce.LabelMapKV["role"+l[conGrpCol]]; !labelCheck {
					utils.LogError(fmt.Sprintf("CSV line - %d - %s does not exist as a group", i+1, l[conGrpCol]))
				} else {
					// Check if we need to update
					if l[rHrefCol] != "" {
						if len(rHrefMap[l[rHrefCol]].Consumers) > 1 || rHrefMap[l[rHrefCol]].Consumers[0].Label.Href != label.Href {
							update = true
							utils.LogInfo(fmt.Sprintf("CSV Line - %d - consumer group list needs to be updated.", i), false)
						}
					}
					consumers = append(consumers, &illumioapi.Consumers{Label: &illumioapi.Label{Href: label.Href}})
				}
			}

			// Providers
			providers := []*illumioapi.Providers{}
			if label, labelCheck := pce.LabelMapKV["role"+l[groupCol]]; !labelCheck {
				utils.LogError(fmt.Sprintf("CSV line - %d - %s does not exist as a group", i+1, l[groupCol]))
			} else {
				// Check if we need to update
				if l[rHrefCol] != "" {
					if len(rHrefMap[l[rHrefCol]].Providers) > 1 || rHrefMap[l[rHrefCol]].Providers[0].Label.Href != label.Href {
						update = true
						utils.LogInfo(fmt.Sprintf("CSV Line - %d - provider group needs to be updated.", i), false)
					}
				}
				providers = append(providers, &illumioapi.Providers{Label: &illumioapi.Label{Href: label.Href}})
			}

			// Services
			ingressSvc := []*illumioapi.IngressServices{}
			if svc, svcCheck := svcNameMap[l[svcCol]]; !svcCheck {
				utils.LogError(fmt.Sprintf("CSV line - %d - %s does not exist as a service.", i+1, l[svcCol]))
			} else {
				// Check if we need to update
				if l[rHrefCol] != "" {
					if len(rHrefMap[l[rHrefCol]].IngressServices) > 1 || rHrefMap[l[rHrefCol]].IngressServices[0].Href != svc.Href {
						update = true
						utils.LogInfo(fmt.Sprintf("CSV Line - %d - service needs to be updated.", i), false)
					}
				}
				ingressSvc = append(ingressSvc, &illumioapi.IngressServices{Href: svcNameMap[l[svcCol]].Href})
			}

			// Enabled
			enabled, err := strconv.ParseBool(l[ruleEnabledCol])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line - %d - %s is not valid boolean", i+1, l[ruleEnabledCol]))
			}
			if l[rHrefCol] != "" {
				if rHrefMap[l[rHrefCol]].Enabled != enabled {
					update = true
					utils.LogInfo(fmt.Sprintf("CSV Line - %d - enabled status needs to be updated.", i), false)
				}
			}

			// MachineAuth
			machineAuth, err := strconv.ParseBool(l[maCol])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line - %d - %s is not valid boolean", i+1, l[maCol]))
			}
			if l[rHrefCol] != "" {
				if rHrefMap[l[rHrefCol]].MachineAuth != machineAuth {
					update = true
					utils.LogInfo(fmt.Sprintf("CSV Line - %d - machine auth status needs to be updated.", i), false)
				}
			}

			// Create the rule
			csvRule := illumioapi.Rule{Consumers: consumers, Providers: providers, IngressServices: ingressSvc, Enabled: enabled, MachineAuth: machineAuth, ResolveLabelsAs: &illumioapi.ResolveLabelsAs{Consumers: []string{"workloads"}, Providers: []string{"workloads"}}}

			// Add to our array
			if l[rHrefCol] == "" {
				newRules = append(newRules, toAdd{ruleSetHref: rs.Href, rule: csvRule, csvLine: i + 1})
				utils.LogInfo(fmt.Sprintf("CSV Line - %d - create new rule for group %s", i, l[groupCol]), true)
			} else {
				if update {
					csvRule.Href = l[rHrefCol]
					updatedRules = append(updatedRules, toAdd{ruleSetHref: rs.Href, rule: csvRule, csvLine: i + 1})
				}
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
			utils.LogInfo(fmt.Sprintf("CSV Line - %d - created rule %s - %d", newRule.csvLine, rule.Href, a.StatusCode), true)
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
			utils.LogInfo(fmt.Sprintf("CSV Line - %d - updated rule %s - %d", updatedRule.csvLine, updatedRule.rule.Href, a.StatusCode), true)
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
