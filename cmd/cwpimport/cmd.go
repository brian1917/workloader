package cwpimport

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/cmd/cwpexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type updateCWP struct {
	cwp     illumioapi.ContainerWorkloadProfile
	csvLine int
}

var importFile, removeValue string
var updatePCE, noPrompt bool
var labelsToBeCreated []illumioapi.Label

func init() {
	ContainerProfileImportCmd.Flags().StringVar(&removeValue, "remove-value", "workloader-remove", "value in csv used to remove existing labels. blank values in the csv will not change existing.")
}

// WkldExportCmd runs the workload identifier
var ContainerProfileImportCmd = &cobra.Command{
	Use:   "cwp-import",
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

		// Get the PCE
		pce, err := utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		importContainerProfiles(pce, importFile, removeValue, updatePCE, noPrompt)
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

func importContainerProfiles(pce illumioapi.PCE, importFile, removeValue string, updatePCE, noPrompt bool) {

	// Log start of command
	utils.LogStartCommand("cwp-import")

	// Parse the input file
	csvData, err := utils.ParseCSV(importFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get all container clusters
	_, a, err := pce.GetContainerClusters(nil)
	utils.LogAPIResp("GetContainerClusters", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Iterate each container cluster and get the container profiles
	cwpMap := make(map[string]illumioapi.ContainerWorkloadProfile)
	for _, cc := range pce.ContainerClustersSlice {
		cp, a, err := pce.GetContainerWkldProfiles(nil, cc.ID())
		utils.LogAPIResp("GetContainerWkldProfiles", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		for _, p := range cp {
			if p.Name == "Default Profile" {
				continue
			}
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

				// Enforcement
				e := row[headers[cwpexport.Enforcement]]
				if e != "idle" && e != "visibility_only" && e != "full" && e != "selective" {
					utils.LogError(fmt.Sprintf("csv line %d - %s is an invalid enforcement value. acceptable values are idle, visibility_only, or full.", index+1, e))
				}
				if cwp.EnforcementMode != e {
					logMsgs = append(logMsgs, fmt.Sprintf("enforcement to be updated from %s to %s", cwp.EnforcementMode, e))
					cwp.EnforcementMode = e
					update = true
				}

				// Visibility

				// Get PCE vis level in UI terms
				pceVisLevel := ""
				switch cwp.VisibilityLevel {
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
					cwp.VisibilityLevel = csvVisLevel
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
						cwp.Labels = &[]illumioapi.ContainerWorkloadProfileLabel{}
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
		utils.LogEndCommand("cwp-import")
		return
	}

	// Log findings
	utils.LogInfo(fmt.Sprintf("%d labels to create and %d container workload profiles to update.", len(labelsToBeCreated), len(updatedCWPs)), true)

	// Stop if not updating pce
	if !updatePCE {
		utils.LogInfo("see workloader.log for more details. to do the import, run again using the --update-pce flag.", true)
		utils.LogEndCommand("cwp-import")
		return
	}

	// Prompt
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("\r\n%s [PROMPT] - do you want to run the import to %s (%s) (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied", true)
			utils.LogEndCommand("cwp-import")
			return
		}
	}

	// Prompt accepted or --no-prompt used - first, create labels.
	for _, label := range labelsToBeCreated {
		newLabel, api, err := pce.CreateLabel(label)
		utils.LogAPIResp("CreateLabel", api)
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
		utils.LogAPIResp("UpdateContainerWorkloadProfiles", api)
		if err != nil {
			utils.LogError(fmt.Sprintf("csv line %d - %s", update.csvLine, err.Error()))
		}
		utils.LogInfo(fmt.Sprintf("csv line %d - updated %s - %d", update.csvLine, update.cwp.Href, api.StatusCode), true)
	}

	utils.LogEndCommand("cwp-import")

}
