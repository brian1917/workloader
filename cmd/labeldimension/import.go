package labeldimension

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var updatePCE, noPrompt bool
var csvFile string

func init() {}

// IplImportCmd runs the iplist import command
var LabelDimensionImportCmd = &cobra.Command{
	Use:   "label-dimension-import [csv file to import]",
	Short: "Create and update label dimensions from a CSV.",
	Long: `
Create and update label dimensions from a CSV file. 

The input should have a header row as the first row will be skipped. The CSV can have columns in any order. The processed headers are below:
- ` + HeaderHref + ` (required for updating a label dimension)
- ` + HeaderKey + ` (required for creating a label dimension)
- ` + HeaderDisplayName + `
- ` + HeaderFGColor + `
- ` + HeaderBGColor + `
- ` + HeaderInitial + `
- ` + HeaderDisplayPlural + `
- ` + HeaderIcon + `
- ` + HeaderExternalDataSet + `
- ` + HeaderExternalDataRef + `

If an href is provided, workloader will make sure the label dimension is what's in the CSV. If no href is provided, workloader looks to create a new label dimension.
	
Recommended to run without --update-pce first to log of what will change. If --update-pce is used, workloader will create the labels with a user prompt. To disable the prompt, use --no-prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get the viper values
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		ImportLabelDimensions(&pce, csvFile, updatePCE, noPrompt)
	},
}

type csvLabelDimension struct {
	labelDimension illumioapi.LabelDimension
	csvLine        int
}

// ImportLabels imports IP Lists to a target PCE from a CSV file
func ImportLabelDimensions(pce *illumioapi.PCE, inputFile string, updatePCE, noPrompt bool) {

	// Get the existing label dimensions
	api, err := pce.GetLabelDimensions(nil)
	utils.LogAPIRespV2("GetLabelDimensions", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Parse the csv
	csvData, err := utils.ParseCSV(inputFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create the header map and new/update slices
	csvHeaderMap := make(map[string]int)
	newLabelDimensions := []csvLabelDimension{}
	updateLabelDimensions := []csvLabelDimension{}

	// Iterate through each row of the csv
	for rowIndex, row := range csvData {

		// Set the update to false
		update := false

		// If it's the first row, process the headers
		if rowIndex == 0 {
			for col, header := range row {
				csvHeaderMap[header] = col
			}
			continue
		}

		// Check if the label dimension exists
		pceLD := illumioapi.LabelDimension{}
		key := ""
		if col, exists := csvHeaderMap[HeaderHref]; exists && row[col] != "" {
			// Get the PCE label dimension
			pceLD = pce.LabelDimensions[row[col]]
			if pceLD.Href == "" {
				utils.LogError(fmt.Sprintf("csv row %d - %s does not exist as a label dimension", rowIndex+1, row[col]))
			}
		} else {
			// If it doesn't exist, ensure we have a key to create it
			if col, exists := csvHeaderMap[HeaderKey]; !exists || row[col] == "" {
				utils.LogError(fmt.Sprintf("csv row %d - key must exist", rowIndex+1))
			} else {
				pceLD = pce.LabelDimensions[row[col]]
				key = row[col]
			}
		}

		// Create the CSV Label dimension
		csvLD := illumioapi.LabelDimension{DisplayInfo: &illumioapi.DisplayInfo{}, ExternalDataSet: illumioapi.Ptr(""), ExternalDataReference: illumioapi.Ptr("")}

		// PCE Targets
		pceTargets := []string{pceLD.DisplayName, illumioapi.PtrToVal(pceLD.ExternalDataReference), illumioapi.PtrToVal(pceLD.ExternalDataSet)}
		if pceLD.DisplayInfo != nil {
			pceTargets = append(pceTargets, pceLD.DisplayInfo.ForegroundColor, pceLD.DisplayInfo.BackgroundColor, pceLD.DisplayInfo.Initial, pceLD.DisplayInfo.DisplayNamePlural, pceLD.DisplayInfo.Icon)
		} else {
			pceTargets = append(pceTargets, "", "", "", "", "")
		}

		// CSV Headers to process for updates
		csvHeaders := []string{HeaderDisplayName, HeaderExternalDataRef, HeaderExternalDataSet, HeaderFGColor, HeaderBGColor, HeaderInitial, HeaderDisplayPlural, HeaderIcon}

		// CSV label dimensions to process
		csvLDTargets := []*string{&csvLD.DisplayName, csvLD.ExternalDataReference, csvLD.ExternalDataSet, &csvLD.DisplayInfo.ForegroundColor, &csvLD.DisplayInfo.BackgroundColor, &csvLD.DisplayInfo.Initial, &csvLD.DisplayInfo.DisplayNamePlural, &csvLD.DisplayInfo.Icon}

		// Iterate over the csv Headers
		for i, h := range csvHeaders {
			// Check if the header exists
			if col, exists := csvHeaderMap[h]; exists {

				// Set the value
				*csvLDTargets[i] = row[col]

				// If the PCE Label dimensions exists compare and log
				if pceLD.Href != "" && row[col] != pceTargets[i] {
					update = true
					utils.LogInfo(fmt.Sprintf("csv row %d - %s to be updated from %s to %s", rowIndex+1, h, pceTargets[i], row[col]), true)
				}
			}
		}

		// Add if updating
		if update {
			csvLD.Href = pceLD.Href
			updateLabelDimensions = append(updateLabelDimensions, csvLabelDimension{labelDimension: csvLD, csvLine: rowIndex + 1})
		}
		// Add if creating
		if pceLD.Href == "" {
			csvLD.Key = key
			newLabelDimensions = append(newLabelDimensions, csvLabelDimension{labelDimension: csvLD, csvLine: rowIndex + 1})
		}
	}

	// End run if we have nothing to do
	if len(newLabelDimensions) == 0 && len(updateLabelDimensions) == 0 {
		utils.LogInfo("nothing to be done.", true)

		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d label dimensions to create and %d label dimensions to update. See workloader.log for all identified changes. To do the import, run again using --update-pce flag", len(newLabelDimensions), len(updateLabelDimensions)), true)

		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("[PROMPT] - workloader will create %d labels and update %d labels in %s (%s). Do you want to run the import (yes/no)? ", len(newLabelDimensions), len(updateLabelDimensions), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))

		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("Prompt denied", true)

			return
		}
	}

	// Create new labels
	var updatedLabelDimensions, createdLabelDimensions int

	for _, create := range newLabelDimensions {
		labelDimension, a, err := pce.CreateLabelDimension(create.labelDimension)
		utils.LogAPIRespV2("CreateLabelDimension", a)
		if err != nil {
			utils.LogError(fmt.Sprintf("csv line %d - %s - %d labels created - %d labels updated", create.csvLine, err, createdLabelDimensions, updatedLabelDimensions))
		}
		utils.LogInfo(fmt.Sprintf("csv line %d - %s created - %s - status code %d", create.csvLine, create.labelDimension.Key, labelDimension.Href, a.StatusCode), true)
		createdLabelDimensions++
	}

	for _, update := range updateLabelDimensions {
		a, err := pce.UpdateLabelDimension(update.labelDimension)
		utils.LogAPIRespV2("UpdateLabelDimension", a)
		if err != nil {
			utils.LogError(fmt.Sprintf("csv line %d - %s - %d labels created - %d labels updated", update.csvLine, err, createdLabelDimensions, updatedLabelDimensions))
		}
		utils.LogInfo(fmt.Sprintf("csv line %d - %s created - %s - status code %d", update.csvLine, update.labelDimension.Key, update.labelDimension.Href, a.StatusCode), true)
		createdLabelDimensions++
	}

	// Log command end

}
