package ruleimport

import (
	"fmt"

	"github.com/brian1917/workloader/cmd/ruleexport"
	"github.com/brian1917/workloader/utils"
)

func (i *Input) processHeaders(headers []string) {

	i.Headers = make(map[string]int)

	for e, h := range headers {
		i.Headers[h] = e
	}

	// Convert the first row into a map
	headerMap := make(map[string]int)
	for i, header := range headers {
		headerMap[header] = i
	}

	// Check for required headers
	requiredHeaders := []string{
		ruleexport.HeaderServices,
		ruleexport.HeaderUnscopedConsumers,
		ruleexport.HeaderRulesetName,
		ruleexport.HeaderRuleEnabled,
		ruleexport.HeaderProviderResolveLabelsAs,
		ruleexport.HeaderConsumerResolveLabelsAs}

	for _, rh := range requiredHeaders {
		if _, ok := i.Headers[rh]; !ok {
			utils.LogError(fmt.Sprintf("No header for found for required field: %s", rh))
		}
	}
}
