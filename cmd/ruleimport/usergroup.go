package ruleimport

import (
	"fmt"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
)

func userGroupComaprison(csvUserGroupNames []string, rule illumioapi.Rule, userGroupMapName map[string]illumioapi.ConsumingSecurityPrincipals, csvLine int) (bool, []*illumioapi.ConsumingSecurityPrincipals) {

	// Build a map of the existing user groups
	ruleUserGroupsNameMap := make(map[string]illumioapi.ConsumingSecurityPrincipals)
	for _, ug := range rule.ConsumingSecurityPrincipals {
		ruleUserGroupsNameMap[userGroupMapName[ug.Href].Name] = userGroupMapName[ug.Href]
	}

	// Build a map of the CSV provided user groups
	csvUserGroupsNameMap := make(map[string]illumioapi.ConsumingSecurityPrincipals)
	for _, ugName := range csvUserGroupNames {
		if ug, ugCheck := userGroupMapName[ugName]; ugCheck {
			csvUserGroupsNameMap[ug.Name] = ug
		} else {
			utils.LogError(fmt.Sprintf("CSV line %d - %s does not exist as a user group", csvLine, ugName))
		}
	}

	// Set change to false
	change := false
	if rule.Href != "" {

		// Check for user groups in CSV that are not in the PCE
		for _, csvUG := range csvUserGroupsNameMap {
			if _, check := ruleUserGroupsNameMap[csvUG.Name]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a user group in the CSV but is not in the rule. It will be added.", csvLine, csvUG.Name), false)
				change = true
			}
		}

		// Check for user groups in the PCE that are not in the rule
		for _, ruleUG := range ruleUserGroupsNameMap {
			if _, check := csvUserGroupsNameMap[ruleUG.Name]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a user group in the rule but is not in the CSV. It will be removed.", csvLine, ruleUG.Name), false)
				change = true
			}
		}
	}

	// Create the right consumingSecPrincipals
	consumingSecPrincipals := []*illumioapi.ConsumingSecurityPrincipals{}
	if change || rule.Href == "" {
		for _, ug := range csvUserGroupsNameMap {
			consumingSecPrincipals = append(consumingSecPrincipals, &illumioapi.ConsumingSecurityPrincipals{Href: ug.Href})
		}
	} else {
		for _, cp := range rule.ConsumingSecurityPrincipals {
			consumingSecPrincipals = append(consumingSecPrincipals, &illumioapi.ConsumingSecurityPrincipals{Href: cp.Href})
		}
	}
	return change, consumingSecPrincipals
}
