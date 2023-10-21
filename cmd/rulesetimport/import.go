package rulesetimport

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Input struct {
	PCE                            illumioapi.PCE
	UpdatePCE, NoPrompt, Provision bool
	ImportFile, ProvisionComment   string
}

var input Input

func init() {
	RuleSetImportCmd.Flags().BoolVar(&input.Provision, "provision", false, "Provision changes.")
	RuleSetImportCmd.Flags().StringVar(&input.ProvisionComment, "provision-comments", "", "Provision comment.")
}

// RuleSetImportCmd runs the import command
var RuleSetImportCmd = &cobra.Command{
	Use:   "ruleset-import [csv file to import]",
	Short: "Create rulesets from a CSV file.",
	Long: `
Create or update rulesets in the PCE from a CSV file.

The following headers are acceptable (order does not matter):
- name
- enabled
- description
- scope
- href

All other headers will be ignored.

Scopes should be semi-colon separated values of label_type:label_value. Label-groups should be in the format of lg:label_group_type:value. Multiple scopes should be separated by a "|". Example of scope entires are below:
- app:erp;env:prod
- app:erp;env:prod|app:erp;env:dev
- lg:env:non-prod

If an href is provided the name, enabled, and description fields can be updated. Scopes cannot be updated.

If an href is not provided, the ruleset will be created.

Recommended to run without --update-pce first to log what will change.`,

	Run: func(cmd *cobra.Command, args []string) {

		var err error
		input.PCE, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.Logger.Fatalf("error getting PCE for csv command - %s", err)
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("command requires 1 argument for the csv file. see usage help.")
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

	// Get all rulesets
	a, err := input.PCE.GetRulesets(nil, "draft")
	utils.LogAPIRespV2("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get the Label Groups
	a, err = input.PCE.GetLabelGroups(nil, "draft")
	utils.LogAPIRespV2("GetAllLabelGroups", a)
	if err != nil {
		utils.LogError(err.Error())
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
	updateRuleSets := []newRuleSet{}

	// Declare hm to hold the headermap
	var hm map[string]int

	// Process the CSV input
csvEntries:
	for i, l := range csvInput {

		// Skip the header row
		if i == 0 {
			// Process the headers
			hm = processHeaders(l)
			continue
		}

		// If the ruleset href column is provided and there is a value, make sure it's valid.
		if rsHrefCol, ok := hm["href"]; ok && l[hm["href"]] != "" {
			var rs illumioapi.RuleSet
			if rs, ok = input.PCE.RuleSets[l[rsHrefCol]]; !ok {
				utils.LogError(fmt.Sprintf("csv line %d - provided ruleset href does not exist", i+1))
			}
			// Begin update checks
			update := false
			// Name
			if rs.Name != l[hm["name"]] {
				utils.LogInfo(fmt.Sprintf("csv line %d - ruleset name needs to be updated from %s to %s", i+1, rs.Name, l[hm["name"]]), false)
				update = true
				rs.Name = l[hm["name"]]
			}
			// Description
			if illumioapi.PtrToVal(rs.Description) != l[hm["description"]] {
				utils.LogInfo(fmt.Sprintf("csv line %d - ruleset description needs to be updated from %s to %s", i+1, illumioapi.PtrToVal(rs.Description), l[hm["description"]]), false)
				update = true
				rs.Description = illumioapi.Ptr(l[hm["description"]])
			}
			// Enabled
			csvEnabled, err := strconv.ParseBool(l[hm["enabled"]])
			if err != nil {
				utils.LogError(fmt.Sprintf("csv line %d - invalid entry for ruleset enabled. Expects true/false", i+1))
			}
			if *rs.Enabled != csvEnabled {
				utils.LogInfo(fmt.Sprintf("csv line %d - ruleset enabled needs to be updated from %s to %s", i+1, strconv.FormatBool(*rs.Enabled), strconv.FormatBool(csvEnabled)), false)
				update = true
				*rs.Enabled = csvEnabled
			}

			if update {
				updateRuleSets = append(updateRuleSets, newRuleSet{csvLine: i + 1, ruleSet: rs})
			}

			// Continue past the new rule set creation
			continue csvEntries
		}

		// Build a new ruleset with name and description
		rs := illumioapi.RuleSet{
			Name:        l[hm["name"]],
			Description: illumioapi.Ptr(l[hm["description"]])}

		t, err := strconv.ParseBool(l[hm["enabled"]])
		if err != nil {
			utils.LogError(fmt.Sprintf("csv line %d - invalid boolean value for enabled.", i+1))
		}
		rs.Enabled = &t

		// Process scopes

		// Get rid of spaces
		csvScopesStr := strings.Replace(l[hm["scope"]], " ;", ";", -1)
		csvScopesStr = strings.Replace(csvScopesStr, "; ", ";", -1)
		csvScopesStr = strings.Replace(csvScopesStr, "| ", "|", -1)
		csvScopesStr = strings.Replace(csvScopesStr, " |", "|", -1)
		csvScopesStr = strings.TrimSuffix(csvScopesStr, " ")
		csvScopesStr = strings.TrimPrefix(csvScopesStr, " ")

		// Create the csvScopes slice of slices
		csvScopes := [][]string{}

		// Split on "|" to get each scope
		scopes := strings.Split(csvScopesStr, "|")
		// Iterate over each scope to make each scope a slice
		for _, scope := range scopes {
			csvScopes = append(csvScopes, strings.Split(scope, ";"))
		}

		// Iterate over the slice of slices to process each scope

		// Star the scopes slice
		rs.Scopes = &[][]illumioapi.Scopes{}

		if csvScopesStr != "" {
			for _, scope := range csvScopes {
				rsScope := []illumioapi.Scopes{}
				for _, entity := range scope {
					if strings.HasPrefix(entity, "lg:") {
						// Remove the lg
						entity = strings.TrimPrefix(entity, "lg:")
						// Remove the key
						entity = strings.TrimPrefix(entity, strings.Split(entity, ":")[0]+":")
						// Get the label Group
						if lg, exists := input.PCE.LabelGroups[entity]; !exists {
							utils.LogError(fmt.Sprintf("csv line %d - %s doesn't exist as a label group", i+1, entity))
						} else {
							rsScope = append(rsScope, illumioapi.Scopes{LabelGroup: &illumioapi.LabelGroup{Href: lg.Href}})
						}
						continue
					}
					// It's a label
					key := strings.Split(entity, ":")[0]
					value := strings.TrimPrefix(entity, key+":")
					// Get the label
					if label, exists := input.PCE.Labels[key+value]; !exists {
						utils.LogError(fmt.Sprintf("csv line %d - %s doesn't exist as a label of type %s.", i+1, value, key))
					} else {
						rsScope = append(rsScope, illumioapi.Scopes{Label: &illumioapi.Label{Href: label.Href}})
					}
				}
				*rs.Scopes = append(*rs.Scopes, rsScope)
			}
		}

		// Append to the new ruleset
		newRuleSets = append(newRuleSets, newRuleSet{ruleSet: rs, csvLine: i + 1})
	}

	// End run if we have nothing to do
	if len(newRuleSets) == 0 && len(updateRuleSets) == 0 {
		utils.LogInfo("nothing to be done", true)
		utils.LogEndCommand("ruleset-import")
		return
	}

	// Log findings
	if !input.UpdatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d rulesets to create and %d rulesets to update. To do the import, run again using --update-pce flag.", len(newRuleSets), len(updateRuleSets)), true)
		utils.LogEndCommand("ruleset-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if input.UpdatePCE && !input.NoPrompt {
		var prompt string
		fmt.Printf("\r\n[PROMPT] - workloader identified %d rulesets to create and %d rulesets to update in %s (%s). Do you want to run the import (yes/no)? ", len(newRuleSets), len(updateRuleSets), input.PCE.FriendlyName, viper.Get(input.PCE.FriendlyName+".fqdn").(string))
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
			ruleset, a, err := input.PCE.CreateRuleset(newRuleSet.ruleSet)
			utils.LogAPIRespV2("CreateRuleSetRule", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			provisionHrefs = append(provisionHrefs, ruleset.Href)
			utils.LogInfo(fmt.Sprintf("csv line %d - created ruleset %s - %d", newRuleSet.csvLine, ruleset.Href, a.StatusCode), true)
		}
	}

	if len(updateRuleSets) > 0 {
		for _, updateRuleSet := range updateRuleSets {
			a, err := input.PCE.UpdateRuleset(updateRuleSet.ruleSet)
			utils.LogAPIRespV2("UpateRuleSet", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			provisionHrefs = append(provisionHrefs, updateRuleSet.ruleSet.Href)
			utils.LogInfo(fmt.Sprintf("csv line %d - updated ruleset %s - %d", updateRuleSet.csvLine, updateRuleSet.ruleSet.Href, a.StatusCode), true)

		}
	}

	// Provision any changes
	if input.Provision {
		a, err := input.PCE.ProvisionHref(provisionHrefs, input.ProvisionComment)
		utils.LogAPIRespV2("ProvisionHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning complete - status code %d", a.StatusCode), true)
	}

	// Log end
	utils.LogEndCommand("ruleset-import")
}

func processHeaders(headerRow []string) map[string]int {
	headerMap := make(map[string]int)
	for i, h := range headerRow {
		headerMap[h] = i
	}
	return headerMap
}
