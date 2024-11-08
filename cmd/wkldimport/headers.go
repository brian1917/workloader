package wkldimport

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/brian1917/workloader/cmd/wkldexport"

	"github.com/brian1917/workloader/utils"
)

func (i *Input) ProcessHeaders(headers []string) {

	// Convert the first row into a map
	csvHeaderMap := make(map[string]int)
	for i, header := range headers {
		csvHeaderMap[header] = i
	}

	// Get the fieldMap
	fieldMap := wkldexport.FieldMapping()

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
