package wkldexport

const (
	HeaderHostname                 = "hostname"
	HeaderName                     = "name"
	HeaderInterfaces               = "interfaces"
	HeaderPublicIP                 = "public_ip"
	HeaderDistinguishedName        = "distinguished_name"
	HeaderIPWithDefaultGw          = "ip_with_default_gw"
	HeaderNetmaskOfIPWithDefGw     = "netmask_of_ip_with_def_gw"
	HeaderDefaultGw                = "default_gw"
	HeaderDefaultGwNetwork         = "default_gw_network"
	HeaderHref                     = "href"
	HeaderDescription              = "description"
	HeaderEnforcement              = "enforcement"
	HeaderVisibility               = "visibility"
	HeaderOnline                   = "online"
	HeaderAgentStatus              = "agent_status"
	HeaderSecurityPolicySyncState  = "security_policy_sync_state"
	HeaderSecurityPolicyAppliedAt  = "security_policy_applied_at"
	HeaderSecurityPolicyReceivedAt = "security_policy_received_at"
	HeaderSecurityPolicyRefreshAt  = "security_policy_refresh_at"
	HeaderLastHeartbeatOn          = "last_heartbeat_on"
	HeaderHoursSinceLastHeartbeat  = "hours_since_last_heartbeat"
	HeaderOsID                     = "os_id"
	HeaderOsDetail                 = "os_detail"
	HeaderVenHref                  = "ven_href"
	HeaderAgentVersion             = "agent_version"
	HeaderAgentID                  = "agent_id"
	HeaderActivePceFqdn            = "active_pce_fqdn"
	HeaderServiceProvider          = "service_provider"
	HeaderDataCenter               = "data_center"
	HeaderDataCenterZone           = "data_center_zone"
	HeaderCloudInstanceID          = "cloud_instance_id"
	HeaderExternalDataSet          = "external_data_set"
	HeaderExternalDataReference    = "external_data_reference"
	HeaderCreatedAt                = "created_at"
	HeaderAgentHealth              = "agent_health"
	HeaderSPN                      = "spn"
	HeaderManaged                  = "managed"
	HeaderVulnExposureScore        = "vuln_exposure_score"
	HeaderNumVulns                 = "num_vulns"
	HeaderMaxVulnScore             = "max_vuln_score"
	HeaderVulnScore                = "vuln_score"
	HeaderVulnPortExposure         = "vuln_port_exposure"
	HeaderAnyVulnExposure          = "any_ip_vuln_exposure"
	HeaderIpListVulnExposure       = "ip_list_vuln_exposure"
	HeaderRansomewareExposure      = "ransomware_exposure"
	HeaderProtectionCoverageScore  = "protection_coverage_score"
)

func AllHeaders(inclVuln bool, inclHref bool) []string {
	headers := []string{
		HeaderHostname,
		HeaderName,
		HeaderInterfaces,
		HeaderPublicIP,
		HeaderDistinguishedName,
		HeaderIPWithDefaultGw,
		HeaderNetmaskOfIPWithDefGw,
		HeaderDefaultGw,
		HeaderDefaultGwNetwork,
	}
	if inclHref {
		headers = append(headers, HeaderHref)
	}
	headers = append(headers,
		HeaderDescription,
		HeaderEnforcement,
		HeaderOnline,
		HeaderAgentStatus,
		HeaderSecurityPolicySyncState,
		HeaderSecurityPolicyAppliedAt,
		HeaderSecurityPolicyReceivedAt,
		HeaderSecurityPolicyRefreshAt,
		HeaderLastHeartbeatOn,
		HeaderHoursSinceLastHeartbeat,
		HeaderOsID,
		HeaderOsDetail,
		HeaderRansomewareExposure,
		HeaderProtectionCoverageScore,
		HeaderVenHref,
		HeaderAgentVersion,
		HeaderAgentID,
		HeaderActivePceFqdn,
		HeaderServiceProvider,
		HeaderDataCenter,
		HeaderDataCenterZone,
		HeaderCloudInstanceID,
		HeaderCreatedAt,
		HeaderAgentHealth,
		HeaderVisibility,
		HeaderSPN,
		HeaderManaged)
	if inclVuln {
		headers = append(headers,
			HeaderVulnExposureScore,
			HeaderNumVulns,
			HeaderMaxVulnScore,
			HeaderVulnScore,
			HeaderVulnPortExposure,
			HeaderAnyVulnExposure,
			HeaderIpListVulnExposure)
	}
	headers = append(headers, HeaderExternalDataSet, HeaderExternalDataReference)

	return headers
}

func ImportHeaders() []string {
	return []string{
		HeaderHostname,
		HeaderName,
		HeaderInterfaces,
		HeaderPublicIP,
		HeaderDistinguishedName,
		HeaderSPN,
		HeaderEnforcement,
		HeaderVisibility,
		HeaderDescription,
		HeaderOsID,
		HeaderOsDetail,
		HeaderDataCenter,
		HeaderExternalDataSet,
		HeaderExternalDataReference}
}
