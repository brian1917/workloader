package rulesetexport

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare some global variables
var pce illumioapi.PCE
var err error
var outputFileName string
var noHref bool

func init() {
	RuleSetExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	RuleSetExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	RuleSetExportCmd.Flags().SortFlags = false

}

// RuleSetExportCmd runs the export command
var RuleSetExportCmd = &cobra.Command{
	Use:   "ruleset-export",
	Short: "Create a CSV export of all rulesets in the PCE.",
	Long: `
Create a CSV export of all rulesets in the PCE.

Label groups used in scopes will have "lg:type:" pre-pended to their name to differentiate them from labels. For example, an environment label group non-prod would appear as "lg:env:non-prod".

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		ExportRuleSets(pce, outputFileName, noHref, []string{})
	},
}

func ExportRuleSets(pce illumioapi.PCE, outputFileName string, templateFormat bool, hrefs []string) {

	// Start the csvData
	headers := []string{"ruleset_name", "enabled", "description", "scope", "contains_custom_iptables_rules"}
	if !templateFormat {
		headers = append(headers, "href")
	}
	csvData := [][]string{headers}

	// Get all rulesets and labels
	apiResps, err := pce.Load(illumioapi.LoadInput{RuleSets: true, Labels: true, ProvisionStatus: "draft"}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}
	allPCERuleSets := pce.RuleSetsSlice

	// Filter down the rulesets if we are given a slice
	allRuleSets := []illumioapi.RuleSet{}
	if len(hrefs) == 0 {
		allRuleSets = allPCERuleSets
	} else {
		// Create a map
		targetRuleSets := make(map[string]bool)
		for _, h := range hrefs {
			targetRuleSets[h] = true
		}
		for _, rs := range allPCERuleSets {
			if targetRuleSets[rs.Href] {
				allRuleSets = append(allRuleSets, rs)
			}
		}
	}

	// Determine if we need label groups
	needLabelGroups := false
	for _, rs := range allRuleSets {
		if rs.Scopes != nil {
			for _, scope := range *rs.Scopes {
				for _, entity := range scope {
					if entity.LabelGroup != nil {
						needLabelGroups = true
						break
					}
				}
			}
		}
	}

	// Get all label groups if necessary
	labelGroupMap := make(map[string]illumioapi.LabelGroup)
	if needLabelGroups {
		utils.LogInfo("ruleset scopes include label groups. getting all label groups...", true)
		a, err := pce.GetLabelGroups(nil, "draft")
		utils.LogAPIRespV2("GetAllLabelGroups", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		allLabelGroups := pce.LabelGroupsSlice
		for _, lg := range allLabelGroups {
			labelGroupMap[lg.Href] = lg
		}
	}

	// Iterate through each ruleset
	for _, rs := range allRuleSets {
		allScopesSlice := []string{}
		// Check for custom iptables rules
		customIPTables := false
		if len(illumioapi.PtrToVal(rs.IPTablesRules)) != 0 {
			customIPTables = true
		}
		utils.LogInfo(fmt.Sprintf("custom iptables rules: %t", customIPTables), false)
		// Iterate through each scope
		if rs.Scopes != nil {
			for _, scope := range *rs.Scopes {
				scopeStrSlice := []string{}
				// Iterate through each scope entity
				for _, scopeEntity := range scope {
					suffix := ""
					if illumioapi.PtrToVal(scopeEntity.Exclusion) {
						suffix = "-exclusion"
					}
					if scopeEntity.Label != nil {
						scopeStrSlice = append(scopeStrSlice, fmt.Sprintf("%s:%s%s", pce.Labels[scopeEntity.Label.Href].Key, pce.Labels[scopeEntity.Label.Href].Value, suffix))
					}
					if scopeEntity.LabelGroup != nil {
						scopeStrSlice = append(scopeStrSlice, fmt.Sprintf("lg:%s:%s%s", labelGroupMap[scopeEntity.LabelGroup.Href].Key, labelGroupMap[scopeEntity.LabelGroup.Href].Name, suffix))
					}
				}
				allScopesSlice = append(allScopesSlice, strings.Join(scopeStrSlice, ";"))
			}
		}

		// Append to the CSV data
		entry := []string{rs.Name, strconv.FormatBool(*rs.Enabled), illumioapi.PtrToVal(rs.Description), strings.Join(allScopesSlice, "|"), strconv.FormatBool(customIPTables)}
		if !templateFormat {
			entry = append(entry, rs.Href)
		}
		csvData = append(csvData, entry)
	}

	// Output the CSV Data
	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-ruleset-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d rulesets exported", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no rulesets in PCE.", true)
	}

}
