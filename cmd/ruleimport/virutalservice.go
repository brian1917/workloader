package ruleimport

import (
	"fmt"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
)

func virtualServiceCompare(csvVSNames []string, rule illumioapi.Rule, pceVSMap map[string]illumioapi.VirtualService, csvLine int, provider bool) (bool, []*illumioapi.VirtualService) {

	// Build a map of the existing Virtual Services
	ruleVirtualServicesNameMap := make(map[string]illumioapi.VirtualService)
	connectionSide := "consumer"
	if provider {
		connectionSide = "provider"
		for _, provider := range rule.Providers {
			if provider.VirtualService != nil {
				ruleVirtualServicesNameMap[pceVSMap[provider.VirtualService.Href].Name] = pceVSMap[provider.VirtualService.Href]
			}
		}
	} else {
		for _, consumer := range rule.Consumers {
			if consumer.VirtualService != nil {
				ruleVirtualServicesNameMap[pceVSMap[consumer.VirtualService.Href].Name] = pceVSMap[consumer.VirtualService.Href]
			}
		}
	}

	// Build a map of the CSV provided Virtual Services
	csvVirtualServicesNameMap := make(map[string]illumioapi.VirtualService)
	for _, vsName := range csvVSNames {
		if vsName != "" {
			if vs, vsCheck := pceVSMap[vsName]; vsCheck {
				csvVirtualServicesNameMap[vs.Name] = vs
			} else {
				utils.LogError(fmt.Sprintf("CSV line %d - %s %s does not exist as a virtual service", csvLine, connectionSide, vsName))
			}
		}
	}

	// Set our change to false
	change := false
	if rule.Href != "" {

		// Check for Virtual Services in CSV that are not in the PCE
		for _, csvVirtualService := range csvVirtualServicesNameMap {
			if _, check := ruleVirtualServicesNameMap[csvVirtualService.Name]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s virtual service in the CSV but is not in the rule. It will be added.", csvLine, csvVirtualService.Name, connectionSide), false)
				change = true
			}
		}

		// Check for Virtual Services in the PCE that are not in the CSV
		for _, existingRuleVS := range ruleVirtualServicesNameMap {
			if _, check := csvVirtualServicesNameMap[existingRuleVS.Name]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s virtual service in the rule but is not in the CSV. It will be removed.", csvLine, existingRuleVS.Name, connectionSide), false)
				change = true
			}
		}
	}

	returnedVirtualServices := []*illumioapi.VirtualService{}
	if change || rule.Href == "" {
		for _, vs := range csvVirtualServicesNameMap {
			returnedVirtualServices = append(returnedVirtualServices, &illumioapi.VirtualService{Href: vs.Href})
		}
	} else {
		for _, vs := range ruleVirtualServicesNameMap {
			returnedVirtualServices = append(returnedVirtualServices, &illumioapi.VirtualService{Href: vs.Href})
		}
	}

	return change, returnedVirtualServices
}
