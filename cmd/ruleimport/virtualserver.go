package ruleimport

import (
	"fmt"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
)

func virtualServerCompare(csvVirtualServerNames []string, rule illumioapi.Rule, pceVirtualServerMap map[string]illumioapi.VirtualServer, csvLine int, provider bool) (bool, []*illumioapi.VirtualServer) {

	// Build a map of the existing Virtual Services
	ruleVirtualServerNameMap := make(map[string]illumioapi.VirtualServer)
	connectionSide := "consumer"
	if provider {
		connectionSide = "provider"
		for _, provider := range rule.Providers {
			if provider.VirtualServer != nil {
				ruleVirtualServerNameMap[pceVirtualServerMap[provider.VirtualServer.Href].Name] = pceVirtualServerMap[provider.VirtualServer.Href]
			}
		}
	}

	// Build a map of the CSV provided Virtual Services
	csvVirtualServerNameMap := make(map[string]illumioapi.VirtualServer)
	for _, vsName := range csvVirtualServerNames {
		if vsName != "" {
			if vs, vsCheck := pceVirtualServerMap[vsName]; vsCheck {
				csvVirtualServerNameMap[vs.Name] = vs
			} else {
				utils.LogError(fmt.Sprintf("CSV line %d - %s %s does not exist as a virtual server", csvLine, connectionSide, vsName))
			}
		}
	}

	// Set our change to false
	change := false
	if rule.Href != "" {

		// Check for Virtual Services in CSV that are not in the PCE
		for _, csvVirtualServer := range csvVirtualServerNameMap {
			if _, check := ruleVirtualServerNameMap[csvVirtualServer.Name]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s virtual server in the CSV but is not in the rule. It will be added.", csvLine, csvVirtualServer.Name, connectionSide), false)
				change = true
			}
		}

		// Check for Virtual Services in the PCE that are not in the CSV
		for _, existingRuleVS := range ruleVirtualServerNameMap {
			if _, check := csvVirtualServerNameMap[existingRuleVS.Name]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s virtual server in the rule but is not in the CSV. It will be removed.", csvLine, existingRuleVS.Name, connectionSide), false)
				change = true
			}
		}
	}

	returnedVirtualServers := []*illumioapi.VirtualServer{}
	if change || rule.Href == "" {
		for _, vs := range csvVirtualServerNameMap {
			returnedVirtualServers = append(returnedVirtualServers, &illumioapi.VirtualServer{Href: vs.Href})
		}
	} else {
		for _, vs := range ruleVirtualServerNameMap {
			returnedVirtualServers = append(returnedVirtualServers, &illumioapi.VirtualServer{Href: vs.Href})
		}
	}

	return change, returnedVirtualServers
}
