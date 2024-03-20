package ruleexport

// Constants for header values in ruleexport and ruleimport
const (
	HeaderRulesetName                   = "ruleset_name"
	HeaderRuleSetScope                  = "ruleset_scope"
	HeaderRulesetEnabled                = "ruleset_enabled"
	HeaderRuleDescription               = "rule_description"
	HeaderRuleEnabled                   = "rule_enabled"
	HeaderUnscopedConsumers             = "unscoped_consumers"
	HeaderConsumerAllWorkloads          = "consumer_all_workloads"
	HeaderConsumerLabels                = "consumer_labels"
	HeaderConsumerLabelsExclusions      = "consumer_labels_exclusions"
	HeaderConsumerLabelGroup            = "consumer_label_groups"
	HeaderConsumerLabelGroupExclusions  = "consumer_label_groups_exclusions"
	HeaderConsumerIplists               = "consumer_iplists"
	HeaderConsumerUserGroups            = "consumer_user_groups"
	HeaderConsumerWorkloads             = "consumer_workloads"
	HeaderConsumerVirtualServices       = "consumer_virtual_services"
	HeaderConsumerUseWorkloadSubnets    = "consumer_use_workload_subnets"
	HeaderProviderAllWorkloads          = "provider_all_workloads"
	HeaderProviderLabels                = "provider_labels"
	HeaderProviderLabelsExclusions      = "provider_labels_exclusions"
	HeaderProviderLabelGroups           = "provider_label_groups"
	HeaderProviderLabelGroupsExclusions = "provider_label_groups_exclusions"
	HeaderProviderIplists               = "provider_iplists"
	HeaderProviderWorkloads             = "provider_workloads"
	HeaderProviderVirtualServices       = "provider_virtual_services"
	HeaderProviderVirtualServers        = "provider_virtual_servers"
	HeaderProviderUseWorkloadSubnets    = "provider_use_workload_subnets"
	HeaderServices                      = "services"
	HeaderConsumerResolveLabelsAs       = "consumer_resolve_labels_as"
	HeaderProviderResolveLabelsAs       = "provider_resolve_labels_as"
	HeaderMachineAuthEnabled            = "machine_auth_enabled"
	HeaderSecureConnectEnabled          = "secure_connect_enabled"
	HeaderStateless                     = "stateless_enabled"
	HeaderRulesetDescription            = "ruleset_description"
	HeaderRulesetContainsCustomIptables = "ruleset_contains_custom_iptables"
	HeaderRulesetHref                   = "ruleset_href"
	HeaderRuleHref                      = "rule_href"
	HeaderUpdateType                    = "update_type"
	HeaderNetworkType                   = "network_type"
)

func getCSVHeaders(templateFormat bool) []string {
	headers := []string{
		HeaderRulesetName,
		HeaderRulesetDescription,
		HeaderRuleSetScope,
		HeaderRulesetEnabled,
		HeaderRuleDescription,
		HeaderRuleEnabled,
		HeaderUnscopedConsumers,
		HeaderConsumerAllWorkloads,
		HeaderConsumerLabels,
		HeaderConsumerLabelsExclusions,
		HeaderConsumerLabelGroup,
		HeaderConsumerLabelGroupExclusions,
		HeaderConsumerIplists,
		HeaderConsumerUserGroups,
		HeaderConsumerWorkloads,
		HeaderConsumerVirtualServices,
		HeaderConsumerUseWorkloadSubnets,
		HeaderProviderAllWorkloads,
		HeaderProviderLabels,
		HeaderProviderLabelsExclusions,
		HeaderProviderLabelGroups,
		HeaderProviderLabelGroupsExclusions,
		HeaderProviderIplists,
		HeaderProviderWorkloads,
		HeaderProviderVirtualServices,
		HeaderProviderVirtualServers,
		HeaderProviderUseWorkloadSubnets,
		HeaderServices,
		HeaderConsumerResolveLabelsAs,
		HeaderProviderResolveLabelsAs,
		HeaderMachineAuthEnabled,
		HeaderSecureConnectEnabled,
		HeaderStateless,
		HeaderNetworkType}

	if !templateFormat {
		headers = append(headers, HeaderRulesetHref, HeaderRuleHref, HeaderUpdateType)
	}

	return headers
}

func createEntrySlice(csvEntryMap map[string]string, templateFormat bool, useSubnets bool) []string {
	entry := []string{}
	for _, h := range getCSVHeaders(templateFormat) {
		if !useSubnets && (h == HeaderConsumerUseWorkloadSubnets || h == HeaderProviderUseWorkloadSubnets) {
			continue
		}
		entry = append(entry, csvEntryMap[h])
	}
	return entry
}
