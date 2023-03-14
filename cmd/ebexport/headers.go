package ebexport

const (
	HeaderName                 = "name"
	HeaderHref                 = "href"
	HeaderEnabled              = "enabled"
	HeaderProviderAllWorkloads = "provider_all_workloads"
	HeaderProviderLabels       = "provider_labels"
	HeaderProviderLabelGroups  = "provider_label_groups"
	HeaderProviderIPLists      = "provider_iplists"
	HeaderConsumerAllWorkloads = "consumer_all_workloads"
	HeaderConsumerLabels       = "consumer_labels"
	HeaderConsumerLabelGroups  = "consumer_label_groups"
	HeaderConsumerIPLists      = "consumer_iplists"
	HeaderServices             = "services"
	HeaderNetworkType          = "network_type"
	HeaderCreatedAt            = "created_at"
	HeaderUpdatedAt            = "updated_at"
	HeaderUpdateType           = "update_type"
)

func AllHeaders(noHref bool) []string {
	headers := []string{HeaderName}
	if !noHref {
		headers = append(headers, HeaderHref)
	}
	headers = append(headers,
		HeaderEnabled,
		HeaderConsumerIPLists,
		HeaderConsumerAllWorkloads,
		HeaderConsumerLabels,
		HeaderConsumerLabelGroups,
		HeaderProviderIPLists,
		HeaderProviderAllWorkloads,
		HeaderProviderLabels,
		HeaderProviderLabelGroups,
		HeaderServices,
		HeaderNetworkType,
		HeaderCreatedAt,
		HeaderUpdatedAt,
		HeaderUpdateType)
	return headers
}
