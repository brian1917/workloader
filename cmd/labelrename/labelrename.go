package labelrename

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var csvFile string
var updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

// LabelRenameCmd Finds workloads that have no communications within an App-Group....
var LabelRenameCmd = &cobra.Command{
	Use:   "label-rename [csv file]",
	Short: "Rename labels based on CSV input.",
	Long: `
Rename labels based on CSV input.

Input file format is below and requires headers.

+-----------+-----------+-----------+
| label_key | old_value | new_value |
+-----------+-----------+-----------+
| app       | erp       | ERP       |
| role      | wb        | WEB       |
| env       | prd       | PROD      |
| loc       | bs        | BOS       |
+-----------+-----------+-----------+

Use --update-pce to make the changes in the PCE.
`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting pce - %s", err))
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)

		renameLabel()
	},
}

//renameLabel reads the csv and renames labels
func renameLabel() {

	// Log start
	utils.LogStartCommand("label-rename")

	// Get all labels
	labels, a, err := pce.GetAllLabels()
	utils.LogAPIResp("GetAllLabels", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Build the label map
	labelMap := make(map[string]illumioapi.Label)
	for _, l := range labels {
		labelMap[l.Key+l.Value] = l
	}

	// Create new label and update label slices
	var newLabels, updateLabels []illumioapi.Label

	// Open input CSV File
	file, err := os.Open(csvFile)
	if err != nil {
		utils.LogError(err.Error())
	}
	defer file.Close()
	reader := csv.NewReader(utils.ClearBOM(bufio.NewReader(file)))

	// Start the counters
	i := 0

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
			continue
		}

		// Check if it needs to be update
		existingLabel := labelMap[line[0]+line[1]]
		if existingLabel.Key == "" {
			utils.LogInfo(fmt.Sprintf("CSV Line %d - %s (%s) does not exist in the PCE. Workloader will create the desired label %s (%s)", i, line[1], line[0], line[2], line[0]), false)
			newLabels = append(newLabels, illumioapi.Label{Key: line[0], Value: line[2]})
			continue
		}

		if existingLabel.Value == line[2] {
			utils.LogInfo(fmt.Sprintf("CSV Line %d - %s (%s) is already in the PCE. No update required.", i, line[2], line[0]), false)
			continue
		}

		utils.LogInfo(fmt.Sprintf("CSV Line %d - %s (%s) PCE will be updated to %s (%s)", i, existingLabel.Value, existingLabel.Key, line[2], line[0]), false)
		updateLabels = append(updateLabels, illumioapi.Label{Key: line[0], Value: line[2], Href: labelMap[line[0]+line[1]].Href})
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("label-rename will update %d labels and create %d labels. See workloader.log for details. Run with --update-pce to make changes.", len(updateLabels), len(newLabels)), true)
		utils.LogEndCommand("label-rename")
	}

	// Prompt the user
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("label-rename will update %d labels and create %d labels in %s (%s). See workloader.log for details. Do you want to run the import (yes/no)? ", len(updateLabels), len(newLabels), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("prompt denied to update %d labels and create %d new labels.", len(updateLabels), len(newLabels)), true)
			utils.LogEndCommand("label-rename")
			return
		}
	}

	// If we get here, we want to make the changes.
	for _, l := range updateLabels {
		a, err := pce.UpdateLabel(l)
		utils.LogAPIResp("UpdateLabel", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("updated %s to %s (%s) - %d", l.Href, l.Value, l.Key, a.StatusCode), true)
	}

	for _, l := range newLabels {
		newL, a, err := pce.CreateLabel(l)
		utils.LogAPIResp("CreateLabel", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("created %s to %s (%s) - %d", newL.Href, newL.Value, newL.Key, a.StatusCode), true)
	}

	utils.LogEndCommand("label-rename")
}
