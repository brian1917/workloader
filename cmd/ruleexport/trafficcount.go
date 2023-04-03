package ruleexport

import (
	"fmt"
	"time"

	ia "github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
)

func (r *RuleExport) TrafficCounter(rs *ia.RuleSet, rule *ia.Rule, counterStr string) ([]string, bool) {
	// Build the new explorer query object
	// Using the raw data structure for more flexibility versus ia.TrafficQuery
	trafficReq := ia.TrafficAnalysisRequest{
		MaxResults:       r.ExplorerMax,
		Sources:          &ia.SrcOrDst{},
		Destinations:     &ia.SrcOrDst{},
		ExplorerServices: &ia.ExplorerServices{},
		PolicyDecisions:  &[]string{},
	}

	// Build the holder consumer and provider label slice
	var consumerLabels, providerLabels []ia.Label

	// Scope exists - used to for checking AMS
	scopeExists := false

	// Scopes - iterate and fill provider slices. If unscoped consumers is false, also fill consumer slices. If it's a label group, expand it first.
	if rs.Scopes != nil {
		for _, scope := range ia.PtrToVal(rs.Scopes) {
			for _, scopeEntity := range scope {
				if scopeEntity.Label != nil {
					scopeExists = true
					providerLabels = append(providerLabels, r.PCE.Labels[scopeEntity.Label.Href])
					if !ia.PtrToVal(rule.UnscopedConsumers) {
						consumerLabels = append(consumerLabels, r.PCE.Labels[scopeEntity.Label.Href])
					}
				}
				if scopeEntity.LabelGroup != nil {
					scopeExists = true
					labelHrefs := r.PCE.ExpandLabelGroup(scopeEntity.LabelGroup.Href)
					for _, labelHref := range labelHrefs {
						providerLabels = append(providerLabels, r.PCE.Labels[labelHref])
						if !ia.PtrToVal(rule.UnscopedConsumers) {
							consumerLabels = append(consumerLabels, r.PCE.Labels[labelHref])
						}
					}
				}
			}
		}
	}

	// Consumers
	for _, consumer := range ia.PtrToVal(rule.Consumers) {
		// Add labels to slice
		if consumer.Label != nil {
			consumerLabels = append(consumerLabels, r.PCE.Labels[consumer.Label.Href])
		}
		// If it's a label group, expand it and add to the slice
		if consumer.LabelGroup != nil {
			labelHrefs := r.PCE.ExpandLabelGroup(consumer.LabelGroup.Href)
			for _, labelHref := range labelHrefs {
				consumerLabels = append(consumerLabels, r.PCE.Labels[labelHref])
			}
		}
		// Add IP lists directly to the traffic query
		if consumer.IPList != nil {
			trafficReq.Sources.Include = append(trafficReq.Sources.Include, []ia.IncludeOrExclude{{IPList: &ia.IPList{Href: consumer.IPList.Href}}})
		}
		// Workload
		if consumer.Workload != nil {
			trafficReq.Sources.Include = append(trafficReq.Sources.Include, []ia.IncludeOrExclude{{Workload: &ia.Workload{Href: consumer.Workload.Href}}})
		}
		// All workloads
		if ia.PtrToVal(consumer.Actors) == "ams" && (!scopeExists || ia.PtrToVal(rule.UnscopedConsumers)) {
			trafficReq.Sources.Include = append(trafficReq.Sources.Include, []ia.IncludeOrExclude{{Actors: "ams"}})
		}

	}

	// Providers
	for _, provider := range ia.PtrToVal(rule.Providers) {
		// Add labels to slice
		if provider.Label != nil {
			providerLabels = append(providerLabels, r.PCE.Labels[provider.Label.Href])
		}
		// If it's a label group, expand it and add to the slice
		if provider.LabelGroup != nil {
			labelHrefs := r.PCE.ExpandLabelGroup(provider.LabelGroup.Href)
			for _, labelHref := range labelHrefs {
				providerLabels = append(providerLabels, r.PCE.Labels[labelHref])
			}
		}
		// Add IP lists directly to the traffic query
		if provider.IPList != nil {
			trafficReq.Destinations.Include = append(trafficReq.Destinations.Include, []ia.IncludeOrExclude{{IPList: &ia.IPList{Href: provider.IPList.Href}}})
		}
		// Workload
		if provider.Workload != nil {
			trafficReq.Destinations.Include = append(trafficReq.Destinations.Include, []ia.IncludeOrExclude{{Workload: &ia.Workload{Href: provider.Workload.Href}}})
		}
		// All workloads
		if ia.PtrToVal(provider.Actors) == "ams" && !scopeExists {
			trafficReq.Destinations.Include = append(trafficReq.Destinations.Include, []ia.IncludeOrExclude{{Actors: "ams"}})
		}
	}

	// Processes the consumer labels
	consumerLabelSets, err := ia.LabelsToRuleStructure(consumerLabels)
	if err != nil {
		utils.LogError(err.Error())
	}
	for _, consumerLabelSet := range consumerLabelSets {
		inc := []ia.IncludeOrExclude{}
		for _, consumerLabel := range consumerLabelSet {
			inc = append(inc, ia.IncludeOrExclude{Label: &ia.Label{Href: consumerLabel.Href}})
		}
		trafficReq.Sources.Include = append(trafficReq.Sources.Include, inc)
	}

	// Process the provider labels
	providerLabelSets, err := ia.LabelsToRuleStructure(providerLabels)
	if err != nil {
		utils.LogError(err.Error())
	}
	for _, providerLabels := range providerLabelSets {
		inc := []ia.IncludeOrExclude{}
		for _, providerLabel := range providerLabels {
			inc = append(inc, ia.IncludeOrExclude{Label: &ia.Label{Href: providerLabel.Href}})
		}
		trafficReq.Destinations.Include = append(trafficReq.Destinations.Include, inc)
	}

	// Check we have a valid rule
	if len(trafficReq.Sources.Include) == 0 {
		utils.LogWarning(fmt.Sprintf("rule %s - %s - ruleset %s - does not have valid consumers for explorer query: labels, label groups, workloads, ip lists, or all workloads. skipping.", counterStr, rule.Href, rs.Name), true)
		return []string{"invalid rule for querying traffic", "invalid rule for querying traffic", "invalid rule for querying traffic", "invalid rule for querying traffic", "invalid rule for querying traffic"}, true
	}
	if len(trafficReq.Destinations.Include) == 0 {
		utils.LogWarning(fmt.Sprintf("rule %s - %s - ruleset %s - does not have valid providers for explorer query: labels, label groups, workloads, ip lists, or all workloads. skipping.", counterStr, rule.Href, rs.Name), true)
		return []string{"invalid rule for querying traffic", "", "", "", ""}, true
	}
	if rule.ConsumingSecurityPrincipals != nil && len(ia.PtrToVal(rule.ConsumingSecurityPrincipals)) > 0 {
		utils.LogWarning(fmt.Sprintf("rule %s - ruleset %s - ad user groups not considered in traffic queries. %s", counterStr, rs.Name, rule.Href), true)

	}

	// Parse services
	// Create the array
	for _, ingressService := range ia.PtrToVal(rule.IngressServices) {
		// Process the policy services
		if ingressService.Href != "" {
			svc := r.PCE.Services[ingressService.Href]
			includes, _ := svc.ToExplorer()
			if len(includes) == 0 {
				trafficReq.ExplorerServices.Include = make([]ia.IncludeOrExclude, 0)
			} else {
				trafficReq.ExplorerServices.Include = append(trafficReq.ExplorerServices.Include, includes...)
			}
			// Process port ranges
		} else if ingressService.Port != nil && ingressService.ToPort != nil {
			trafficReq.ExplorerServices.Include = append(trafficReq.ExplorerServices.Include, ia.IncludeOrExclude{Port: ia.PtrToVal(ingressService.Port), ToPort: ia.PtrToVal(ingressService.ToPort)})
			// Process ports
		} else if ingressService.Port != nil && ingressService.ToPort == nil {
			trafficReq.ExplorerServices.Include = append(trafficReq.ExplorerServices.Include, ia.IncludeOrExclude{Port: ia.PtrToVal(ingressService.Port)})
		} else {
			trafficReq.ExplorerServices.Include = make([]ia.IncludeOrExclude, 0)
		}
	}

	if len(ia.PtrToVal(rule.IngressServices)) == 0 {
		trafficReq.ExplorerServices.Include = make([]ia.IncludeOrExclude, 0)
	}

	// Create empty arrays for JSON marshalling for parameters we don't need.
	trafficReq.Sources.Exclude = make([]ia.IncludeOrExclude, 0)
	trafficReq.Destinations.Exclude = make([]ia.IncludeOrExclude, 0)
	trafficReq.ExplorerServices.Exclude = make([]ia.IncludeOrExclude, 0)
	trafficReq.PolicyDecisions = &[]string{}
	_, api, err := r.PCE.GetVersion()
	utils.LogAPIRespV2("GetVersion", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	r.PCE.GetVersion()
	if r.PCE.Version.Major > 19 {
		x := false
		trafficReq.ExcludeWorkloadsFromIPListQuery = &x
	}

	// Get the start date
	t, err := time.Parse("2006-01-02 MST", r.ExplorerStart+" UTC")
	if err != nil {
		utils.LogError(err.Error())
	}
	trafficReq.StartDate = t.In(time.UTC)
	// Get the end date
	t, err = time.Parse("2006-01-02 MST", r.ExplorerEnd+" UTC")
	if err != nil {
		utils.LogError(err.Error())
	}
	trafficReq.EndDate = t.In(time.UTC)

	// Give it a name
	name := "workloader-rule-usage-" + rule.Href
	trafficReq.QueryName = &name

	// Make the traffic request
	utils.LogInfo(fmt.Sprintf("rule %s - ruleset %s - creating async explorer query for %s", counterStr, rs.Name, rule.Href), true)
	asyncTrafficQuery, a, err := r.PCE.CreateAsyncTrafficRequest(trafficReq)
	utils.LogAPIRespV2("GetTrafficAnalysisAPI", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	return []string{asyncTrafficQuery.Href, "", "", "", a.ReqBody}, false

}
