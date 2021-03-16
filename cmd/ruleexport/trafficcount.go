package ruleexport

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
)

func trafficCounter(input Input, rs illumioapi.RuleSet, r illumioapi.Rule) []string {
	// Build the new explorer query object
	// Using the raw data structure for more flexibility versus illumioapi.TrafficQuery
	trafficReq := illumioapi.TrafficAnalysisRequest{}

	// Build the holder consumer label slice
	var consumerLabels []illumioapi.Label

	// Consumers
	for _, consumer := range r.Consumers {
		// Add labels to slice
		if consumer.Label != nil {
			consumerLabels = append(consumerLabels, input.PCE.Labels[consumer.Label.Href])
		}
		// If it's a label group, expand it and add to the slice
		if consumer.LabelGroup != nil {
			labelHrefs := input.PCE.ExpandLabelGroup(consumer.LabelGroup.Href)
			for _, labelHref := range labelHrefs {
				consumerLabels = append(consumerLabels, input.PCE.Labels[labelHref])
			}
		}
		// Add IP lists directly to the traffic query
		if consumer.IPList != nil {
			trafficReq.Sources.Include = append(trafficReq.Sources.Include, []illumioapi.Include{illumioapi.Include{IPList: &illumioapi.IPList{Href: consumer.IPList.Href}}})
		}
	}

	// Build the holder provider label slice
	var providerLabels []illumioapi.Label

	// Providers
	for _, provider := range r.Providers {
		// Add labels to slice
		if provider.Label != nil {
			providerLabels = append(providerLabels, input.PCE.Labels[provider.Label.Href])
		}
		// If it's a label group, expand it and add to the slice
		if provider.LabelGroup != nil {
			labelHrefs := input.PCE.ExpandLabelGroup(provider.LabelGroup.Href)
			for _, labelHref := range labelHrefs {
				providerLabels = append(providerLabels, input.PCE.Labels[labelHref])
			}
		}
		// Add IP lists directly to the traffic query
		if provider.IPList != nil {
			trafficReq.Destinations.Include = append(trafficReq.Destinations.Include, []illumioapi.Include{illumioapi.Include{IPList: &illumioapi.IPList{Href: provider.IPList.Href}}})
		}
	}

	// Scopes - iterate and fill provider slices. If unscoped consumers is false, also fill consumer slices. If it's a label group, expand it first.
	for _, scope := range rs.Scopes {
		for _, scopeEntity := range scope {
			if scopeEntity.Label != nil {
				providerLabels = append(providerLabels, input.PCE.Labels[scopeEntity.Label.Href])
				if !*r.UnscopedConsumers {
					consumerLabels = append(consumerLabels, input.PCE.Labels[scopeEntity.Label.Href])
				}
			}
			if scopeEntity.LabelGroup != nil {
				labelHrefs := input.PCE.ExpandLabelGroup(scopeEntity.LabelGroup.Href)
				for _, labelHref := range labelHrefs {
					providerLabels = append(providerLabels, input.PCE.Labels[labelHref])
					if !*r.UnscopedConsumers {
						consumerLabels = append(consumerLabels, input.PCE.Labels[labelHref])
					}
				}
			}
		}
	}

	// Processes the consumer labels
	consumerLabelSets, err := illumioapi.LabelsToRuleStructure(consumerLabels)
	if err != nil {
		utils.LogError(err.Error())
	}
	for _, consumerLabelSet := range consumerLabelSets {
		inc := []illumioapi.Include{}
		for _, consumerLabel := range consumerLabelSet {
			inc = append(inc, illumioapi.Include{Label: &illumioapi.Label{Href: consumerLabel.Href}})
		}
		trafficReq.Sources.Include = append(trafficReq.Sources.Include, inc)
	}

	// Process the provider labels
	providerLabelSets, err := illumioapi.LabelsToRuleStructure(providerLabels)
	if err != nil {
		utils.LogError(err.Error())
	}
	for _, providerLabels := range providerLabelSets {
		inc := []illumioapi.Include{}
		for _, providerLabel := range providerLabels {
			inc = append(inc, illumioapi.Include{Label: &illumioapi.Label{Href: providerLabel.Href}})
		}
		trafficReq.Destinations.Include = append(trafficReq.Destinations.Include, inc)
	}

	// Parse services
	// Create the array
	for _, ingressService := range *r.IngressServices {
		// Process the policy services
		if ingressService.Href != nil && *ingressService.Href != "" {
			svc := input.PCE.Services[*ingressService.Href]
			includes, _ := svc.ToExplorer()
			if len(includes) == 0 {
				trafficReq.ExplorerServices.Include = make([]illumioapi.Include, 0)
			} else {
				trafficReq.ExplorerServices.Include = append(trafficReq.ExplorerServices.Include, includes...)
			}
			// Process port ranges
		} else if ingressService.Port != nil && ingressService.ToPort != nil {
			trafficReq.ExplorerServices.Include = append(trafficReq.ExplorerServices.Include, illumioapi.Include{Port: *ingressService.Port, ToPort: *ingressService.ToPort})
			// Process ports
		} else if ingressService.Port != nil && ingressService.ToPort == nil {
			trafficReq.ExplorerServices.Include = append(trafficReq.ExplorerServices.Include, illumioapi.Include{Port: *ingressService.Port})
		} else {
			trafficReq.ExplorerServices.Include = make([]illumioapi.Include, 0)
		}
	}

	if len(*r.IngressServices) == 0 {
		trafficReq.ExplorerServices.Include = make([]illumioapi.Include, 0)
	}

	// Create empty arrays for JSON marshalling for parameters we don't need.
	trafficReq.Sources.Exclude = make([]illumioapi.Exclude, 0)
	trafficReq.Destinations.Exclude = make([]illumioapi.Exclude, 0)
	trafficReq.ExplorerServices.Exclude = make([]illumioapi.Exclude, 0)
	trafficReq.PolicyDecisions = []string{}

	// Get the start date
	t, err := time.Parse("2006-01-02 MST", input.ExplorerStart+" UTC")
	if err != nil {
		utils.LogError(err.Error())
	}
	trafficReq.StartDate = t.In(time.UTC)
	// Get the end date
	t, err = time.Parse("2006-01-02 MST", input.ExplorerEnd+" UTC")
	if err != nil {
		utils.LogError(err.Error())
	}
	trafficReq.EndDate = t.In(time.UTC)

	// Make the traffic request
	utils.LogInfo(fmt.Sprintf("ruleset %s - executing explorer query for %s...", rs.Name, r.Href), true)
	traffic, a, err := input.PCE.GetTrafficAnalysisAPI(trafficReq)
	utils.LogAPIResp("GetTrafficAnalysisAPI", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get flow count
	flows := 0
	ports := make(map[string]int)
	protocols := illumioapi.ProtocolList()
	type entry struct {
		flows int
		port  string
		proto string
	}
	entries := []entry{}
	for _, t := range traffic {
		flows = flows + t.NumConnections
		ports[fmt.Sprintf("%d-%d", t.ExpSrv.Port, t.ExpSrv.Proto)] = ports[fmt.Sprintf("%d-%d", t.ExpSrv.Port, t.ExpSrv.Proto)] + flows
	}
	for a, p := range ports {
		portProtoString := strings.Split(a, "-")
		protoInt, err := strconv.Atoi(portProtoString[1])
		if err != nil {
			utils.LogError(err.Error())
		}
		entries = append(entries, entry{port: portProtoString[0], proto: protocols[protoInt], flows: p})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].flows < entries[j].flows
	})
	entriesString := []string{}
	for _, e := range entries {
		entriesString = append(entriesString, fmt.Sprintf("%s %s (%d)", e.port, e.proto, e.flows))
	}

	return []string{strconv.Itoa(flows), strings.Join(entriesString, "; ")}
}
