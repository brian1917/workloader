package wkldimport

import (
	"fmt"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
)

// checkLabels validates if a label exists.
// If the label exists it returns the label.
// If the label does not exist it creates a temporary label for later
func checkLabel(pce illumioapi.PCE, label illumioapi.Label, newLabels []illumioapi.Label) (illumioapi.Label, []illumioapi.Label) {

	// Check if it exists or not
	if _, ok := pce.Labels[label.Key+label.Value]; ok {
		return pce.Labels[label.Key+label.Value], newLabels
	}

	// If the label doesn't exist, create a placeholder for it
	label.Href = fmt.Sprintf("wkld-import-temp-%s-%s", label.Key, label.Value)
	newLabels = append(newLabels, label)

	// Append the label back to the map
	pce.Labels[label.Key+label.Value] = label
	pce.Labels[label.Href] = label

	return label, newLabels
}

func (w *importWkld) labels(input Input, newLabels []illumioapi.Label, labelKeysMap map[string]bool) []illumioapi.Label {

	// Create a copy of the workload before editing it
	originalWkld := *w.wkld

	// Initialize a variable to clear the labels only if we are processing them
	labelsCleared := false

	// Put all labels that don't have a header back in
	nonProcessedLabels := []*illumioapi.Label{}
	if w.wkld.Labels != nil {
		for _, l := range *w.wkld.Labels {
			if _, ok := input.Headers[input.PCE.Labels[l.Href].Key]; !ok {
				nonProcessedLabels = append(nonProcessedLabels, l)
			}
		}
	}

	// Process all headers to see if they are labels
	for headerValue, index := range input.Headers {

		// Skip if the header is not a label
		if !labelKeysMap[headerValue] {
			continue
		}

		// If the input has a label header, we clear the original labels.
		if !labelsCleared {
			// Clear the labels
			w.wkld.Labels = &[]*illumioapi.Label{}
			labelsCleared = true
		}

		// Get the current label
		currentLabel := originalWkld.GetLabelByKey(headerValue, input.PCE.Labels)

		// If the value is blank and the current label exists keep the current label.
		// Or, if the CSV entry equals the current value, keep it
		if (w.csvLine[index] == "" && currentLabel.Href != "") || (w.csvLine[index] == currentLabel.Value && w.csvLine[index] != "") {
			*w.wkld.Labels = append(*w.wkld.Labels, &illumioapi.Label{Href: currentLabel.Href})
			continue
		}

		// If the value is the delete value, the value is not blank, and the current label is not already blank, log a change without putting any label in.
		if w.csvLine[index] == input.RemoveValue && w.csvLine[index] != "" && currentLabel.Href != "" {
			// Log if updating
			if w.wkld.Href != "" && input.UpdateWorkloads {
				w.change = true
				utils.LogInfo(fmt.Sprintf("csv line %d - %-s - %s label of %s to be removed.", w.csvLineNum, w.compareString, currentLabel.Key, currentLabel.Value), false)
			}
			// Stop processing this label
			continue
		}

		// If the value does not equal the current value and it does not equal the remove value, add the new label.
		if w.csvLine[index] != currentLabel.Value && w.csvLine[index] != input.RemoveValue {
			// Add that label to the new labels slice]
			var retrievedLabel illumioapi.Label
			retrievedLabel, newLabels = checkLabel(input.PCE, illumioapi.Label{Key: headerValue, Value: w.csvLine[index]}, newLabels)
			*w.wkld.Labels = append(*w.wkld.Labels, &illumioapi.Label{Href: retrievedLabel.Href})

			// Log if updating
			if w.wkld.Href != "" && input.UpdateWorkloads {
				w.change = true
				// Log change required
				currentlLabelLogValue := currentLabel.Value
				if currentLabel.Value == "" {
					currentlLabelLogValue = "<empty>"
				}
				utils.LogInfo(fmt.Sprintf("csv line %d - %s - %s label to be changed from %s to %s.", w.csvLineNum, w.compareString, headerValue, currentlLabelLogValue, w.csvLine[index]), false)
			}
		}
	}
	// Add the unprocessed labels if they were cleared
	if labelsCleared {
		*w.wkld.Labels = append(*w.wkld.Labels, nonProcessedLabels...)
	}

	return newLabels
}
