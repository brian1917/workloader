package wkldimport

import (
	"fmt"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/utils"
)

func (w *importWkld) name(input Input) {
	if index, ok := input.Headers[wkldexport.HeaderName]; ok {
		// It has to either be a new workload or not matching on name
		if illumioapi.PtrToVal(w.wkld.Name) == "" || (input.MatchString != wkldexport.HeaderName) {
			if illumioapi.PtrToVal(w.wkld.Name) != w.csvLine[index] {
				if w.wkld.Href != "" && input.UpdateWorkloads {
					w.change = true
					utils.LogInfo(fmt.Sprintf("csv line %d - %s - name to be changed from %s to %s", w.csvLineNum, w.compareString, utils.LogBlankValue(illumioapi.PtrToVal(w.wkld.Name)), w.csvLine[index]), false)
				}
				w.wkld.Name = &w.csvLine[index]
			}
		}
	}
}
