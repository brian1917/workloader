package ruleimport

import (
	"github.com/brian1917/workloader/utils"
)

func (i *Input) processHeaders(headers []string) {

	// Convert the first row into a map
	headerMap := make(map[string]int)
	for i, header := range headers {
		headerMap[header] = i
	}

	// Get the fieldMap
	fieldMap := fieldMapping()

	for header, index := range headerMap {
		x := index
		switch fieldMap[header] {
		case "ruleset_name":
			i.RulesetNameIndex = &x
		case "global_consumers":
			i.GlobalConsumerIndex = &x
		case "consumer_role":
			i.ConsRoleIndex = &x
		case "consumer_app":
			i.ConsAppIndex = &x
		case "consumer_env":
			i.ConsEnvIndex = &x
		case "consumer_loc":
			i.ConsLocIndex = &x
		case "consumer_iplists":
			i.ConsIPLIndex = &x
		case "consumer_workloads":
			i.ConsWkldIndex = &x
		case "consumer_user_group":
			i.ConsUserGroupIndex = &x
		case "consumer_label_groups":
			i.ConsLabelGroupIndex = &x
		case "provider_role":
			i.ProvRoleIndex = &x
		case "provider_app":
			i.ProvAppIndex = &x
		case "provider_env":
			i.ProvEnvIndex = &x
		case "provider_loc":
			i.ProvLocIndex = &x
		case "provider_iplists":
			i.ProvIPLIndex = &x
		case "provider_workloads":
			i.ProvWkldIndex = &x
		case "provider_label_groups":
			i.ProvLabelGroupIndex = &x
		case "services":
			i.ServicesIndex = &x
		case "rule_enabled":
			i.RuleEnabledIndex = &x
		case "machine_auth_enabled":
			i.MachineAuthIndex = &x
		case "secure_connect_enabled":
			i.SecureConnectIndex = &x
		case "rule_href":
			i.RuleHrefIndex = &x
		case "consumers_virtual_services_only":
			i.ConsVSOnlyIndex = &x
		case "consumers_virtual_services_and_workloads":
			i.ConsVSandWkldsIndex = &x
		case "providers_virtual_services_only":
			i.ProvsVSOnlyIndex = &x
		case "providers_virtual_services_and_workloads":
			i.ProvsVSandWkldIndex = &x
		}

	}

	// Check for required headers
	if i.ServicesIndex == nil {
		utils.LogError("No header for found for required field: services.")
	}
	if i.GlobalConsumerIndex == nil {
		utils.LogError("No header for found for required field: global_consumers.")
	}
	if i.RulesetNameIndex == nil {
		utils.LogError("No header for found for required field: ruleset_name.")
	}
	if i.RuleEnabledIndex == nil {
		utils.LogError("No header found for required field: rule_enabled.")
	}
	if i.ProvsVSOnlyIndex == nil {
		utils.LogError("No header found for required field: providers_virtual_services_only.")
	}
	if i.ProvsVSandWkldIndex == nil {
		utils.LogError("No header found for required field: providers_virtual_services_and_workloads.")
	}
	if i.ConsVSOnlyIndex == nil {
		utils.LogError("No header found for required field: consumers_virtual_services_only.")
	}
	if i.ConsVSandWkldsIndex == nil {
		utils.LogError("No header found for required field: consumers_virtual_services_and_workloads.")
	}
}

func fieldMapping() map[string]string {

	fieldMapping := make(map[string]string)

	fieldMapping["ruleset_name"] = "ruleset_name"
	fieldMapping["global_consumers"] = "global_consumers"
	fieldMapping["consumer_role"] = "consumer_role"
	fieldMapping["consumer_app"] = "consumer_app"
	fieldMapping["consumer_env"] = "consumer_env"
	fieldMapping["consumer_loc"] = "consumer_loc"
	fieldMapping["consumer_iplists"] = "consumer_iplists"
	fieldMapping["consumer_workloads"] = "consumer_workloads"
	fieldMapping["consumer_user_group"] = "consumer_user_group"
	fieldMapping["consumer_label_groups"] = "consumer_label_groups"
	fieldMapping["provider_role"] = "provider_role"
	fieldMapping["provider_app"] = "provider_app"
	fieldMapping["provider_env"] = "provider_env"
	fieldMapping["provider_loc"] = "provider_loc"
	fieldMapping["provider_iplists"] = "provider_iplists"
	fieldMapping["provider_workloads"] = "provider_workloads"
	fieldMapping["provider_label_groups"] = "provider_label_groups"
	fieldMapping["services"] = "services"
	fieldMapping["rule_enabled"] = "rule_enabled"
	fieldMapping["machine_auth_enabled"] = "machine_auth_enabled"
	fieldMapping["secure_connect_enabled"] = "secure_connect_enabled"
	fieldMapping["rule_href"] = "rule_href"
	fieldMapping["consumers_virtual_services_only"] = "consumers_virtual_services_only"
	fieldMapping["consumers_virtual_services_and_workloads"] = "consumers_virtual_services_and_workloads"
	fieldMapping["providers_virtual_services_only"] = "providers_virtual_services_only"
	fieldMapping["providers_virtual_services_and_workloads"] = "providers_virtual_services_and_workloads"
	return fieldMapping
}

func (i *Input) log() {

	// v := reflect.ValueOf(*i)

	// logEntry := []string{}
	// for i := 0; i < v.NumField(); i++ {
	// 	if v.Type().Field(i).Name == "PCE" {
	// 		continue
	// 	}

	// 	logEntry = append(logEntry, fmt.Sprintf("%s: %v", v.Type().Field(i).Name, v.Field(i).Interface()))

	// }
	// utils.LogInfo(strings.Join(logEntry, "; "), false)
}
