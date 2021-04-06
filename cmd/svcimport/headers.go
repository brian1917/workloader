package svcimport

import (
	"strings"

	"github.com/brian1917/workloader/cmd/svcexport"
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

	// Update the header map
	for header, col := range csvHeaderMap {
		i.Headers[fieldMap[header]] = col
	}

}

// fieldMapping returns a map with key csv header and value the expected header.
// By default the values are the same but the method allows for providing alternative CSV headers that map to the same expected header.
func fieldMapping() map[string]string {

	// Get all the headers
	importHeaders := svcexport.ImportHeaders()

	// Check for the existing of the headers
	fieldMapping := make(map[string]string)

	// Assign defaults
	for _, h := range importHeaders {
		fieldMapping[h] = h
	}

	return fieldMapping
}
