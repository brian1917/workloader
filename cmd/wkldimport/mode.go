package wkldimport

import (
	"fmt"
	"strings"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/utils"
)

func (w *importWkld) enforcement(input Input) {
	if input.AllowEnforcementChanges {
		// Update the enforcement
		if index, ok := input.Headers[wkldexport.HeaderEnforcement]; ok && strings.ToLower(w.csvLine[index]) != "unmanaged" && w.csvLine[index] != "" {
			m := strings.ToLower(w.csvLine[index])
			if m != "visibility_only" && m != "full" && m != "selective" && m != "idle" && m != "" {
				utils.LogWarning(fmt.Sprintf("csv line %d - %s - invalid mode state. values must be blank, visibility_only, full, selective, or idle. skipping line.", w.csvLineNum, w.compareString), true)
				return
			}
			if illumioapi.PtrToVal(w.wkld.EnforcementMode) != m {
				if w.wkld.Href != "" && input.UpdateWorkloads {
					w.change = true
					utils.LogInfo(fmt.Sprintf("csv line %d - %s enforcement to be changed from %s to %s", w.csvLineNum, w.compareString, illumioapi.PtrToVal(w.wkld.EnforcementMode), w.csvLine[index]), false)
				}
				w.wkld.EnforcementMode = &m
			}
		}
	}
}

func (w *importWkld) visibility(input Input) {
	if input.AllowEnforcementChanges {
		if index, ok := input.Headers[wkldexport.HeaderVisibility]; ok && strings.ToLower(w.csvLine[index]) != "unmanaged" && w.csvLine[index] != "" {
			v := strings.ToLower(w.csvLine[index])
			if v != "blocked_allowed" && v != "blocked" && v != "off" && v != "" {
				utils.LogWarning(fmt.Sprintf("csv line %d - %s - invalid visibility state. values must be blank, blocked_allowed, blocked, or off. skipping line.", w.csvLineNum, w.compareString), true)
				return
			}
			if w.wkld.GetVisibilityLevel() != v {
				if w.wkld.Href != "" && input.UpdateWorkloads {
					w.change = true
					utils.LogInfo(fmt.Sprintf("csv line %d - %s visibility to be changed from %s to %s", w.csvLineNum, w.compareString, illumioapi.PtrToVal(w.wkld.VisibilityLevel), w.csvLine[index]), false)
				}
				w.wkld.SetVisibilityLevel(v)
			}
		}
	}
}
