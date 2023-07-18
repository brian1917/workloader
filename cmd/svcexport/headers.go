package svcexport

const (
	HeaderHref               = "href"
	HeaderName               = "name"
	HeaderDescription        = "description"
	HeaderPort               = "ports"
	HeaderProto              = "protocol"
	HeaderProcess            = "process_name"
	HeaderService            = "service_name"
	HeaderWinService         = "is_windows_service"
	HeaderICMPCode           = "icmp_code"
	HeaderICMPType           = "icmp_type"
	HeaderRansomwareCategory = "ransomware_category"
	HeaderRansomwareSeverity = "ransomware_severity"
	HeaderRansomWareOs       = "ransomware_os_platform"
)

func ImportHeaders() []string {
	return []string{
		HeaderHref,
		HeaderName,
		HeaderDescription,
		HeaderPort,
		HeaderProto,
		HeaderProcess,
		HeaderService,
		HeaderWinService,
		HeaderICMPCode,
		HeaderICMPType,
		HeaderRansomwareCategory,
		HeaderRansomwareSeverity,
		HeaderRansomWareOs,
	}
}
