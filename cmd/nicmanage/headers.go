package nicmanage

import (
	"fmt"
	"strings"

	"github.com/brian1917/workloader/utils"
)

type headers struct {
	wkldHref      int
	interfaceName int
	ignored       int
}

func findHeaders(headerRow []string) headers {
	fieldMaps := fieldMapping()
	headers := headers{}
	ok := 0

	for i, h := range headerRow {
		if fieldMaps[strings.ToLower(h)] == "wkld_href" {
			headers.wkldHref = i
			ok++
		}
		if fieldMaps[strings.ToLower(h)] == "interface_name" {
			headers.interfaceName = i
			ok++
		}
		if fieldMaps[strings.ToLower(h)] == "ignored" {
			headers.ignored = i
			ok++
		}
	}

	if ok != 3 {
		utils.LogError("input requires a header row with three values - wkld_href, ignored, and nic_name")
	}

	utils.LogInfo(fmt.Sprintf("wkld_href index: %d; ignored index: %d", headers.wkldHref, headers.ignored), false)

	return headers

}

func fieldMapping() map[string]string {
	// Check for the existing of the headers
	fieldMapping := make(map[string]string)

	// Workload HREF
	fieldMapping["wkld_href"] = "wkld_href"
	fieldMapping["wkld href"] = "wkld_href"
	fieldMapping["workloader_href"] = "wkld_href"
	fieldMapping["href"] = "wkld_href"

	// Interface name
	fieldMapping["interface_name"] = "interface_name"
	fieldMapping["interface name"] = "interface_name"
	fieldMapping["int_name"] = "interface_name"
	fieldMapping["int name"] = "interface_name"
	fieldMapping["nic name"] = "interface_name"
	fieldMapping["nic_name"] = "interface_name"

	// Ignored
	fieldMapping["ignored"] = "ignored"
	fieldMapping["ignore"] = "ignored"

	return fieldMapping
}
