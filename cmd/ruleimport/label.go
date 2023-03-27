package ruleimport

import (
	"fmt"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
)

func LabelComparison(csvLabels []illumioapi.Label, pce illumioapi.PCE, rule illumioapi.Rule, csvLine int, provider bool) (bool, []illumioapi.Label) {

	// Build a map of the existing labels
	ruleLabelMap := make(map[string]illumioapi.Label)
	connectionSide := "consumer"
	if provider {
		connectionSide = "provider"
		for _, provider := range illumioapi.PtrToVal(rule.Providers) {
			if provider.Label != nil {
				ruleLabelMap[pce.Labels[provider.Label.Href].Key+pce.Labels[provider.Label.Href].Value] = pce.Labels[provider.Label.Href]
			}
		}
	} else {
		for _, consumer := range illumioapi.PtrToVal(rule.Consumers) {
			if consumer.Label != nil {
				ruleLabelMap[pce.Labels[consumer.Label.Href].Key+pce.Labels[consumer.Label.Href].Value] = pce.Labels[consumer.Label.Href]
			}
		}
	}

	// Build a map of the CSV provided labels
	csvLabelMap := make(map[string]illumioapi.Label)
	for _, label := range csvLabels {
		if pceLabel, labelExists := pce.Labels[label.Key+label.Value]; labelExists {
			csvLabelMap[label.Key+label.Value] = pceLabel
		} else if globalInput.CreateLabels {
			if globalInput.UpdatePCE {
				createdLabel, a, err := pce.CreateLabel(illumioapi.Label{Key: label.Key, Value: label.Value})
				utils.LogAPIRespV2("CreateLabel", a)
				if err != nil {
					utils.LogError(fmt.Sprintf("csv line %d - creating label - %s", csvLine, err.Error()))
				}
				csvLabelMap[label.Key+label.Value] = createdLabel
				pce.Labels[label.Href] = createdLabel
				pce.Labels[label.Key+label.Value] = createdLabel
				utils.LogInfo(fmt.Sprintf("csv line %d - %s does not exist as a %s label. created %d", csvLine, label.Value, label.Key, a.StatusCode), true)
			} else {
				utils.LogInfo(fmt.Sprintf("csv line %d - %s does not exist as a %s label. will be created with update-pce", csvLine, label.Value, label.Key), true)
			}
		} else {
			utils.LogError(fmt.Sprintf("csv line %d - %s %s does not exist as a %s label", csvLine, connectionSide, label.Value, label.Key))
		}
	}

	// Set change to false
	change := false
	if rule.Href != "" {

		// Check for label in CSV that are not in the PCE
		for _, csvLabel := range csvLabelMap {
			if _, check := ruleLabelMap[csvLabel.Key+csvLabel.Value]; !check {
				utils.LogInfo(fmt.Sprintf("csv line %d - %s is a %s %s label in the CSV but is not in the rule. It will be added.", csvLine, csvLabel.Value, connectionSide, csvLabel.Key), false)
				change = true
			}
		}

		// Check for labels in the PCE that are not in the CSV
		for _, existingRuleLabel := range ruleLabelMap {
			if _, check := csvLabelMap[existingRuleLabel.Key+existingRuleLabel.Value]; !check {
				utils.LogInfo(fmt.Sprintf("csv line %d - %s is a %s %s label in the rule but is not in the CSV. It will be removed.", csvLine, existingRuleLabel.Value, connectionSide, existingRuleLabel.Key), false)
				change = true
			}
		}
	}

	// Build out the returned labels
	returnedLabels := []illumioapi.Label{}
	if change || rule.Href == "" {
		for _, label := range csvLabelMap {
			returnedLabels = append(returnedLabels, illumioapi.Label{Href: label.Href})
		}
	} else {
		for _, label := range ruleLabelMap {
			returnedLabels = append(returnedLabels, illumioapi.Label{Href: label.Href})
		}
	}

	return change, returnedLabels
}
