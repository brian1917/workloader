package svcimport

import (
	"strings"
)

func (i *Input) processHeaders(headers []string) {

	// Convert the first row into a map
	csvHeaderMap := make(map[string]int)
	for i, header := range headers {
		csvHeaderMap[strings.ToLower(header)] = i
	}

	// Initiate the map
	i.Headers = make(map[string]int)

	// Update the header map
	for header, col := range csvHeaderMap {
		i.Headers[header] = col
	}

}
