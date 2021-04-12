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
	HeaderConsumerRole                  = "consumer_roles"
	HeaderConsumerApp                   = "consumer_apps"
	HeaderConsumerEnv                   = "consumer_envs"
	HeaderConsumerLoc                   = "consumer_locs"
	HeaderConsumerLabelGroup            = "consumer_label_groups"
	HeaderConsumerIplists               = "consumer_iplists"
	HeaderConsumerUserGroups            = "consumer_user_groups"
	HeaderConsumerWorkloads             = "consumer_workloads"
	HeaderConsumerVirtualServices       = "consumer_virtual_services"
	HeaderProviderAllWorkloads          = "provider_all_workloads"
	HeaderProviderRole                  = "provider_roles"
	HeaderProviderApp                   = "provider_apps"
	HeaderProviderEnv                   = "provider_envs"
	HeaderProviderLoc                   = "provider_locs"
	HeaderProviderLabelGroups           = "provider_label_groups"
	HeaderProviderIplists               = "provider_iplists"
	HeaderProviderWorkloads             = "provider_workloads"
	HeaderProviderVirtualServices       = "provider_virtual_services"
	HeaderProviderVirtualServers        = "provider_virtual_servers"
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
		HeaderConsumerRole,
		HeaderConsumerApp,
		HeaderConsumerEnv,
		HeaderConsumerLoc,
		HeaderConsumerLabelGroup,
		HeaderConsumerIplists,
		HeaderConsumerUserGroups,
		HeaderConsumerWorkloads,
		HeaderConsumerVirtualServices,
		HeaderProviderAllWorkloads,
		HeaderProviderRole,
		HeaderProviderApp,
		HeaderProviderEnv,
		HeaderProviderLoc,
		HeaderProviderLabelGroups,
		HeaderProviderIplists,
		HeaderProviderWorkloads,
		HeaderProviderVirtualServices,
		HeaderProviderVirtualServers,
		HeaderServices,
		HeaderConsumerResolveLabelsAs,
		HeaderProviderResolveLabelsAs,
		HeaderMachineAuthEnabled,
		HeaderSecureConnectEnabled,
		HeaderStateless,
		HeaderRulesetContainsCustomIptables}

	if !templateFormat {
		headers = append(headers, HeaderRulesetHref, HeaderRuleHref, HeaderUpdateType)
	}

	return headers
}

func createEntrySlice(csvEntryMap map[string]string, templateFormat bool) []string {
	entry := []string{}
	for _, h := range getCSVHeaders(templateFormat) {
		entry = append(entry, csvEntryMap[h])
	}
	return entry
}
