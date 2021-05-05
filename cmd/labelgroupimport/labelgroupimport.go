package labelgroupimport

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Global variables
var csvFile string
var provision, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

// Struct for entries
type entry struct {
	csvLine    int
	labelGroup illumioapi.LabelGroup
}

func init() {
	LabelGroupImportCmd.Flags().BoolVarP(&provision, "provision", "p", false, "Provision changes.")
	LabelGroupImportCmd.Flags().SortFlags = false
}

// LabelGroupImportCmd runs the upload command
var LabelGroupImportCmd = &cobra.Command{
	Use:   "labelgroup-import [csv file to import]",
	Short: "Create and modify label groups from a CSV file.",
	Long: `
Create and modify label groups from a CSV file.

The input file requires headers and matches fields to header values. The following headers can be used:
- name
- href
- description
- key
- member_label_values
- member_label_groups

It's recommended to start with the output from labelgroup-export command and use that as provided input.

If an href is provided, the label group will be modified. If no href is provided, the label group will be created.

Other columns are alloewd but will be ignored.

Member label values and member label groups should be separated by a semi-colon.

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		labelGroupImport()
	},
}

func labelGroupImport() {
	// Log start of command
	utils.LogStartCommand("labelgroup-import")

	// Parse the CSV
	csvData, err := utils.ParseCSV(csvFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get all the labelgroups
	pceLGMap := make(map[string]illumioapi.LabelGroup)
	pceLGKeyNameMap := make(map[string]illumioapi.LabelGroup)
	labelGroups, a, err := pce.GetAllLabelGroups("draft")
	utils.LogAPIResp("GetAllLabelGroups", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	for _, lg := range labelGroups {
		pceLGMap[lg.Name] = lg
		pceLGMap[lg.Href] = lg
		pceLGKeyNameMap[lg.Key+lg.Name] = lg
	}

	// Create the csvParse
	c := csvParser{}

	// Parse the headers
	c.processHeaders(csvData[0])
	c.log()

	// Start slices to hold the results
	newLabelGroups := []entry{}
	updatedLabelGroups := []entry{}

	// Process each row of the CSV
CSVEntries:
	for i, line := range csvData {

		// Skip the header
		if i == 0 {
			continue
		}

		// If the href header is not present or the value is blank, it's created
		if line[c.HrefIndex] == "" || c.HrefIndex == 99999 {
			newLG := illumioapi.LabelGroup{}

			// Name
			if line[c.NameIndex] == "" {
				utils.LogWarning(fmt.Sprintf("CSV Line %d - name field cannot be blank for new label group. Skipping entry", i+1), true)
				continue CSVEntries
			} else {
				newLG.Name = line[c.NameIndex]
			}

			// Description
			newLG.Description = line[c.DescriptionIndex]

			// Key
			if line[c.KeyIndex] == "" || c.KeyIndex == 99999 {
				utils.LogWarning(fmt.Sprintf("CSV Line %d - key field cannot be blank for new label group. Skipping entry", i+1), true)
				continue CSVEntries
			}
			if strings.ToLower(line[c.KeyIndex]) != "role" && strings.ToLower(line[c.KeyIndex]) != "app" && strings.ToLower(line[c.KeyIndex]) != "loc" && strings.ToLower(line[c.KeyIndex]) != "env" {
				utils.LogWarning(fmt.Sprintf("CSV Line %d - key field must be either role, app, env, or loc", i+1), true)
			}
			newLG.Key = strings.ToLower(line[c.KeyIndex])

			// Member Labels
			labels := strings.Split(strings.Replace(line[c.MemberLabelsIndex], "; ", ";", -1), ";")
			if line[c.MemberLabelsIndex] == "" {
				labels = []string{}
			}
			for _, l := range labels {
				if pceLabel, check := pce.Labels[line[c.KeyIndex]+l]; !check {
					utils.LogWarning(fmt.Sprintf("CSV Line %d - the label %s (%s) does not exist. Skipping entry.", i+1, l, line[c.KeyIndex]), true)
					continue CSVEntries
				} else {
					newLG.Labels = append(newLG.Labels, &illumioapi.Label{Href: pceLabel.Href})
				}
			}

			// Member Label Groups
			labelGroups := strings.Split(strings.Replace(line[c.MemberSGsIndex], "; ", ";", -1), ";")
			if line[c.MemberSGsIndex] == "" {
				labelGroups = []string{}
			}

			for _, lg := range labelGroups {
				if pceLabelGroup, check := pceLGKeyNameMap[line[c.KeyIndex]+lg]; !check {
					utils.LogWarning(fmt.Sprintf("CSV Line %d - the label group %s (%s) does not exist. Skipping entry.", i+1, lg, line[c.KeyIndex]), true)
					continue CSVEntries
				} else {
					newLG.SubGroups = append(newLG.SubGroups, &illumioapi.SubGroups{Href: pceLabelGroup.Href})
				}
			}

			// Add to the new labelgroup slice
			newLabelGroups = append(newLabelGroups, entry{csvLine: i + 1, labelGroup: newLG})
			utils.LogInfo(fmt.Sprintf("CSV Line %d - %s - will be created.", i+1, line[c.NameIndex]), false)

		} else {
			// The label group HREF is provided, first, make sure it's valid.
			var pceLabelGroup illumioapi.LabelGroup
			var check bool
			if pceLabelGroup, check = pceLGMap[line[c.HrefIndex]]; !check {
				utils.LogWarning(fmt.Sprintf("CSV Line %d - href is provided but it does not exist in the PCE. Skipping entry.", i+1), true)
				continue CSVEntries
			}

			// Set update to false
			update := false

			// Name
			if line[c.NameIndex] != pceLabelGroup.Name {
				update = true
				utils.LogInfo(fmt.Sprintf("CSV Line %d - the name will change from %s to %s.", i+1, pceLabelGroup.Name, line[c.NameIndex]), false)
				pceLabelGroup.Name = line[c.NameIndex]
			}

			// Description
			if c.DescriptionIndex != 99999 && line[c.DescriptionIndex] != pceLabelGroup.Description {
				update = true
				utils.LogInfo(fmt.Sprintf("CSV Line %d - %s - the description will be updated.", i+1, line[c.NameIndex]), false)
				pceLabelGroup.Description = line[c.DescriptionIndex]
			}

			// Key
			if line[c.KeyIndex] != "" && line[c.KeyIndex] != pceLabelGroup.Key {
				utils.LogWarning(fmt.Sprintf("CSV Line %d - %s - the key cannot be changed for an existing label group. Skipping entry.", i+1, line[c.NameIndex]), true)
				continue CSVEntries
			}

			// Member labels
			// Create maps for the labels in the PCE and the labels in the CSV entry
			// Set the label update to false
			labelUpdate := false
			pceLabels := make(map[string]bool)
			csvLabels := make(map[string]bool)
			if c.MemberLabelsIndex != 99999 {
				for _, l := range pceLabelGroup.Labels {
					pceLabels[pce.Labels[l.Href].Value] = true
				}
				labels := []string{}
				if line[c.MemberLabelsIndex] != "" {
					labels = strings.Split(strings.Replace(line[c.MemberLabelsIndex], "; ", ";", -1), ";")
				}
				for _, l := range labels {
					csvLabels[l] = true
				}
				// Check if CSV labels are in the PCE
				for l := range csvLabels {
					if !pceLabels[l] {
						// Check if the label exists
						if _, check := pce.Labels[line[c.KeyIndex]+l]; !check {
							utils.LogWarning(fmt.Sprintf("CSV Line %d - %s - %s(%s) does not exist in the PCE as a label. Skipping entry.", i+1, line[c.NameIndex], l, line[c.KeyIndex]), true)
							continue CSVEntries
						}
						labelUpdate = true
						utils.LogInfo(fmt.Sprintf("CSV Line %d - %s - %s label is in the CSV but not in the PCE. It will be added.", i+1, line[c.NameIndex], l), false)
					}
				}
				// Check if PCE labels are in the CSV
				for l := range pceLabels {
					if !csvLabels[l] {
						labelUpdate = true
						utils.LogInfo(fmt.Sprintf("CSV Line %d - %s - %s label is not in the CSV but is in the PCE. It will be removed.", i+1, line[c.NameIndex], l), false)
					}
				}
			}
			// If we are updating the labels, replace with newLabels
			var newLabels []*illumioapi.Label
			if labelUpdate {
				update = true
				for l := range csvLabels {
					newLabels = append(newLabels, &illumioapi.Label{Href: pce.Labels[line[c.KeyIndex]+l].Href})
				}
				pceLabelGroup.Labels = newLabels
			} else {
				for _, l := range pceLabelGroup.Labels {
					newLabels = append(newLabels, &illumioapi.Label{Href: l.Href})
				}
				pceLabelGroup.Labels = newLabels
			}

			// Member Sub Groups
			// Create maps for the subgroups in the PCE and the labels in the CSV entry and set sgUpdate to false
			sgUpdate := false
			pceSGs := make(map[string]bool)
			csvSGs := make(map[string]bool)
			if c.MemberSGsIndex != 99999 {
				for _, sg := range pceLabelGroup.SubGroups {
					pceSGs[pceLGMap[sg.Href].Name] = true
				}
				subGroups := []string{}
				if line[c.MemberSGsIndex] != "" {
					subGroups = strings.Split(strings.Replace(line[c.MemberSGsIndex], "; ", ";", -1), ";")
				}
				for _, sg := range subGroups {
					csvSGs[sg] = true
				}

				// Check if CSV labels are in the PCE
				for sg := range csvSGs {
					if !pceSGs[sg] {
						// Check if the label exists
						if _, check := pceLGKeyNameMap[line[c.KeyIndex]+sg]; !check {
							utils.LogWarning(fmt.Sprintf("CSV Line %d - %s - %s(%s) does not exist in the PCE as a label. Skipping entry.", i+1, line[c.NameIndex], sg, line[c.KeyIndex]), true)
							continue CSVEntries
						}
						sgUpdate = true
						utils.LogInfo(fmt.Sprintf("CSV Line %d - %s subgroup is in the CSV but not in the PCE. It will be added.", i+1, sg), false)
					}
				}
				// Check if PCE labels are in the CSV
				for sg := range pceSGs {
					if !csvSGs[sg] {
						sgUpdate = true
						utils.LogInfo(fmt.Sprintf("CSV Line %d - %s subgroup is not in the CSV but is in the PCE. It will be removed.", i+1, sg), false)
					}
				}
			}
			// If we are updating the sub groups, replace with newSubGroups
			var newSubGroups []*illumioapi.SubGroups
			if sgUpdate {
				update = true
				for sg := range csvSGs {
					newSubGroups = append(newSubGroups, &illumioapi.SubGroups{Href: pceLGMap[sg].Href})
				}
				pceLabelGroup.SubGroups = newSubGroups
			} else {
				for _, sg := range pceLabelGroup.SubGroups {
					newSubGroups = append(newSubGroups, &illumioapi.SubGroups{Href: sg.Href})
				}
				pceLabelGroup.SubGroups = newSubGroups
			}

			// If update is set to true, add it to the slice
			if update {
				updatedLabelGroups = append(updatedLabelGroups, entry{csvLine: i + 1, labelGroup: pceLabelGroup})
			}
		}
	}

	// End run if we have nothing to do
	if len(newLabelGroups) == 0 && len(updatedLabelGroups) == 0 {
		utils.LogInfo("nothing to be done.", true)
		utils.LogEndCommand("labelgroup-import")
		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d label groups to create and %d label groups to update. See workloader.log for all identified changes. To do the import, run again using --update-pce flag", len(newLabelGroups), len(updatedLabelGroups)), true)
		utils.LogEndCommand("labelgroup-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("[PROMPT] - workloader will create %d label groups and update %d label groups in %s (%s). Do you want to run the import (yes/no)? ", len(newLabelGroups), len(updatedLabelGroups), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))

		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("Prompt denied for creating %d label groups and updating %d label groups.", len(newLabelGroups), len(updatedLabelGroups)), true)
			utils.LogEndCommand("labelgroup-import")
			return
		}
	}

	skipped := 0
	createdLGs := 0
	updatedLGs := 0
	provisionableLGs := []string{}
	// Create Label Groups
	for _, newLG := range newLabelGroups {
		lg, a, err := pce.CreateLabelGroup(newLG.labelGroup)
		utils.LogAPIResp("CreateLabelGroup", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("Ending run - %d label groups created - %d label groups updated.", createdLGs, updatedLGs))
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("CSV Line %d - %s - 406 Not Acceptable - See workloader.log for more details", newLG.csvLine, newLG.labelGroup.Name), true)
			utils.LogWarning(a.RespBody, false)
			skipped++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("CSV Line %d - %s created - status code %d", newLG.csvLine, lg.Name, a.StatusCode), true)
			createdLGs++
			provisionableLGs = append(provisionableLGs, lg.Href)
		}
	}

	// Update Label Groups
	for _, updateLG := range updatedLabelGroups {
		a, err := pce.UpdateLabelGroup(updateLG.labelGroup)
		utils.LogAPIResp("UpdateLabelGroup", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("Ending run - %d label groups created - %d label groups updated.", createdLGs, updatedLGs))
			utils.LogError(err.Error())
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("CSV Line %d - %s - 406 Not Acceptable - See workloader.log for more details", updateLG.csvLine, updateLG.labelGroup.Name), true)
			utils.LogWarning(a.RespBody, false)
			skipped++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("CSV Line %d - %s updated - status code %d", updateLG.csvLine, updateLG.labelGroup.Name, a.StatusCode), true)
			updatedLGs++
			provisionableLGs = append(provisionableLGs, updateLG.labelGroup.Href)
		}
	}

	// Provision
	if provision {
		a, err := pce.ProvisionHref(provisionableLGs, "workloader labelgroup-import")
		utils.LogAPIResp("ProvisionHrefs", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("Provisioning successful - status code %d", a.StatusCode), true)
	}

}
