package wkldimport

import (
	"fmt"

	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/utils"
)

func (w *importWkld) publcIP(input Input) {
	if index, ok := input.Headers[wkldexport.HeaderPublicIP]; ok {
		if w.csvLine[index] != w.wkld.PublicIP {
			// Validate it first
			if !publicIPIsValid(w.csvLine[index]) {
				utils.LogError(fmt.Sprintf("csv line %d - invalid Public IP address format.", w.csvLineNum))
			}
			if w.wkld.Href != "" && input.UpdateWorkloads {
				w.change = true
				utils.LogInfo(fmt.Sprintf("csv line %d - Public IP to be changed from %s to %s", w.csvLineNum, w.wkld.PublicIP, w.csvLine[index]), false)
			}
			w.wkld.PublicIP = w.csvLine[index]
		}
	}
}
