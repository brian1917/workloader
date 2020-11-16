package labelgroupimport

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/brian1917/workloader/utils"
)

type csvParser struct {
	NameIndex, DescriptionIndex, HrefIndex, KeyIndex, MemberLabelsIndex, MemberSGsIndex int
}

func (c *csvParser) processHeaders(headers []string) {

	// Convert the first row into a map
	headerMap := make(map[string]int)
	for i, header := range headers {
		headerMap[header] = i
	}

	// Get the fieldMap
	fieldMap := fieldMapping()

	// Set the initial values
	// Using 99999 because 0 (default) could mean first column and we need to differentiate between provided and not
	c.NameIndex, c.DescriptionIndex, c.HrefIndex, c.KeyIndex, c.MemberLabelsIndex, c.MemberSGsIndex = 99999, 99999, 99999, 99999, 99999, 99999

	// Create our from CSV input
	for header, col := range headerMap {
		switch fieldMap[header] {
		case "name":
			if c.NameIndex == 99999 {
				c.NameIndex = col
			}
		case "href":
			if c.HrefIndex == 99999 {
				c.HrefIndex = col
			}
		case "key":
			if c.KeyIndex == 99999 {
				c.KeyIndex = col
			}
		case "description":
			if c.DescriptionIndex == 99999 {
				c.DescriptionIndex = col
			}
		case "member_labels":
			if c.MemberLabelsIndex == 99999 {
				c.MemberLabelsIndex = col
			}
		case "member_label_groups":
			if c.MemberSGsIndex == 99999 {
				c.MemberSGsIndex = col
			}
		}
	}
}

func fieldMapping() map[string]string {
	// Check for the existing of the headers
	fieldMapping := make(map[string]string)

	// Hostname
	fieldMapping["href"] = "href"

	// Name
	fieldMapping["name"] = "name"

	// Key
	fieldMapping["key"] = "key"

	// MemberLabels
	fieldMapping["member_labels"] = "member_labels"
	fieldMapping["memberlabels"] = "member_labels"
	fieldMapping["member labels"] = "member_labels"
	fieldMapping["labels"] = "member_labels"

	// MemberLabelGroups
	fieldMapping["member_label_groups"] = "member_label_groups"
	fieldMapping["member_label_groups"] = "member_label_values"
	fieldMapping["member label groups"] = "member_label_groups"
	fieldMapping["member_label_groups"] = "member_label_groups"
	fieldMapping["label_groups"] = "member_label_groups"
	fieldMapping["label groups"] = "member_label_groups"

	// Description
	fieldMapping["description"] = "description"
	fieldMapping["desc"] = "description"

	return fieldMapping
}

func (c *csvParser) log() {

	v := reflect.ValueOf(*c)

	logEntry := []string{}
	for i := 0; i < v.NumField(); i++ {
		if v.Type().Field(i).Name == "PCE" || v.Type().Field(i).Name == "KeepAllPCEInterfaces" || v.Type().Field(i).Name == "FQDNtoHostname" {
			continue
		}
		logEntry = append(logEntry, fmt.Sprintf("%s: %v", v.Type().Field(i).Name, v.Field(i).Interface()))
	}
	utils.LogInfo(strings.Join(logEntry, "; "), false)
}
