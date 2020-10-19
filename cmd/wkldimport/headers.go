package wkldimport

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/brian1917/workloader/utils"
)

func (f *FromCSVInput) processHeaders(headers []string) {

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
			if f.HostnameIndex == 99999 {
				f.HostnameIndex = col
			}
		case "name":
			if f.NameIndex == 99999 {
				f.NameIndex = col
			}
		case "role":
			if f.RoleIndex == 99999 {
				f.RoleIndex = col
			}
		case "app":
			if f.AppIndex == 99999 {
				f.AppIndex = col
			}
		case "env":
			if f.EnvIndex == 99999 {
				f.EnvIndex = col
			}
		case "loc":
			if f.LocIndex == 99999 {
				f.LocIndex = col
			}
		case "interfaces":
			if f.IntIndex == 99999 {
				f.IntIndex = col
			}
		case "description":
			if f.DescIndex == 99999 {
				f.DescIndex = col
			}
		case "href":
			if f.HrefIndex == 99999 {
				f.HrefIndex = col
			}
		}
	}

	// Find the match column
	if f.MatchIndex != 99999 {
		utils.LogInfo(fmt.Sprintf("match column set to %d by user", f.MatchIndex), false)
		return
	}
	if f.Umwl {
		f.MatchIndex = f.HostnameIndex
		utils.LogInfo(fmt.Sprintf("match column set to hostname column (%d) because umwl is enabled", f.MatchIndex), false)
		return
	}
	if f.HrefIndex != 99999 {
		f.MatchIndex = f.HrefIndex
		utils.LogInfo(fmt.Sprintf("match column set to href column (%d) because umwl is disabled and href provided", f.MatchIndex), false)
		return
	}
	if f.HostnameIndex != 99999 {
		f.MatchIndex = f.HostnameIndex
		utils.LogInfo(fmt.Sprintf("match column set to hostname column (%d) because href column is not provided", f.MatchIndex), false)
		return
	}
	if f.NameIndex != 99999 {
		f.MatchIndex = f.NameIndex
		utils.LogInfo(fmt.Sprintf("match column set to name column (%d) because href and hostname column is not provided", f.MatchIndex), false)
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

	// App
	fieldMapping["app"] = "app"
	fieldMapping["app label"] = "app"
	fieldMapping["app_label"] = "app"
	fieldMapping["applabel"] = "app"
	fieldMapping["application"] = "app"
	fieldMapping["application label"] = "app"
	fieldMapping["application_label"] = "app"
	fieldMapping["applicationlabel"] = "app"

	// Env
	fieldMapping["env"] = "env"
	fieldMapping["env label"] = "env"
	fieldMapping["env_label"] = "env"
	fieldMapping["envlabel"] = "env"
	fieldMapping["environment"] = "env"
	fieldMapping["environment label"] = "env"
	fieldMapping["environment"] = "env"
	fieldMapping["environmentlabel"] = "env"

	// Loc
	fieldMapping["loc"] = "loc"
	fieldMapping["Loc label"] = "loc"
	fieldMapping["loc_label"] = "loc"
	fieldMapping["loclabel"] = "loc"
	fieldMapping["location"] = "loc"
	fieldMapping["location label"] = "loc"
	fieldMapping["location"] = "loc"
	fieldMapping["locationlabel"] = "loc"

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

	return fieldMapping
}

func (f *FromCSVInput) decreaseColBy1() {
	if f.MatchIndex != 0 {
		f.MatchIndex--
	}
	if f.HostnameIndex != 0 {
		f.HostnameIndex--
	}
	if f.HrefIndex != 0 {
		f.HrefIndex--
	}
	if f.NameIndex != 0 {
		f.NameIndex--
	}
	if f.RoleIndex != 0 {
		f.RoleIndex--
	}
	if f.AppIndex != 0 {
		f.AppIndex--
	}
	if f.EnvIndex != 0 {
		f.EnvIndex--
	}
	if f.LocIndex != 0 {
		f.LocIndex--
	}
	if f.IntIndex != 0 {
		f.IntIndex--
	}
	if f.DescIndex != 0 {
		f.DescIndex--
	}
}

func (f *FromCSVInput) log() {

	v := reflect.ValueOf(*f)

	logEntry := []string{}
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Name == "PCE" || v.Type().Field(i).Name == "KeepAllPCEInterfaces" || v.Type().Field(i).Name == "FQDNtoHostname" {
			continue
		}
		logEntry = append(logEntry, fmt.Sprintf("%s: %v", v.Type().Field(i).Name, v.Field(i).Interface()))
	}
	utils.LogInfo(strings.Join(logEntry, "; "), false)
}
