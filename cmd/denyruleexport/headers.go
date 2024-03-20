package denyruleexport

const (
	HeaderName            = "name"
	HeaderHref            = "href"
	HeaderEnabled         = "enabled"
	HeaderDstAllWorkloads = "dst_all_workloads"
	HeaderDstLabels       = "dst_labels"
	HeaderDstLabelGroups  = "dst_label_groups"
	HeaderDstIPLists      = "dst_iplists"
	HeaderSrcAllWorkloads = "src_all_workloads"
	HeaderSrcLabels       = "src_labels"
	HeaderSrcLabelGroups  = "src_label_groups"
	HeaderSrcIPLists      = "src_iplists"
	HeaderServices        = "services"
	HeaderNetworkType     = "network_type"
	HeaderCreatedAt       = "created_at"
	HeaderUpdatedAt       = "updated_at"
	HeaderUpdateType      = "update_type"
)

func AllHeaders(noHref bool) []string {
	headers := []string{HeaderName}
	if !noHref {
		headers = append(headers, HeaderHref)
	}
	headers = append(headers,
		HeaderEnabled,
		HeaderSrcIPLists,
		HeaderSrcAllWorkloads,
		HeaderSrcLabels,
		HeaderSrcLabelGroups,
		HeaderDstIPLists,
		HeaderDstAllWorkloads,
		HeaderDstLabels,
		HeaderDstLabelGroups,
		HeaderServices,
		HeaderNetworkType,
		HeaderCreatedAt,
		HeaderUpdatedAt,
		HeaderUpdateType)
	return headers
}
