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

var inputFile, csvFile, outFormat string
var debug, updatePCE, noPrompt bool
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
	
The --update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.Log(1, fmt.Sprintf("error getting pce - %s", err))
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		renameLabel()
	},
}

//renameLabel reads the csv and renames labels
func renameLabel() {

	// Log start
	utils.Log(0, "started running label-rename command")

	// Get all labels
	labels, a, err := pce.GetAllLabels()
	utils.LogAPIResp("GetAllLabels", a)
	if err != nil {
		utils.Log(1, err.Error())
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
		utils.Log(1, err.Error())
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
			utils.Log(1, err.Error())
		}

		// Skip the header row
		if i == 1 {
			continue
		}

		// Check if it needs to be update
		existingLabel := labelMap[line[0]+line[1]]
		if existingLabel.Key == "" {
			utils.Log(0, fmt.Sprintf("CSV Line %d - %s (%s) does not exist in the PCE. Workloader will create the desired label %s (%s)", i, line[1], line[0], line[2], line[0]))
			newLabels = append(newLabels, illumioapi.Label{Key: line[0], Value: line[2]})
			continue
		}

		if existingLabel.Value == line[2] {
			utils.Log(0, fmt.Sprintf("CSV Line %d - %s (%s) is already in the PCE. No update required.", i, line[2], line[0]))
			continue
		}

		utils.Log(0, fmt.Sprintf("CSV Line %d - %s (%s) PCE will be updated to %s (%s)", i, existingLabel.Value, existingLabel.Key, line[2], line[0]))
		updateLabels = append(updateLabels, illumioapi.Label{Key: line[0], Value: line[2], Href: labelMap[line[0]+line[1]].Href})
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.Log(0, fmt.Sprintf("label-rename will update %d labels and create %d labels.", len(updateLabels), len(newLabels)))
		fmt.Printf("label-rename will update %d labels and create %d labels. See workloader.log for details. Run with --update-pce to make changes.\r\n", len(updateLabels), len(newLabels))
		utils.Log(0, "completed running label-rename command")
		return
	}

	// Prompt the user
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("label-rename will update %d labels and create %d labels. See workloader.log for details. Do you want to run the import (yes/no)? ", len(updateLabels), len(newLabels))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.Log(0, fmt.Sprintf("label-rename identified %d labels requiring update and %d labels to be created. user denied prompt", len(updateLabels), len(newLabels)))
			fmt.Println("Prompt denied.")
			utils.Log(0, "completed running label-rename command")
			return
		}
	}

	// If we get here, we want to make the changes.
	for _, l := range updateLabels {
		a, err := pce.UpdateLabel(l)
		utils.LogAPIResp("UpdateLabel", a)
		if err != nil {
			utils.Log(1, err.Error())
		}
		utils.Log(0, fmt.Sprintf("updated %s to %s (%s) - %d", l.Href, l.Value, l.Key, a.StatusCode))
		fmt.Printf("[INFO] - updated %s to %s (%s) - %d\r\n", l.Href, l.Value, l.Key, a.StatusCode)
	}

	for _, l := range newLabels {
		newL, a, err := pce.CreateLabel(l)
		utils.LogAPIResp("CreateLabel", a)
		if err != nil {
			utils.Log(1, err.Error())
		}
		utils.Log(0, fmt.Sprintf("created %s to %s (%s) - %d", newL.Href, newL.Value, newL.Key, a.StatusCode))
		fmt.Printf("created %s to %s (%s) - %d\r\n", newL.Href, newL.Value, newL.Key, a.StatusCode)
	}
}
