package cwpimport

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/cwpexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type updateCWP struct {
	cwp     illumioapi.ContainerWorkloadProfile
	csvLine int
}

var importFile, removeValueInput string
var updatePCE, noPrompt bool
var labelsToBeCreated []illumioapi.Label

func init() {
	ContainerProfileImportCmd.Flags().StringVar(&removeValueInput, "remove-value", "workloader-remove", "used for removing existing labels. by default blank cells in the csv are ignored. this unique string in the csv cell is used in place of the existing value to tell workloader to replace the existing value with a blank value.")
}

// WkldExportCmd runs the workload identifier
var ContainerProfileImportCmd = &cobra.Command{
	Use:   "cwp-import [csv file to import]",
	Short: "Update container workload profiles in the PCE.",
	Long: `
Update container workload profiles in the PCE.

It's recommended to start with a cwp-export command to get the proper format and the container workload profile HREFs.

Only label assignments are supported. Label restrictions will show as blank in the export. Adding a value to the blank will change the restriction to an assignment.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		importFile = args[0]

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		// Validate remove value
		if removeValueInput == "" {
			utils.LogError("remove-value cannot be blank")
		}

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		ImportContainerProfiles(pce, importFile, removeValueInput, updatePCE, noPrompt)
	},
}

func checkLabel(pce illumioapi.PCE, label illumioapi.Label) illumioapi.Label {

	// Check if it exists or not
	if _, ok := pce.Labels[label.Key+label.Value]; ok {
		return pce.Labels[label.Key+label.Value]
	}

	// Put a place holder label in there
	newLabel := illumioapi.Label{Key: label.Key, Value: label.Value}
	labelsToBeCreated = append(labelsToBeCreated, newLabel)
	pce.Labels[label.Key+label.Value] = newLabel
	return newLabel
}

func ImportContainerProfiles(pce illumioapi.PCE, importFile, removeValue string, updatePCE, noPrompt bool) {

	// Parse the input file
	csvData, err := utils.ParseCSV(importFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get all container clusters
	a, err := pce.GetContainerClusters(nil)
	utils.LogAPIRespV2("GetContainerClusters", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Iterate each container cluster and get the container profiles
	cwpMap := make(map[string]illumioapi.ContainerWorkloadProfile)
	for _, cc := range pce.ContainerClustersSlice {
		a, err := pce.GetContainerWkldProfiles(nil, cc.ID())
		utils.LogAPIRespV2("GetContainerWkldProfiles", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		for _, p := range pce.ContainerWorkloadProfilesSlice {
			// if p.Name != nil && *p.Name == "Default Profile" {
			p.ClusterName = cc.Name
			cwpMap[p.Href] = p
		}
	}

	// Create a map of our headers and a slice for our updates
	headers := make(map[string]int)
	updatedCWPs := []updateCWP{}

	// Process each csv row
	for index, row := range csvData {
		update := false
		// If it's the first row, process the headers
		if index == 0 {
			for col, header := range row {
				headers[header] = col
			}
		}

		// Process all other rows
		if index != 0 {
			// Get the current pce container workload profile
			if cwp, exists := cwpMap[row[headers[cwpexport.Href]]]; !exists {
				utils.LogWarning(fmt.Sprintf("csv row %d - %s does not exist. skipping.", index+1, row[headers[cwpexport.Href]]), true)
			} else {
				logMsgs := []string{}

				// Name - Blank to a value
				if cwp.Name == nil && row[headers[cwpexport.Name]] != "" && row[headers[cwpexport.Name]] != removeValue {
					logMsgs = append(logMsgs, fmt.Sprintf("blank name value to be changed to %s", row[headers[cwpexport.Name]]))
					cwp.Name = &row[headers[cwpexport.Name]]
					update = true
				}

				// Name - Remove a value
				if cwp.Name != nil && *cwp.Name != "" && row[headers[cwpexport.Name]] == removeValue {
					logMsgs = append(logMsgs, fmt.Sprintf("name of %s to be removed", *cwp.Name))
					cwp.Name = nil
					update = true
				}

				// Name - Update a value
				if cwp.Name != nil && row[headers[cwpexport.Name]] != "" && *cwp.Name != row[headers[cwpexport.Name]] && row[headers[cwpexport.Name]] != removeValue {
					logMsgs = append(logMsgs, fmt.Sprintf("name to be changed from %s to %s", *cwp.Name, row[headers[cwpexport.Name]]))
					cwp.Name = &row[headers[cwpexport.Name]]
					update = true
				}

				// Description - Blank to a value
				if (cwp.Description == nil || *cwp.Description == "") && row[headers[cwpexport.Description]] != "" && row[headers[cwpexport.Description]] != removeValue {
					logMsgs = append(logMsgs, fmt.Sprintf("blank description value to be changed to %s", row[headers[cwpexport.Description]]))
					cwp.Description = &row[headers[cwpexport.Description]]
					update = true
				}

				// Description - Remove a value
				if cwp.Description != nil && *cwp.Description != "" && row[headers[cwpexport.Description]] == removeValue {
					logMsgs = append(logMsgs, fmt.Sprintf("description of %s to be removed", *cwp.Description))
					*cwp.Description = ""
					update = true
				}

				// Description - Update a value
				if cwp.Description != nil && row[headers[cwpexport.Description]] != "" && *cwp.Description != row[headers[cwpexport.Description]] && row[headers[cwpexport.Description]] != removeValue {
					logMsgs = append(logMsgs, fmt.Sprintf("description to be changed from %s to %s", *cwp.Description, row[headers[cwpexport.Description]]))
					cwp.Description = &row[headers[cwpexport.Description]]
					update = true
				}

				// Enforcement
				e := row[headers[cwpexport.Enforcement]]
				if e != "idle" && e != "visibility_only" && e != "full" && e != "selective" {
					utils.LogError(fmt.Sprintf("csv line %d - %s is an invalid enforcement value. acceptable values are idle, visibility_only, or full.", index+1, e))
				}
				if illumioapi.PtrToVal(cwp.EnforcementMode) != e {
					logMsgs = append(logMsgs, fmt.Sprintf("enforcement to be updated from %s to %s", illumioapi.PtrToVal(cwp.EnforcementMode), e))
					cwp.EnforcementMode = &e
					update = true
				}

				// Visibility

				// Get PCE vis level in UI terms
				pceVisLevel := ""
				switch illumioapi.PtrToVal(cwp.VisibilityLevel) {
				case "flow_summary":
					pceVisLevel = "blocked_allowed"
				case "flow_drops":
					pceVisLevel = "blocked"
				case "flow_off":
					pceVisLevel = "off"
				case "enhanced_data_collection":
					pceVisLevel = "enhanced_data_collection"
				}
				csvVisLevel := ""

				// Validate acceptable value
				c := strings.ToLower(row[headers[cwpexport.Visibility]])
				if c != "blocked_allowed" && c != "blocked" && c != "off" && c != "enhanced_data_collection" {
					utils.LogError(fmt.Sprintf("csv line %d - %s is an invalid visibility value. acceptable values are blocked_allowed, blocked, off, or enhanced_data_collection.", index+1, c))
				}

				// Put the CSV value into API terms
				switch c {
				case "blocked_allowed":
					csvVisLevel = "flow_summary"
				case "blocked":
					csvVisLevel = "flow_drops"
				case "off":
					csvVisLevel = "flow_off"
				case "enhanced_data_collection":
					csvVisLevel = "enhanced_data_collection"
				}

				// Compare the converted PCE level to the provided CSV level
				if pceVisLevel != c {
					logMsgs = append(logMsgs, fmt.Sprintf("visibility to be updated from %s to %s", pceVisLevel, c))
					cwp.VisibilityLevel = &csvVisLevel
					update = true
				}

				// Managed
				csvManaged, err := strconv.ParseBool(row[headers[cwpexport.Managed]])
				if err != nil {
					utils.LogError(fmt.Sprintf("csv row %d - %s is an invalid managed boolean value", index+1, row[headers[cwpexport.Managed]]))
				}
				if *cwp.Managed != csvManaged {
					logMsgs = append(logMsgs, fmt.Sprintf("managed to be updated from %t to %t", *cwp.Managed, csvManaged))
					cwp.Managed = &csvManaged
					update = true
				}

				// Labels
				keys := []string{"role", "app", "env", "loc"}
				values := []string{row[headers[cwpexport.Role]], row[headers[cwpexport.App]], row[headers[cwpexport.Env]], row[headers[cwpexport.Loc]]}
				for i, key := range keys {
					// If the value is blank, skip it
					if values[i] == "" {
						continue
					} else if values[0] == removeValue && values[1] == removeValue && values[2] == removeValue && values[3] == removeValue {
						update = true
						cwp.Labels = &[]illumioapi.Label{}
						logMsgs = append(logMsgs, "all labels to be removed")
						break
					} else if values[i] == removeValue {
						logMsgs = append(logMsgs, fmt.Sprintf("%s label %s to be removed", key, cwp.GetLabelByKey(key)))
						cwp.RemoveLabel(key)
						update = true
						// Process everything else
					} else {
						// Get the label
						csvLabel := checkLabel(pce, illumioapi.Label{Key: key, Value: values[i]})
						// If there is no HREF, log that the label needs to be created
						if csvLabel.Href == "" {
							logMsgs = append(logMsgs, fmt.Sprintf("%s label %s to be created", csvLabel.Key, csvLabel.Value))
						}
						if cwp.GetLabelByKey(key) != csvLabel.Value {
							logMsgs = append(logMsgs, fmt.Sprintf("%s label to be updated from %s to %s", key, cwp.GetLabelByKey(key), csvLabel.Value))
							cwp.SetLabel(csvLabel, &pce)
							update = true
						}
					}
				}

				// Log Message
				if update {
					utils.LogInfo(fmt.Sprintf("csv line %d - %s", index+1, strings.Join(logMsgs, "; ")), true)
					updatedCWPs = append(updatedCWPs, updateCWP{cwp: cwp, csvLine: index + 1})
				}
			}
		}
	}

	// Process updates
	if len(updatedCWPs) == 0 {
		utils.LogInfo("nothing to be done", true)

		return
	}

	// Log findings
	utils.LogInfo(fmt.Sprintf("%d labels to create and %d container workload profiles to update.", len(labelsToBeCreated), len(updatedCWPs)), true)

	// Stop if not updating pce
	if !updatePCE {
		utils.LogInfo("see workloader.log for more details. to do the import, run again using the --update-pce flag.", true)

		return
	}

	// Prompt
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("\r\n%s [PROMPT] - do you want to run the import to %s (%s) (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied", true)

			return
		}
	}

	// Prompt accepted or --no-prompt used - first, create labels.
	for _, label := range labelsToBeCreated {
		newLabel, api, err := pce.CreateLabel(label)
		utils.LogAPIRespV2("CreateLabel", api)
		if err != nil {
			utils.LogError(err.Error())
		}
		pce.Labels[newLabel.Key+newLabel.Value] = newLabel
		utils.LogInfo(fmt.Sprintf("created %s %s label - %d", newLabel.Value, newLabel.Key, api.StatusCode), true)
	}

	// Update CWPs that have place holder labels
	for i, update := range updatedCWPs {
		for _, label := range *update.cwp.Labels {
			if label.Assignment.Value != "" && label.Assignment.Href == "" {
				update.cwp.SetLabel(pce.Labels[label.Key+label.Assignment.Value], &pce)
			}
		}
		updatedCWPs[i] = updateCWP{cwp: update.cwp, csvLine: update.csvLine}
	}

	// Update the CWPs
	for _, update := range updatedCWPs {
		api, err := pce.UpdateContainerWkldProfiles(update.cwp)
		utils.LogAPIRespV2("UpdateContainerWorkloadProfiles", api)
		if err != nil {
			utils.LogError(fmt.Sprintf("csv line %d - %s", update.csvLine, err.Error()))
		}
		utils.LogInfo(fmt.Sprintf("csv line %d - updated %s - %d", update.csvLine, update.cwp.Href, api.StatusCode), true)
	}

}
