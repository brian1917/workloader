package wkldimport

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/brian1917/workloader/cmd/wkldexport"

	"github.com/brian1917/workloader/utils"
)

func (i *Input) processHeaders(headers []string) {

	// Convert the first row into a map
	csvHeaderMap := make(map[string]int)
	for i, header := range headers {
		csvHeaderMap[strings.ToLower(header)] = i
	}

	// Get the fieldMap
	fieldMap := fieldMapping()

	// Initiate the map
	i.Headers = make(map[string]int)

	// Look to see if we have alternatve values for the provided header
	for header, col := range csvHeaderMap {
		if _, ok := fieldMap[header]; ok {
			i.Headers[fieldMap[header]] = col
		} else {
			// If there is no alternative value, use the provided value
			i.Headers[header] = col
		}

	}

	if i.MatchString != "" {
		if i.MatchString != "href" && i.MatchString != "hostname" && i.MatchString != "name" && i.MatchString != "external_data" {
			utils.LogError("invalid match value. must be href, hostname, name, or external_data")
		}
		return
	}

	// If href is provided and UMWL is not set, use href
	if val, ok := i.Headers[wkldexport.HeaderHref]; ok && !i.Umwl {
		i.MatchString = wkldexport.HeaderHref
		utils.LogInfo(fmt.Sprintf("match column set to %d because href header is present and unmanaged workload flag is not set.", val), false)
		return
	}

	// If hostname is set, use that.
	if val, ok := i.Headers[wkldexport.HeaderHostname]; ok {
		i.MatchString = wkldexport.HeaderHostname
		utils.LogInfo(fmt.Sprintf("match column set to hostname column (%d)", val), false)
		return
	}

	// If name is set, use that.
	if val, ok := i.Headers[wkldexport.HeaderName]; ok {
		i.MatchString = wkldexport.HeaderName
		utils.LogInfo(fmt.Sprintf("match column set to name column (%d)", val), false)
		return
	}

	utils.LogError("cannot set a match column based on provided input")
}

func fieldMapping() map[string]string {

	// Get all the headers
	allHeaders := wkldexport.AllHeaders(true, true)

	// Check for the existing of the headers
	fieldMapping := make(map[string]string)

	// Assign defaults
	for _, h := range allHeaders {
		fieldMapping[h] = h
	}

	// Alternate names for hostname
	fieldMapping["host"] = "hostname"
	fieldMapping["host_name"] = "hostname"
	fieldMapping["host name"] = "hostname"

	// Alternate names for role
	fieldMapping["role label"] = "role"
	fieldMapping["role_label"] = "role"
	fieldMapping["rolelabel"] = "role"
	fieldMapping["suggested_role"] = "role" // for traffic command
	fieldMapping["edge_group"] = "role"     // for edge

	// Alternate names for app
	fieldMapping["app label"] = "app"
	fieldMapping["app_label"] = "app"
	fieldMapping["applabel"] = "app"
	fieldMapping["application"] = "app"
	fieldMapping["application label"] = "app"
	fieldMapping["application_label"] = "app"
	fieldMapping["applicationlabel"] = "app"
	fieldMapping["suggested_app"] = "app" // for traffic command

	// Alternate names for env
	fieldMapping["env label"] = "env"
	fieldMapping["env_label"] = "env"
	fieldMapping["envlabel"] = "env"
	fieldMapping["environment"] = "env"
	fieldMapping["environment label"] = "env"
	fieldMapping["environment"] = "env"
	fieldMapping["environmentlabel"] = "env"
	fieldMapping["suggested_env"] = "env" // for traffic command

	// Alternate names for loc
	fieldMapping["Loc label"] = "loc"
	fieldMapping["loc_label"] = "loc"
	fieldMapping["loclabel"] = "loc"
	fieldMapping["location"] = "loc"
	fieldMapping["location label"] = "loc"
	fieldMapping["location"] = "loc"
	fieldMapping["locationlabel"] = "loc"
	fieldMapping["suggested_loc"] = "env" // for traffic command

	// Alternate names for interfaces
	fieldMapping["interface"] = "interfaces"
	fieldMapping["ifaces"] = "interfaces"
	fieldMapping["iface"] = "interfaces"
	fieldMapping["ip"] = "interfaces"
	fieldMapping["ip_address"] = "interfaces"
	fieldMapping["ips"] = "interfaces"

	// Description
	fieldMapping["desc"] = "description"

	return fieldMapping
}

func (i *Input) log() {

	v := reflect.ValueOf(*i)

	logEntry := []string{}
	for a := 0; a < v.NumField(); a++ {
		if v.Type().Field(a).Name == "PCE" || v.Type().Field(a).Name == "KeepAllPCEInterfaces" || v.Type().Field(a).Name == "FQDNtoHostname" {
			continue
		}
		logEntry = append(logEntry, fmt.Sprintf("%s: %v", v.Type().Field(a).Name, v.Field(a).Interface()))
	}

	utils.LogInfo(strings.Join(logEntry, "; "), false)
}
