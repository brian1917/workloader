package wkldexport

const (
	HeaderHostname                 = "hostname"
	HeaderName                     = "name"
	HeaderRole                     = "role"
	HeaderApp                      = "app"
	HeaderEnv                      = "env"
	HeaderLoc                      = "loc"
	HeaderInterfaces               = "interfaces"
	HeaderPublicIP                 = "public_ip"
	HeaderMachineAuthenticationID  = "machine_authentication_id"
	HeaderIPWithDefaultGw          = "ip_with_default_gw"
	HeaderNetmaskOfIPWithDefGw     = "netmask_of_ip_with_def_gw"
	HeaderDefaultGw                = "default_gw"
	HeaderDefaultGwNetwork         = "default_gw_network"
	HeaderHref                     = "href"
	HeaderDescription              = "description"
	HeaderPolicyState              = "enforcement"
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
	HeaderVisibilityState          = "visibility"
	HeaderSPN                      = "spn"
	HeaderManaged                  = "managed"
	HeaderVulnExposureScore        = "vuln_exposure_score"
	HeaderNumVulns                 = "num_vulns"
	HeaderMaxVulnScore             = "max_vuln_score"
	HeaderVulnScore                = "vuln_score"
	HeaderVulnPortExposure         = "vuln_port_exposure"
	HeaderAnyVulnExposure          = "any_ip_vuln_exposure"
	HeaderIpListVulnExposure       = "ip_list_vuln_exposure"
)

func AllHeaders() []string {
	return []string{
		HeaderHostname,
		HeaderName,
		HeaderRole,
		HeaderApp,
		HeaderEnv,
		HeaderLoc,
		HeaderInterfaces,
		HeaderPublicIP,
		HeaderMachineAuthenticationID,
		HeaderIPWithDefaultGw,
		HeaderNetmaskOfIPWithDefGw,
		HeaderDefaultGw,
		HeaderDefaultGwNetwork,
		HeaderHref,
		HeaderDescription,
		HeaderPolicyState,
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
		HeaderAgentVersion,
		HeaderAgentID,
		HeaderActivePceFqdn,
		HeaderServiceProvider,
		HeaderDataCenter,
		HeaderDataCenterZone,
		HeaderCloudInstanceID,
		HeaderExternalDataSet,
		HeaderExternalDataReference,
		HeaderCreatedAt,
		HeaderAgentHealth,
		HeaderVisibilityState,
		HeaderSPN,
		HeaderManaged,
		HeaderVulnExposureScore,
		HeaderNumVulns,
		HeaderMaxVulnScore,
		HeaderVulnScore,
		HeaderVulnPortExposure,
		HeaderAnyVulnExposure,
		HeaderIpListVulnExposure}
}

func ImportHeaders() []string {
	return []string{
		HeaderHostname,
		HeaderName,
		HeaderRole,
		HeaderApp,
		HeaderEnv,
		HeaderLoc,
		HeaderInterfaces,
		HeaderPublicIP,
		HeaderMachineAuthenticationID,
		HeaderSPN,
		HeaderPolicyState,
		HeaderVisibilityState,
		HeaderDescription,
		HeaderOsID,
		HeaderOsDetail,
		HeaderDataCenter,
		HeaderExternalDataSet,
		HeaderExternalDataReference}
}
