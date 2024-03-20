package ruleimport

import (
	"fmt"
	"strings"

	"github.com/brian1917/workloader/cmd/ruleexport"
	"github.com/brian1917/workloader/utils"
)

func (i *Input) processHeaders(headers []string) {

	i.Headers = make(map[string]int)

	headerWarningGiven := false
	for e, h := range headers {
		if strings.Contains(h, "consumer_") {
			if !headerWarningGiven {
				utils.LogWarning("deprecation - headers are using legacy terminology of consumer and provider. switch to src and dst. see help menu for accceptable headers. processing will continue.", true)
				headerWarningGiven = true
			}
			h = strings.Replace(h, "consumer_", "src_", -1)
		}
		if strings.Contains(h, "provider_") {
			if !headerWarningGiven {
				utils.LogWarning("deprecation - headers are using legacy terminology of consumer and provider. switch to src and dst. see help menu for accceptable headers. processing will continue.", true)
				headerWarningGiven = true
			}
			h = strings.Replace(h, "provider_", "dst_", -1)
		}
		i.Headers[h] = e
	}

	// Convert the first row into a map
	headerMap := make(map[string]int)
	for i, header := range headers {
		if strings.Contains(header, "consumer_") {
			if !headerWarningGiven {
				utils.LogWarning("deprecation - headers are using legacy terminology of consumer and provider. switch to src and dst. see help menu for accceptable headers. processing will continue.", true)
				headerWarningGiven = true
			}
			header = strings.Replace(header, "consumer_", "src_", -1)
		}
		if strings.Contains(header, "provider_") {
			if !headerWarningGiven {
				utils.LogWarning("deprecation - headers are using legacy terminology of consumer and provider. switch to src and dst. see help menu for accceptable headers. processing will continue.", true)
				headerWarningGiven = true
			}
			header = strings.Replace(header, "provider_", "dst_", -1)
		}
		headerMap[header] = i
	}

	// Check for required headers
	requiredHeaders := []string{
		ruleexport.HeaderServices,
		ruleexport.HeaderUnscopedConsumers,
		ruleexport.HeaderRulesetName,
		ruleexport.HeaderRuleEnabled,
		ruleexport.HeaderDstResolveLabelsAs,
		ruleexport.HeaderSrcResolveLabelsAs}

	for _, rh := range requiredHeaders {
		if _, ok := i.Headers[rh]; !ok {
			utils.LogError(fmt.Sprintf("No header for found for required field: %s", rh))
		}
	}
}
