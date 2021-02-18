package wkldimport

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/brian1917/workloader/utils"
)

var fields []int

func (i *Input) processHeaders(headers []string) {

	// Convert the first row into a map
	headerMap := make(map[string]int)
	for i, header := range headers {
		headerMap[header] = i + 1
	}

	// Get the fieldMap
	fieldMap := fieldMapping()

	// Create our from CSV input
	for header, col := range headerMap {
		switch fieldMap[header] {
		case "hostname":
			if i.HostnameIndex == 99999 {
				i.HostnameIndex = col
			}
		case "name":
			if i.NameIndex == 99999 {
				i.NameIndex = col
			}
		case "role":
			if i.RoleIndex == 99999 {
				i.RoleIndex = col
			}
		case "app":
			if i.AppIndex == 99999 {
				i.AppIndex = col
			}
		case "env":
			if i.EnvIndex == 99999 {
				i.EnvIndex = col
			}
		case "loc":
			if i.LocIndex == 99999 {
				i.LocIndex = col
			}
		case "interfaces":
			if i.IntIndex == 99999 {
				i.IntIndex = col
			}
		case "description":
			if i.DescIndex == 99999 {
				i.DescIndex = col
			}
		case "href":
			if i.HrefIndex == 99999 {
				i.HrefIndex = col
			}
		case "external_data_set":
			if i.ExtDataSetIndex == 99999 {
				i.ExtDataSetIndex = col
			}
		case "external_data_reference":
			if i.ExtDataRefIndex == 99999 {
				i.ExtDataRefIndex = col
			}
		case "public_ip":
			if i.PublicIPIndex == 99999 {
				i.PublicIPIndex = col
			}
		case "os_id":
			if i.OSIDIndex == 99999 {
				i.OSIDIndex = col
			}
		case "os_detail":
			if i.OSDetailIndex == 99999 {
				i.OSDetailIndex = col
			}
		case "datacenter":
			if i.DatacenterIndex == 99999 {
				i.DatacenterIndex = col
			}
		}

	}

	// Find the match column
	if i.MatchIndex != 99999 {
		utils.LogInfo(fmt.Sprintf("match column set to %d by user", i.MatchIndex), false)
		return
	}
	if i.Umwl {
		i.MatchIndex = i.HostnameIndex
		utils.LogInfo(fmt.Sprintf("match column set to hostname column (%d) because umwl is enabled", i.MatchIndex), false)
		return
	}
	if i.HrefIndex != 99999 {
		i.MatchIndex = i.HrefIndex
		utils.LogInfo(fmt.Sprintf("match column set to href column (%d) because umwl is disabled and href provided", i.MatchIndex), false)
		return
	}
	if i.HostnameIndex != 99999 {
		i.MatchIndex = i.HostnameIndex
		utils.LogInfo(fmt.Sprintf("match column set to hostname column (%d) because href column is not provided", i.MatchIndex), false)
		return
	}
	if i.NameIndex != 99999 {
		i.MatchIndex = i.NameIndex
		utils.LogInfo(fmt.Sprintf("match column set to name column (%d) because href and hostname column is not provided", i.MatchIndex), false)
		return
	}

	utils.LogError("cannot set a match column based on provided input")
}

