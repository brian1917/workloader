package rulesetexport

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

// Declare some global variables
var pce illumioapi.PCE
var err error
var debug bool
var importFile, outputFileName, outFormat string

func init() {
	RuleSetExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
}

// RuleSetExportCmd runs the export command
var RuleSetExportCmd = &cobra.Command{
	Use:   "ruleset-export",
	Short: "Create a CSV export of all rulesets in the PCE.",
	Long: `
Create a CSV export of all rulesets in the PCE.

Note - any label groups used in scopes will have "-lg" appended to their name to differentiate labels and label groups.

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

		exportRuleSets()
	},
}

func exportRuleSets() {
	// Log the start of the command
	utils.LogStartCommand("ruleset-export")

	// Star the csvData
	csvData := [][]string{{"ruleset_name", "ruleset_enabled", "ruleset_description", "app_scope", "env_scope", "loc_scope"}}

	// Get all rulesets
	allRuleSets, a, err := pce.GetAllRuleSets("draft")
	utils.LogAPIResp("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get all label groups
	allLabelGroups, a, err := pce.GetAllLabelGroups("draft")
	utils.LogAPIResp("GetAllLabelGroups", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	labelGroupMap := make(map[string]illumioapi.LabelGroup)
	for _, lg := range allLabelGroups {
		labelGroupMap[lg.Href] = lg
	}

	// Iterate through each ruleset
	for _, rs := range allRuleSets {
		var appScopes, envScopes, locScopes []string
		// Iterate through each scope
		for _, scope := range rs.Scopes {
			var appCheck, envCheck, locCheck bool
			// Iterate through each scope entity
			for _, scopeEntity := range scope {
				if scopeEntity.Label != nil {
					// Check the key and add it to the right slice
					if pce.LabelMapH[scopeEntity.Label.Href].Key == "app" {
						appScopes = append(appScopes, pce.LabelMapH[scopeEntity.Label.Href].Value)
						appCheck = true
					}
					if pce.LabelMapH[scopeEntity.Label.Href].Key == "env" {
						envScopes = append(envScopes, pce.LabelMapH[scopeEntity.Label.Href].Value)
						envCheck = true
					}
					if pce.LabelMapH[scopeEntity.Label.Href].Key == "loc" {
						locScopes = append(locScopes, pce.LabelMapH[scopeEntity.Label.Href].Value)
						locCheck = true
					}
				}
				if scopeEntity.LabelGroup != nil {
					if labelGroupMap[scopeEntity.LabelGroup.Href].Key == "app" {
						appScopes = append(appScopes, fmt.Sprintf("%s-lg", labelGroupMap[scopeEntity.LabelGroup.Href].Name))
						appCheck = true
					}
					if labelGroupMap[scopeEntity.LabelGroup.Href].Key == "env" {
						envScopes = append(envScopes, fmt.Sprintf("%s-lg", labelGroupMap[scopeEntity.LabelGroup.Href].Name))
						envCheck = true
					}
					if labelGroupMap[scopeEntity.LabelGroup.Href].Key == "loc" {
						locScopes = append(locScopes, fmt.Sprintf("%s-lg", labelGroupMap[scopeEntity.LabelGroup.Href].Name))
						locCheck = true
					}
				}
			}
			if !appCheck {
				appScopes = append(appScopes, "all apps")
			}
			if !envCheck {
				envScopes = append(envScopes, "all envs")
			}
			if !locCheck {
				locScopes = append(locScopes, "all locs")
			}

		}

		// Append to the CSV data
		csvData = append(csvData, []string{rs.Name, strconv.FormatBool(*rs.Enabled), rs.Description, strings.Join(appScopes, ";"), strings.Join(envScopes, ";"), strings.Join(locScopes, ";")})
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
	utils.LogEndCommand("ruleset-export")

}
