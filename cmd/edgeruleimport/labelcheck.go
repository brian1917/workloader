package edgeruleimport

import (
	"fmt"
	"strings"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
)

func labelComparison(key string, csvLabels []string, pce illumioapi.PCE, rule illumioapi.Rule, csvLine int, provider bool) (bool, []*illumioapi.Label) {

	// Build a map of the existing labels for that key
	ruleLabelValueMap := make(map[string]illumioapi.Label)
	connectionSide := "consumer"
	if provider {
		connectionSide = "provider"
		for _, provider := range rule.Providers {
			if provider.Label != nil {
				ruleLabelValueMap[pce.LabelMapH[provider.Label.Href].Value] = pce.LabelMapH[provider.Label.Href]
			}
		}
	} else {
		for _, consumer := range rule.Consumers {
			if consumer.Label != nil {
				ruleLabelValueMap[pce.LabelMapH[consumer.Label.Href].Value] = pce.LabelMapH[consumer.Label.Href]
			}
		}
	}

	// Build a map of the CSV provided labels for that key
	csvLabelValueMap := make(map[string]illumioapi.Label)
	for _, value := range csvLabels {
		if strings.ToLower(value) == "all workloads" {
			continue
		}
		if label, labelCheck := pce.LabelMapKV[key+value]; labelCheck {
			csvLabelValueMap[label.Value] = label
		} else {
			utils.LogError(fmt.Sprintf("CSV line %d - %s %s does not exist as a %s label", csvLine, connectionSide, value, key))
		}
	}

	// Set change to false
	change := false
	if rule.Href != "" {

		// Check for label in CSV that are not in the PCE
		for _, csvLabel := range csvLabelValueMap {
			if _, check := ruleLabelValueMap[csvLabel.Value]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s %s label in the CSV but is not in the rule. It will be added.", csvLine, csvLabel.Value, connectionSide, key), false)
				change = true
			}
		}

		// Check for IP Lists in the PCE that are not in the CSV
		for _, existingRuleLabel := range ruleLabelValueMap {
			if _, check := csvLabelValueMap[existingRuleLabel.Value]; !check {
				utils.LogInfo(fmt.Sprintf("CSV line %d - %s is a %s %s label in the rule but is not in the CSV. It will be removed.", csvLine, existingRuleLabel.Value, connectionSide, key), false)
				change = true
			}
		}
	}

	// Build out the returned labels
	returnedLabels := []*illumioapi.Label{}
	if change || rule.Href == "" {
		for _, label := range csvLabelValueMap {
			returnedLabels = append(returnedLabels, &illumioapi.Label{Href: label.Href})
		}
	} else {
		for _, label := range ruleLabelValueMap {
			returnedLabels = append(returnedLabels, &illumioapi.Label{Href: label.Href})
		}
	}

	return change, returnedLabels
}
