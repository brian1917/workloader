package rulesetimport

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

type Input struct {
	PCE                                          illumioapi.PCE
	UpdatePCE, NoPrompt, Provision, CreateLabels bool
	ImportFile, ProvisionComment                 string
}

var input Input

func init() {
	RuleSetImportCmd.Flags().BoolVar(&input.Provision, "provision", false, "Provision changes.")
	RuleSetImportCmd.Flags().StringVar(&input.ProvisionComment, "provision-comments", "", "Provision comment.")
	RuleSetImportCmd.Flags().BoolVar(&input.CreateLabels, "create-labels", false, "Create labels in scope if they do not exist.")
}

// RuleSetImportCmd runs the import command
var RuleSetImportCmd = &cobra.Command{
	Use:   "ruleset-import [csv file to import]",
	Short: "Create rulesets from a CSV file.",
	Long: `
Create rulesets in the PCE from a CSV file.

If the app_scope, env_scope, or loc_scope is a label group, the name bust be appended with "-lg" in the CSV. The PCE object does not need any modification; it's just a tag in the CSV to tell workloader to look up the object as a label group.

To use the "All" construct, you must use all_apps, all_envs, or all_locs.

Multiple scopes can be separated by semi-colons. For example, the input below would create a ruleset with two scopes: App1 | Prod | All Locations and App 1 | Dev | All Locations.
+--------------+-----------------+---------------------+-----------+-----------+------------------+
| ruleset_name | ruleset_enabled | ruleset_description | app_scope | env_scope |    loc_scope     |
+--------------+-----------------+---------------------+-----------+-----------+------------------+
| Example      | TRUE            | This is a test      | App1;App1 | Prod;Dev  | all locs;all loc |
+--------------+-----------------+---------------------+-----------+-----------+------------------+

The order of the CSV columns do not matter. The input format requires the following headers:
- ruleset_name
- ruleset_enabled
- ruleset_description
- app_scope
- env_scope
- loc_scope

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
		input.UpdatePCE = viper.Get("update_pce").(bool)
		input.NoPrompt = viper.Get("no_prompt").(bool)

		ImportRuleSetsFromCSV(input)
	},
}

func ImportRuleSetsFromCSV(input Input) {

	// Log the start of the command
	utils.LogStartCommand("ruleset-import")

	// Get all rulesets
	pceRuleSets, a, err := input.PCE.GetAllRuleSets("draft")
	utils.LogAPIResp("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create the Ruleset HREF map
	rsMap := make(map[string]illumioapi.RuleSet)
	for _, rs := range pceRuleSets {
		rsMap[rs.Name] = rs
		rsMap[rs.Href] = rs
	}

	// Get the Label Groups
	allLabelGroups, a, err := input.PCE.GetAllLabelGroups("draft")
	utils.LogAPIResp("GetAllLabelGroups", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	labelGroupMap := make(map[string]illumioapi.LabelGroup)
	for _, lg := range allLabelGroups {
		labelGroupMap[lg.Name] = lg
		labelGroupMap[lg.Href] = lg
	}

	// Parse the CSV file
	csvInput, err := utils.ParseCSV(input.ImportFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create the array for new rulesets
	type newRuleSet struct {
		ruleSet illumioapi.RuleSet
		csvLine int
	}
	newRuleSets := []newRuleSet{}

	// Declare hm to hold the headermap
	var hm map[string]int

	// Process the CSV input
	for i, l := range csvInput {

		// Skip the header row

		if i == 0 {
			// Process the headers
			hm = processHeaders(l)
			continue
		}

		// If the ruleset column is provided and there is a value, make sure it's valid.
		if rsHrefCol, ok := hm["ruleset_href"]; ok && l[hm["ruleset_href"]] != "" {
			var rs illumioapi.RuleSet
			if rs, ok = rsMap[l[rsHrefCol]]; !ok {
				utils.LogError(fmt.Sprintf("CSV line %d - provided ruleset href does not exist", i+1))
			}
			// Begin update checks
			//update := false
			// Name
			if rs.Name != l[hm["ruleset_name"]] {
				utils.LogInfo(fmt.Sprintf("CSV line %d - ruleset name needs to be updated from %s to %s", i+1, rs.Name, l[hm["ruleset_name"]]), false)
				//update = true
			}
			// Description
			if rs.Description != l[hm["ruleset_description"]] {
				utils.LogInfo(fmt.Sprintf("CSV line %d - ruleset description needs to be updated from %s to %s", i+1, rs.Description, l[hm["ruleset_description"]]), false)
				//update = true
			}
			// Enabled
			csvEnabled, err := strconv.ParseBool(l[hm["ruleset_enabled"]])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - invalid entry for ruleset enabled. Expects true/false", i+1))
			}
			if *rs.Enabled != csvEnabled {
				utils.LogInfo(fmt.Sprintf("CSV line %d - ruleset enabled needs to be updated from %s to %s", i+1, strconv.FormatBool(*rs.Enabled), strconv.FormatBool(csvEnabled)), false)
				//update = true
			}
			// Scopes - coming soon
		}

		// Build a new ruleset with name and description
		rs := illumioapi.RuleSet{
			Name:        l[hm["ruleset_name"]],
			Description: l[hm["ruleset_description"]]}

		t, err := strconv.ParseBool(l[hm["ruleset_enabled"]])
		if err != nil {
			utils.LogError(fmt.Sprintf("CSV line %d - invalid boolean value for ruleset_enabled.", i+1))
		}
		rs.Enabled = &t

		// Set scopes
		apps := strings.Split(strings.Replace(l[hm["app_scope"]], "; ", ";", -1), ";")
		envs := strings.Split(strings.Replace(l[hm["env_scope"]], "; ", ";", -1), ";")
		locs := strings.Split(strings.Replace(l[hm["loc_scope"]], "; ", ";", -1), ";")

		// Validate the correct length
		if len(apps) != len(envs) || len(apps) != len(locs) {
			utils.LogError(fmt.Sprintf("CSV line %d - app, env, and loc scopes must be of equal length", i+1))
		}

		// The csvScopes will be a slice of slices. Inner slices will be the app, env, and loc.
		csvScopes := [][]string{}
		for n := range apps {
			csvScopes = append(csvScopes, []string{apps[n], envs[n], locs[n]})
		}

		// Process the scopes
		for _, scope := range csvScopes {
			// Declare the scope for each run
			rsScope := []*illumioapi.Scopes{}
			keys := []string{"app", "env", "loc"}
			for n, entry := range scope {
				// If the entry ends in "-lg", it's a label group
				if len(entry) >= 3 && strings.ToLower(entry[len(entry)-3:]) == "-lg" {
					// Check if the label group exists. Log if it does not.
					if val, ok := labelGroupMap[strings.Replace(entry, "-lg", "", -1)]; !ok {
						utils.LogError(fmt.Sprintf("CSV line %d - the label group %s does not exist", i+1, strings.Replace(entry, "-lg", "", -1)))
					} else {
						// If it does exist, add the label group the scope
						rsScope = append(rsScope, &illumioapi.Scopes{LabelGroup: &illumioapi.LabelGroup{Href: val.Href}})
						// Don't process this entry any more if it matched.
						continue
					}
				}
				// If it's not a label group, check if it's all <key>s and skip it. If it is all, we don't add it to the scope because no entry for a key is considered "all"
				if entry == fmt.Sprintf("all %ss", keys[n]) {
					continue
				}
				// If it's not all <key>s, check if the value exists. If it doesn't log error.
				if val, ok := input.PCE.Labels[keys[n]+entry]; !ok {
					if !input.CreateLabels {
						utils.LogError(fmt.Sprintf("CSV line %d - the %s label %s does not exist", i+1, keys[n], entry))
					} else if input.UpdatePCE {
						l, a, err := input.PCE.CreateLabel(illumioapi.Label{Key: keys[n], Value: entry})
						utils.LogAPIResp("CreateLabel", a)
						if err != nil {
							utils.LogError(fmt.Sprintf("CSV line %d - %s", i+1, err.Error()))
						}
						utils.LogInfo(fmt.Sprintf("CSV line %d - %s does not exist as a %s label - created %d", i+1, entry, keys[n], a.StatusCode), true)
						input.PCE.Labels[l.Href] = l
						input.PCE.Labels[keys[n]+entry] = l
						rsScope = append(rsScope, &illumioapi.Scopes{Label: &illumioapi.Label{Href: l.Href}})
					} else {
						utils.LogInfo(fmt.Sprintf("CSV line %d - %s does not exist as a %s label - label will be created with --update-pce", i+1, entry, keys[n]), true)
					}
					// If the value does exist, we add it to the scope
				} else {
					rsScope = append(rsScope, &illumioapi.Scopes{Label: &illumioapi.Label{Href: val.Href}})
				}
			}
			// Append the rsScope to the ruleset
			rs.Scopes = append(rs.Scopes, rsScope)
		}

		// Append to the new ruleset
		newRuleSets = append(newRuleSets, newRuleSet{ruleSet: rs, csvLine: i + 1})
	}

	// End run if we have nothing to do
	if len(newRuleSets) == 0 {
		utils.LogInfo("nothing to be done", true)
		utils.LogEndCommand("ruleset-import")
		return
	}

	// Log findings
	if !input.UpdatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d rulesets to create. To do the import, run again using --update-pce flag.", len(newRuleSets)), true)
		utils.LogEndCommand("ruleset-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if input.UpdatePCE && !input.NoPrompt {
		var prompt string
		fmt.Printf("\r\n[PROMPT] - workloader identified %d rulesets to create in %s (%s). Do you want to run the import (yes/no)? ", len(newRuleSets), input.PCE.FriendlyName, viper.Get(input.PCE.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied.", true)
			utils.LogEndCommand("ruleset-import")
			return
		}
	}

	// Create the new rules
	provisionHrefs := []string{}
	if len(newRuleSets) > 0 {
		for _, newRuleSet := range newRuleSets {
			ruleset, a, err := input.PCE.CreateRuleSet(newRuleSet.ruleSet)
			utils.LogAPIResp("CreateRuleSetRule", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			provisionHrefs = append(provisionHrefs, ruleset.Href)
			utils.LogInfo(fmt.Sprintf("CSV line %d - created ruleset %s - %d", newRuleSet.csvLine, ruleset.Href, a.StatusCode), true)
		}
	}

	// Provision any changes
	if input.Provision {
		a, err := input.PCE.ProvisionHref(provisionHrefs, input.ProvisionComment)
		utils.LogAPIResp("ProvisionHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning complete - status code %d", a.StatusCode), true)
	}

	// Log end
	utils.LogEndCommand("rule-import")
}

func processHeaders(headerRow []string) map[string]int {
	headerMap := make(map[string]int)
	for i, h := range headerRow {
		headerMap[h] = i
	}
	reqHeaders := []string{"ruleset_name", "ruleset_enabled", "ruleset_description", "app_scope", "env_scope", "loc_scope"}
	for _, header := range reqHeaders {
		if _, ok := headerMap[header]; !ok {
			utils.LogError(fmt.Sprintf("required header %s not in input file", header))
		}
	}
	return headerMap
}
