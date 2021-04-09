package ruleimport

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brian1917/workloader/utils"

	"github.com/brian1917/illumioapi"
)

func serviceComparison(csvServices []string, rule illumioapi.Rule, pceServiceMap map[string]illumioapi.Service, csvLine int) (bool, []*illumioapi.IngressServices) {

	// The key in the maps is name, protocol, from, to all concatenated together
	csvServiceEntries := make(map[string]illumioapi.IngressServices)
	ruleServiceEntries := make(map[string]illumioapi.IngressServices)

	// Process the CSV provided services
	for _, c := range csvServices {

		// Check if last three letters are TCP or UDP
		if _, err := strconv.Atoi(string(c[0])); err == nil && (strings.ToLower(c[len(c)-3:]) == "tcp" || strings.ToLower(c[len(c)-3:]) == "udp") && strings.Count(c, " ") == 1 {

			protocol, port, toPort, err := parseCSVPortEntry(c)
			if err != nil {
				utils.LogError(err.Error())
			}

			// Add to our slice
			proto := 6
			if protocol == "udp" {
				proto = 17
			}
			csvServiceEntries[fmt.Sprintf("%s-%d-%d", protocol, port, toPort)] = illumioapi.IngressServices{Protocol: &proto, Port: port, ToPort: toPort}

			// Check if it's a service
		} else if service, exists := pceServiceMap[c]; exists {
			// Add to our slice
			csvServiceEntries[pceServiceMap[service.Href].Name] = illumioapi.IngressServices{Href: &service.Href}
		} else {
			utils.LogError(fmt.Sprintf("CSV line %d - %s does not exist as a service", csvLine, c))
		}
	}

	// Process the rule provided ingress services
	if rule.IngressServices != nil {
		for _, ruleService := range *rule.IngressServices {
			// Port range here
			if ruleService.Href == nil {
				protocol := "tcp"
				if *ruleService.Protocol == 17 {
					protocol = "udp"
				}
				ruleServiceEntries[fmt.Sprintf("%s-%d-%d", protocol, ruleService.Port, ruleService.ToPort)] = *ruleService
			} else {
				ruleServiceEntries[pceServiceMap[*ruleService.Href].Name] = *ruleService
			}
		}
	}

	// Initalize the change
	change := false

	// Check to see if what's in the CSV is in the PCE rule

	for s := range csvServiceEntries {
		if _, check := ruleServiceEntries[s]; !check && rule.Href != "" {
			utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a service in the CSV rule but is not in the PCE rule. It will be added.", csvLine, s), false)
			change = true
		}
	}

	// Check to see if what's in the PCE rule is in the CSV
	for s := range ruleServiceEntries {
		if _, check := csvServiceEntries[s]; !check && rule.Href != "" {
			utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a service in the PCE rule but is not in the CSV rule. It will be removed.", csvLine, s), false)
			change = true
		}
	}

	returnServices := []*illumioapi.IngressServices{}
	if change || rule.Href == "" {
		for _, s := range csvServiceEntries {
			returnServices = append(returnServices, &illumioapi.IngressServices{Port: s.Port, ToPort: s.ToPort, Href: s.Href, Protocol: s.Protocol})
		}
		return true, returnServices
	}
	return false, *rule.IngressServices
}

func parseCSVPortEntry(entry string) (protocol string, port *int, toPort *int, err error) {

	// Get the protocol
	protocol = strings.ToLower(entry[len(entry)-3:])

	// Remove the spaces
	entry = strings.Replace(entry, " ", "", -1)

	// Split the ports on the dash
	s := strings.Split(entry, "-")
	if len(s) == 1 {
		p, err := strconv.Atoi(s[0][:len(s[0])-3])
		port = &p
		if err != nil {
			return protocol, port, toPort, err
		}
	} else {
		*port, err = strconv.Atoi(s[0])
		if err != nil {
			return protocol, port, toPort, err
		}
		*toPort, err = strconv.Atoi(s[1][:len(s[1])-3])
		if err != nil {
			return protocol, port, toPort, err
		}
	}

	return protocol, port, toPort, nil

}
