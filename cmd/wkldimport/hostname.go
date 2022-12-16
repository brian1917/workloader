package wkldimport

import (
	"fmt"

	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/utils"
)

func (w *importWkld) hostname(input Input) {
	if index, ok := input.Headers[wkldexport.HeaderHostname]; ok {
		// It has to either be a new workload or not matching on hostname
		if w.wkld.Href == "" || (input.MatchString != wkldexport.HeaderHostname) {
			if w.wkld.Hostname != w.csvLine[index] {
				if w.wkld.Href != "" && input.UpdateWorkloads {
					w.change = true
					utils.LogInfo(fmt.Sprintf("csv line %d - %s - hostname to be changed from %s to %s", w.csvLineNum, w.compareString, utils.LogBlankValue(w.wkld.Hostname), w.csvLine[index]), false)
				}
				w.wkld.Hostname = w.csvLine[index]
			}
		}
	}
}
