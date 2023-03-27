package ruleimport

import (
	"fmt"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
)

func wkldComparison(csvWkldNames []string, rule illumioapi.Rule, pceWkldMap map[string]illumioapi.Workload, csvLine int, provider bool) (bool, []illumioapi.Workload) {

	// Build a map of the existing Workloads
	ruleWkldsNameMap := make(map[string]illumioapi.Workload)
	connectionSide := "consumer"
	if provider {
		connectionSide = "provider"
		for _, provider := range illumioapi.PtrToVal(rule.Providers) {
			if provider.Workload != nil {
				ruleWkldsNameMap[illumioapi.PtrToVal(pceWkldMap[provider.Workload.Href].Name)] = pceWkldMap[provider.Workload.Href]
			}
		}
	} else {
		for _, consumer := range illumioapi.PtrToVal(rule.Consumers) {
			if consumer.Workload != nil {
				ruleWkldsNameMap[illumioapi.PtrToVal(pceWkldMap[consumer.Workload.Href].Name)] = pceWkldMap[consumer.Workload.Href]
			}
		}
	}

	// Build a map of the CSV provided Workloads
	csvWkldsNameMap := make(map[string]illumioapi.Workload)
	for _, wkldName := range csvWkldNames {
		if wkldName != "" {
			if wkld, wkldCheck := pceWkldMap[wkldName]; wkldCheck {
				csvWkldsNameMap[illumioapi.PtrToVal(wkld.Name)] = wkld
			} else {
				utils.LogError(fmt.Sprintf("CSV line %d - %s %s does not exist as a workload", csvLine, connectionSide, wkldName))
			}
		}
	}

	// Set our wkldChange to false
	change := false
	if rule.Href != "" {

		// Check for Workloads in CSV that are not in the PCE
		for _, csvWkld := range csvWkldsNameMap {
			if _, check := ruleWkldsNameMap[illumioapi.PtrToVal(csvWkld.Name)]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s workload in the CSV but is not in the rule. It will be added.", csvLine, illumioapi.PtrToVal(csvWkld.Name), connectionSide), false)
				change = true
			}
		}

		// Check for Workloads in the PCE that are not in the CSV
		for _, existingRuleWkld := range ruleWkldsNameMap {
			if _, check := csvWkldsNameMap[illumioapi.PtrToVal(existingRuleWkld.Name)]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s workload in the rule but is not in the CSV. It will be removed.", csvLine, illumioapi.PtrToVal(existingRuleWkld.Name), connectionSide), false)
				change = true
			}
		}
	}

	returnedWklds := []illumioapi.Workload{}
	if change || rule.Href == "" {
		for _, wkld := range csvWkldsNameMap {
			returnedWklds = append(returnedWklds, illumioapi.Workload{Href: wkld.Href})
		}
	} else {
		for _, wkld := range ruleWkldsNameMap {
			returnedWklds = append(returnedWklds, illumioapi.Workload{Href: wkld.Href})
		}
	}

	return change, returnedWklds
}
