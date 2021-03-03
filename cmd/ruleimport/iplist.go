package ruleimport

import (
	"fmt"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
)

func iplComparison(csvIPLNames []string, rule illumioapi.Rule, pceIPLMap map[string]illumioapi.IPList, csvLine int, provider bool) (bool, []*illumioapi.IPList) {

	// Build a map of the existing IP Lists
	ruleIPLsNameMap := make(map[string]illumioapi.IPList)
	connectionSide := "consumer"
	if provider {
		connectionSide = "provider"
		for _, provider := range rule.Providers {
			if provider.IPList != nil {
				ruleIPLsNameMap[pceIPLMap[provider.IPList.Href].Name] = pceIPLMap[provider.IPList.Href]
			}
		}
	} else {
		for _, consumer := range rule.Consumers {
			if consumer.IPList != nil {
				ruleIPLsNameMap[pceIPLMap[consumer.IPList.Href].Name] = pceIPLMap[consumer.IPList.Href]
			}
		}
	}

	// Build a map of the CSV provided IP Lists
	csvIPLsNameMap := make(map[string]illumioapi.IPList)
	for _, iplName := range csvIPLNames {
		if iplName != "" {
			if ipl, iplCheck := pceIPLMap[iplName]; iplCheck {
				csvIPLsNameMap[ipl.Name] = ipl
			} else {
				utils.LogError(fmt.Sprintf("CSV line %d - %s %s does not exist as an IP List", csvLine, connectionSide, iplName))
			}
		}
	}

	// Set our iplChange to false
	change := false
	if rule.Href != "" {

		// Check for IP Lists in CSV that are not in the PCE
		for _, csvIPL := range csvIPLsNameMap {
			if _, check := ruleIPLsNameMap[csvIPL.Name]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s IP List in the CSV but is not in the rule. It will be added.", csvLine, csvIPL.Name, connectionSide), false)
				change = true
			}
		}

		// Check for IP Lists in the PCE that are not in the CSV
		for _, existingRuleIPL := range ruleIPLsNameMap {
			if _, check := csvIPLsNameMap[existingRuleIPL.Name]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s IP List in the rule but is not in the CSV. It will be removed.", csvLine, existingRuleIPL.Name, connectionSide), false)
				change = true
			}
		}
	}

	returnedIPLs := []*illumioapi.IPList{}
	if change || rule.Href == "" {
		for _, ipl := range csvIPLsNameMap {
			returnedIPLs = append(returnedIPLs, &illumioapi.IPList{Href: ipl.Href})
		}
	} else {
		for _, ipl := range ruleIPLsNameMap {
			returnedIPLs = append(returnedIPLs, &illumioapi.IPList{Href: ipl.Href})
		}
	}

	return change, returnedIPLs
}
