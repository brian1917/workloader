package ruleexport

// Constants for header values in ruleexport and ruleimport
const (
	HeaderRulesetName                   = "ruleset_name"
	HeaderRuleSetScope                  = "ruleset_scope"
	HeaderRulesetEnabled                = "ruleset_enabled"
	HeaderRuleDescription               = "rule_description"
	HeaderRuleEnabled                   = "rule_enabled"
	HeaderUnscopedConsumers             = "unscoped_consumers"
	HeaderSrcAllWorkloads               = "src_all_workloads"
	HeaderSrcLabels                     = "src_labels"
	HeaderSrcLabelsExclusions           = "src_labels_exclusions"
	HeaderSrcLabelGroup                 = "src_label_groups"
	HeaderSrcLabelGroupExclusions       = "src_label_groups_exclusions"
	HeaderSrcIplists                    = "src_iplists"
	HeaderSrcUserGroups                 = "src_user_groups"
	HeaderSrcWorkloads                  = "src_workloads"
	HeaderSrcVirtualServices            = "src_virtual_services"
	HeaderSrcUseWorkloadSubnets         = "src_use_workload_subnets"
	HeaderDstAllWorkloads               = "dst_all_workloads"
	HeaderDstLabels                     = "dst_labels"
	HeaderDstLabelsExclusions           = "dst_labels_exclusions"
	HeaderDstLabelGroups                = "dst_label_groups"
	HeaderDstLabelGroupsExclusions      = "dst_label_groups_exclusions"
	HeaderDstIplists                    = "dst_iplists"
	HeaderDstWorkloads                  = "dst_workloads"
	HeaderDstVirtualServices            = "dst_virtual_services"
	HeaderDstVirtualServers             = "dst_virtual_servers"
	HeaderDstUseWorkloadSubnets         = "dst_use_workload_subnets"
	HeaderServices                      = "services"
	HeaderSrcResolveLabelsAs            = "src_resolve_labels_as"
	HeaderDstResolveLabelsAs            = "dst_resolve_labels_as"
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
		HeaderSrcAllWorkloads,
		HeaderSrcLabels,
		HeaderSrcLabelsExclusions,
		HeaderSrcLabelGroup,
		HeaderSrcLabelGroupExclusions,
		HeaderSrcIplists,
		HeaderSrcUserGroups,
		HeaderSrcWorkloads,
		HeaderSrcVirtualServices,
		HeaderSrcUseWorkloadSubnets,
		HeaderDstAllWorkloads,
		HeaderDstLabels,
		HeaderDstLabelsExclusions,
		HeaderDstLabelGroups,
		HeaderDstLabelGroupsExclusions,
		HeaderDstIplists,
		HeaderDstWorkloads,
		HeaderDstVirtualServices,
		HeaderDstVirtualServers,
		HeaderDstUseWorkloadSubnets,
		HeaderServices,
		HeaderSrcResolveLabelsAs,
		HeaderDstResolveLabelsAs,
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
		if !useSubnets && (h == HeaderSrcUseWorkloadSubnets || h == HeaderDstUseWorkloadSubnets) {
			continue
		}
		entry = append(entry, csvEntryMap[h])
	}
	return entry
}
