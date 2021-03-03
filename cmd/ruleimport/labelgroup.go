package ruleimport

import (
	"fmt"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
)

func lgComparison(csvLGNames []string, rule illumioapi.Rule, pceLGMap map[string]illumioapi.LabelGroup, csvLine int, provider bool) (bool, []*illumioapi.LabelGroup) {

	// Build a map of the existing Label Groups
	ruleLGsNameMap := make(map[string]illumioapi.LabelGroup)
	connectionSide := "consumer"
	if provider {
		connectionSide = "provider"
		for _, provider := range rule.Providers {
			if provider.LabelGroup != nil {
				ruleLGsNameMap[pceLGMap[provider.LabelGroup.Href].Name] = pceLGMap[provider.LabelGroup.Href]
			}
		}
	} else {
		for _, consumer := range rule.Consumers {
			if consumer.LabelGroup != nil {
				ruleLGsNameMap[pceLGMap[consumer.LabelGroup.Href].Name] = pceLGMap[consumer.LabelGroup.Href]
			}
		}
	}

	// Build a map of the CSV provided Label Groups
	csvLGsNameMap := make(map[string]illumioapi.LabelGroup)
	for _, lgName := range csvLGNames {
		if lgName != "" {
			if lg, lgCheck := pceLGMap[lgName]; lgCheck {
				csvLGsNameMap[lg.Name] = lg
			} else {
				utils.LogError(fmt.Sprintf("CSV line %d - %s %s does not exist as an label group", csvLine, connectionSide, lgName))
			}
		}
	}

	// Set our change to false
	change := false
	if rule.Href != "" {

		// Check for Label Groups in CSV that are not in the PCE
		for _, csvLG := range csvLGsNameMap {
			if _, check := ruleLGsNameMap[csvLG.Name]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s label group in the CSV but is not in the rule. It will be added.", csvLine, csvLG.Name, connectionSide), false)
				change = true
			}
		}

		// Check for Label Groups in the PCE that are not in the CSV
		for _, existingRuleLG := range ruleLGsNameMap {
			if _, check := csvLGsNameMap[existingRuleLG.Name]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s label group in the rule but is not in the CSV. It will be removed.", csvLine, existingRuleLG.Name, connectionSide), false)
				change = true
			}
		}
	}

	returnedLGs := []*illumioapi.LabelGroup{}
	if change || rule.Href == "" {
		for _, lg := range csvLGsNameMap {
			returnedLGs = append(returnedLGs, &illumioapi.LabelGroup{Href: lg.Href})
		}
	} else {
		for _, lg := range ruleLGsNameMap {
			returnedLGs = append(returnedLGs, &illumioapi.LabelGroup{Href: lg.Href})
		}
	}

	return change, returnedLGs
}
