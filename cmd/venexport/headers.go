package venexport

const (
	HeaderHref             = "href"
	HeaderName             = "name"
	HeaderDescription      = "description"
	HeaderVenType          = "ven_type"
	HeaderHostname         = "primary_workload_hostname"
	HeaderUID              = "uid"
	HeaderStatus           = "status"
	HeaderVersion          = "version"
	HeaderActivationType   = "activation_type"
	HeaderActivePceFqdn    = "active_pce_fqdn"
	HeaderTargetPceFqdn    = "target_pce_fqdn"
	HeaderWorkloads        = "workloads"
	HeaderContainerCluster = "container_cluster"
	HeaderHealth           = "ven_health"
	HeaderWkldHref         = "wkld_href"
)

func AllHeaders() []string {
	return []string{
		HeaderHref,
		HeaderName,
		HeaderDescription,
		HeaderVenType,
		HeaderHostname,
		HeaderUID,
		HeaderStatus,
		HeaderVersion,
		HeaderActivationType,
		HeaderActivePceFqdn,
		HeaderTargetPceFqdn,
		HeaderWorkloads,
		HeaderContainerCluster,
		HeaderHealth,
		HeaderWkldHref,
	}
}
