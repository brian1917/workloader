package labelimport

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	HeaderHref          = "href"
	HeaderKey           = "key"
	HeaderValue         = "value"
	HeaderExtDataSet    = "ext_dataset"
	HeaderExtDataSetRef = "ext_dataset_ref"
	HeaderCreatedAt     = "created_at"
	HeaderCreatedBy     = "created_by"
	HeaderUpdatedAt     = "updated_at"
	HeaderUpdatedBy     = "updated_by"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var updatePCE, noPrompt bool
var csvFile string

func init() {}

// IplImportCmd runs the iplist import command
var LabelImportCmd = &cobra.Command{
	Use:   "label-import [csv file to import]",
	Short: "Create and update labels from a CSV.",
	Long: `
Create and update labels from a CSV file. 

The input should have a header row as the first row will be skipped. The CSV can have columns in any order. The processed headers are below:
- ` + HeaderHref + `
- ` + HeaderKey + ` (required)
- ` + HeaderValue + ` (required)
- ` + HeaderExtDataSet + `
- ` + HeaderExtDataSetRef + `

If an href is provided, workloader will make sure the label is what's in the CSV. If no href is provided, workloader looks to create a new label.
	
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

		ImportLabels(pce, csvFile, updatePCE, noPrompt)
	},
}

type csvLabel struct {
	label   illumioapi.Label
	csvLine int
}

// ImportLabels imports IP Lists to a target PCE from a CSV file
func ImportLabels(pce illumioapi.PCE, inputFile string, updatePCE, noPrompt bool) {

	// Open CSV File
	file, err := os.Open(inputFile)
	if err != nil {
		utils.LogErrorf("error opening %s - %s", csvFile, err)
	}
	defer file.Close()
	reader := csv.NewReader(utils.ClearBOM(bufio.NewReader(file)))

	// Get all the labels
	apiResps, err := pce.Load(illumioapi.LoadInput{Labels: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Start the counters
	i := 0

	// Set headers
	headers := make(map[string]*int)

	// Set slices for create and update
	var labelsToCreate, labelsToUpdate []csvLabel

	// Iterate through CSV entries
	for {

		// Increment the counter
		i++

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.LogError(err.Error())
		}

		// Skip the header row
		if i == 1 {
			for c, l := range line {
				x := c
				headers[l] = &x
			}
			if _, ok := headers["key"]; !ok {
				utils.LogError("csv requires a key header.")
			}
			if _, ok := headers["value"]; !ok {
				utils.LogError("csv requires a value header.")
			}
			continue
		}

		// No href provided means check if the label exists and create it if not
		if headers[HeaderHref] == nil || line[*headers[HeaderHref]] == "" {
			// Check if the label already exists in the PCE
			if val, ok := pce.Labels[line[*headers[HeaderKey]]+line[*headers[HeaderValue]]]; ok {
				utils.LogInfo(fmt.Sprintf("csv line %d - %s (%s) already exists - %s. to edit provide the href in the csv input.", i, val.Value, val.Key, val.Href), false)
			} else {
				// Create the label if it doesn't already exist.
				label := illumioapi.Label{
					Key:   line[*headers[HeaderKey]],
					Value: line[*headers[HeaderValue]]}
				if headers[HeaderExtDataSetRef] != nil {
					label.ExternalDataReference = illumioapi.Ptr(line[*headers[HeaderExtDataSetRef]])
				}
				if headers[HeaderExtDataSet] != nil {
					label.ExternalDataSet = illumioapi.Ptr(line[*headers[HeaderExtDataSet]])
				}
				// If either data reference or dataset is blank, don't include them
				if illumioapi.PtrToVal(label.ExternalDataReference) == "" || illumioapi.PtrToVal(label.ExternalDataSet) == "" {
					label.ExternalDataReference = nil
					label.ExternalDataSet = nil
				}
				labelsToCreate = append(labelsToCreate, csvLabel{label: label, csvLine: i})
				label.Href = "To-Be-Created-From-This-Workloader-Run"
				pce.Labels[line[*headers[HeaderKey]]+line[*headers[HeaderValue]]] = label
				utils.LogInfo(fmt.Sprintf("csv line %d - %s (%s) to be created", i, label.Value, label.Key), false)
			}
		} else {
			// We are updating the labels here because there is an href
			if val, ok := pce.Labels[line[*headers[HeaderHref]]]; !ok {
				utils.LogWarning(fmt.Sprintf("csv line %d - %s does not exist in the PCE. Skipping", i, line[*headers[HeaderHref]]), true)
			} else {
				update := false
				comments := []string{}
				if val.Key != line[*headers[HeaderKey]] {
					utils.LogWarning(fmt.Sprintf("csv line %d - %s - cannot change label key. Skipping", i, line[*headers[HeaderHref]]), true)
					continue
				}
				if headers[HeaderValue] != nil && val.Value != line[*headers[HeaderValue]] {
					comments = append(comments, fmt.Sprintf("value will be updated from %s to %s", val.Value, line[*headers[HeaderValue]]))
					update = true
					val.Value = line[*headers[HeaderValue]]
				}
				if headers[HeaderExtDataSetRef] != nil && illumioapi.PtrToVal(val.ExternalDataReference) != line[*headers[HeaderExtDataSetRef]] {
					comments = append(comments, fmt.Sprintf("external_data_ref will be updated from %s to %s", illumioapi.PtrToVal(val.ExternalDataReference), line[*headers[HeaderExtDataSetRef]]))
					update = true
					val.ExternalDataReference = illumioapi.Ptr(line[*headers[HeaderExtDataSetRef]])
				}
				if headers[HeaderExtDataSet] != nil && illumioapi.PtrToVal(val.ExternalDataSet) != line[*headers[HeaderExtDataSet]] {
					comments = append(comments, fmt.Sprintf("external_data_set will be updated from %s to %s", illumioapi.PtrToVal(val.ExternalDataSet), line[*headers[HeaderExtDataSet]]))
					update = true
					val.ExternalDataSet = illumioapi.Ptr(line[*headers[HeaderExtDataSet]])
				}
				if update {
					labelsToUpdate = append(labelsToUpdate, csvLabel{csvLine: i, label: val})
					utils.LogInfo(fmt.Sprintf("csv line %d - %s - %s", i, val.Href, strings.Join(comments, "; ")), false)
				}
			}
		}

	}

	// End run if we have nothing to do
	if len(labelsToCreate) == 0 && len(labelsToUpdate) == 0 {
		utils.LogInfo("nothing to be done.", true)

		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d labels to create and %d labels to update. See workloader.log for all identified changes. To do the import, run again using --update-pce flag", len(labelsToCreate), len(labelsToUpdate)), true)

		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("[PROMPT] - workloader will create %d labels and update %d labels in %s (%s). Do you want to run the import (yes/no)? ", len(labelsToCreate), len(labelsToUpdate), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))

		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("Prompt denied for creating %d labels and updating %d labels.", len(labelsToCreate), len(labelsToUpdate)), true)

			return
		}
	}

	// Create new labels
	var updatedLabels, createdLabels, skippedLabels int

	for _, newLabel := range labelsToCreate {
		label, a, err := pce.CreateLabel(newLabel.label)
		utils.LogAPIRespV2("CreateLabel", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("csv line %d - %s - ending run - %d labels created - %d labels updated", newLabel.csvLine, err, createdLabels, updatedLabels))
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("csv line %d - %s (%s) - 406 Not Acceptable - See workloader.log for more details", newLabel.csvLine, newLabel.label.Value, newLabel.label.Key), true)
			utils.LogWarning(a.RespBody, false)
			skippedLabels++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("csv line %d - %s (%s) created - %s - status code %d", newLabel.csvLine, label.Value, label.Key, label.Href, a.StatusCode), true)
			createdLabels++
		}
	}

	// Update IPLs
	for _, updateLabel := range labelsToUpdate {
		a, err := pce.UpdateLabel(updateLabel.label)
		utils.LogAPIRespV2("UpdateLabel", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("csv line %d - %s - ending run - %d labels created - %d labels updated", updateLabel.csvLine, err, createdLabels, updatedLabels))
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("csv line %d - %s (%s) - 406 Not Acceptable - See workloader.log for more details", updateLabel.csvLine, updateLabel.label.Value, updateLabel.label.Key), true)
			utils.LogWarning(a.RespBody, false)
			skippedLabels++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("csv line %d - %s updated - status code %d", updateLabel.csvLine, updateLabel.label.Href, a.StatusCode), true)
			updatedLabels++
		}
	}

}
