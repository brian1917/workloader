package venimport

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/cmd/venexport"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pce illumioapi.PCE
var importFile string
var updatePCE, noPrompt bool

func init() {}

// WkldImportCmd runs the upload command
var VenImportCmd = &cobra.Command{
	Use:   "ven-import [csv file to import]",
	Short: "Update VENs from a CSV file.",
	Long: `
Update VENs from a CSV file.

The input file requires headers and matches fields to header values. The following headers can be used for editing (other headers will be ignored):
` + "\r\n- " + venexport.HeaderHref + " (required)\r\n" +
		"- " + venexport.HeaderDescription + "\r\n" +
		"- " + venexport.HeaderStatus + "\r\n" + `

Besides href for matching, no field is required.

It's recommend to run a ven-export and edit the same file to import with changes.

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		var err error
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.Logger.Fatalf("error getting PCE for csv command - %s", err)
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		importFile = args[0]

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		importVens()
	},
}

type updateVEN struct {
	csvLine int
	ven     illumioapi.VEN
}

func importVens() {

	// Load PCE
	apiResps, err := pce.Load(illumioapi.LoadInput{VENs: true})
	utils.LogMultiAPIResp(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Parse the CSV
	csvData, err := utils.ParseCSV(importFile)
	if err != nil {
		utils.LogError("href is a required header")
	}

	// Create our update VENs slice
	vensToUpdate := []updateVEN{}

	// Create the headers map
	headers := make(map[string]*int)

	// Iterate through the CSV
	for i, row := range csvData {

		update := false

		// If it's the first line, process the headers
		if i == 0 {
			for c, entry := range row {
				column := c
				headers[entry] = &column
			}
			if headers[venexport.HeaderHref] == nil {
				utils.LogError("")
			}
			continue
		}

		// Set the CSV Href
		csvHref := row[*headers[venexport.HeaderHref]]

		// Get the VEN
		var ven illumioapi.VEN
		var venExists bool
		if ven, venExists = pce.VENs[csvHref]; !venExists {
			utils.LogError(fmt.Sprintf("csv line %d - %s href does not exist", i+1, row[*headers[venexport.HeaderHref]]))
		}

		// Name
		if col, ok := headers[venexport.HeaderName]; ok {
			if row[*col] != pce.VENs[csvHref].Name {
				utils.LogInfo(fmt.Sprintf("csv line %d - name requires update from %s to %s", i+1, pce.VENs[csvHref].Name, row[*col]), false)
				ven.Name = row[*col]
				update = true
			}
		}

		// Description
		if col, ok := headers[venexport.HeaderDescription]; ok {
			if row[*col] != pce.VENs[csvHref].Description {
				utils.LogInfo(fmt.Sprintf("csv line %d - description requires update from %s to %s", i+1, pce.VENs[csvHref].Description, row[*col]), false)
				ven.Description = row[*col]
				update = true
			}
		}

		// Status
		if col, ok := headers[venexport.HeaderStatus]; ok {
			if strings.ToLower(row[*col]) != "active" && strings.ToLower(row[*col]) != "suspended" {
				utils.LogError(fmt.Sprintf("csv line %d - %s is not a valid status. it must be active or suspended", i+1, row[*col]))
			}
			if strings.ToLower(row[*col]) != pce.VENs[csvHref].Status {
				utils.LogInfo(fmt.Sprintf("csv line %d - status requires update from %s to %s", i+1, pce.VENs[csvHref].Status, row[*col]), false)
				ven.Status = row[*col]
				update = true
			}
		}

		if update {
			vensToUpdate = append(vensToUpdate, updateVEN{csvLine: i + 1, ven: ven})
		}
	}

	utils.LogInfo(fmt.Sprintf("%d vens requiring updates.", len(vensToUpdate)), true)

	// End if there are no updates required
	if len(vensToUpdate) == 0 {
		utils.LogEndCommand("ven-import")
		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo("See workloader.log for more details. To do the import, run again using --update-pce flag.", true)
		utils.LogEndCommand("ven-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("\r\n%s [PROMPT] - %d vens requiring update in %s(%s). Do you want to run the import (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), len(vensToUpdate), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("prompt denied to update %d vens.", len(vensToUpdate)), true)
			utils.LogEndCommand("ven-import")
			return
		}
	}

	// If we get here, we are running the update.
	for _, v := range vensToUpdate {
		a, err := pce.UpdateVen(v.ven)
		utils.LogAPIResp("UpdateVen", a)
		if err != nil {
			utils.LogWarning(fmt.Sprintf("csv line %d - %d - %s", v.csvLine, a.StatusCode, err.Error()), true)
		} else {
			utils.LogInfo(fmt.Sprintf("csv line %d - %d", v.csvLine, a.StatusCode), true)
		}

	}
	utils.LogEndCommand("ven-import")

}