func fieldMapping() map[string]string {
	// Check for the existing of the headers
	fieldMapping := make(map[string]string)

	// Hostname
	fieldMapping["hostname"] = "hostname"
	fieldMapping["host"] = "hostname"
	fieldMapping["host_name"] = "hostname"
	fieldMapping["host name"] = "hostname"

	// Name
	fieldMapping["name"] = "name"

	// Role
	fieldMapping["role"] = "role"
	fieldMapping["role label"] = "role"
	fieldMapping["role_label"] = "role"
	fieldMapping["rolelabel"] = "role"
	fieldMapping["suggested_role"] = "role" // for traffic command
	fieldMapping["edge_group"] = "role"     // for edge

	// App
	fieldMapping["app"] = "app"
	fieldMapping["app label"] = "app"
	fieldMapping["app_label"] = "app"
	fieldMapping["applabel"] = "app"
	fieldMapping["application"] = "app"
	fieldMapping["application label"] = "app"
	fieldMapping["application_label"] = "app"
	fieldMapping["applicationlabel"] = "app"
	fieldMapping["suggested_app"] = "app" // for traffic command

	// Env
	fieldMapping["env"] = "env"
	fieldMapping["env label"] = "env"
	fieldMapping["env_label"] = "env"
	fieldMapping["envlabel"] = "env"
	fieldMapping["environment"] = "env"
	fieldMapping["environment label"] = "env"
	fieldMapping["environment"] = "env"
	fieldMapping["environmentlabel"] = "env"
	fieldMapping["suggested_env"] = "env" // for traffic command

	// Loc
	fieldMapping["loc"] = "loc"
	fieldMapping["Loc label"] = "loc"
	fieldMapping["loc_label"] = "loc"
	fieldMapping["loclabel"] = "loc"
	fieldMapping["location"] = "loc"
	fieldMapping["location label"] = "loc"
	fieldMapping["location"] = "loc"
	fieldMapping["locationlabel"] = "loc"
	fieldMapping["suggested_loc"] = "env" // for traffic command

	// Interfaces
	fieldMapping["interfaces"] = "interfaces"
	fieldMapping["interface"] = "interfaces"
	fieldMapping["ifaces"] = "interfaces"
	fieldMapping["iface"] = "interfaces"
	fieldMapping["ip"] = "interfaces"
	fieldMapping["ip_address"] = "interfaces"
	fieldMapping["ips"] = "interfaces"

	// Description
	fieldMapping["description"] = "description"
	fieldMapping["desc"] = "description"

	// Href
	fieldMapping["href"] = "href"

	// External Data Set
	fieldMapping["external_data_set"] = "external_data_set"

	// External Data Reference
	fieldMapping["external_data_reference"] = "external_data_reference"

	// Public IP
	fieldMapping["public_ip"] = "public_ip"

	// OS ID
	fieldMapping["os_id"] = "os_id"

	// OS Detail
	fieldMapping["os_detail"] = "os_detail"

	// Datacenter
	fieldMapping["datacenter"] = "datacenter"

	return fieldMapping
}

func (i *Input) decreaseColBy1() {
	if i.MatchIndex != 0 {
		i.MatchIndex--
	}
	if i.HostnameIndex != 0 {
		i.HostnameIndex--
	}
	if i.HrefIndex != 0 {
		i.HrefIndex--
	}
	if i.NameIndex != 0 {
		i.NameIndex--
	}
	if i.RoleIndex != 0 {
		i.RoleIndex--
	}
	if i.AppIndex != 0 {
		i.AppIndex--
	}
	if i.EnvIndex != 0 {
		i.EnvIndex--
	}
	if i.LocIndex != 0 {
		i.LocIndex--
	}
	if i.IntIndex != 0 {
		i.IntIndex--
	}
	if i.DescIndex != 0 {
		i.DescIndex--
	}
	if i.ExtDataSetIndex != 0 {
		i.ExtDataSetIndex--
	}
	if i.ExtDataRefIndex != 0 {
		i.ExtDataRefIndex--
	}
	if i.PublicIPIndex != 0 {
		i.PublicIPIndex--
	}
	if i.OSIDIndex != 0 {
		i.OSIDIndex--
	}
	if i.OSDetailIndex != 0 {
		i.OSDetailIndex--
	}
	if i.DatacenterIndex != 0 {
		i.DatacenterIndex--
	}
}

func (i *Input) log() {

	v := reflect.ValueOf(*i)

	logEntry := []string{}
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Name == "PCE" || v.Type().Field(i).Name == "KeepAllPCEInterfaces" || v.Type().Field(i).Name == "FQDNtoHostname" {
			continue
		}
		logEntry = append(logEntry, fmt.Sprintf("%s: %v", v.Type().Field(i).Name, v.Field(i).Interface()))
	}
	utils.LogInfo(strings.Join(logEntry, "; "), false)
}
