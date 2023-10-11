package adgroupimport

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	HeaderName        = "name"
	HeaderDescription = "description"
	HeaderSid         = "sid"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var updatePCE, noPrompt bool
var csvFile string

func init() {}

// IplImportCmd runs the iplist import command
var AdGroupImportCmd = &cobra.Command{
	Use:   "adgroup-import [csv file to import]",
	Short: "Create and update AD groups from a csv.",
	Long: `
	Create and update AD groups from a csv. 

The input should have a header row as the first row will be skipped. The CSV can have columns in any order. The processed headers are below:
- ` + HeaderName + ` (required)
- ` + HeaderSid + ` (required)
- ` + HeaderDescription + `

If the SID already exists, workloader will update the description and/or name if needed. If SID does not already exist, workloader creates a new AD group.
	
Recommended to run without --update-pce first to log of what will change. If --update-pce is used, workloader will create and update the AD groups with a user prompt. To disable the prompt, use --no-prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(false)
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

		ImportADGroups(pce, csvFile, updatePCE, noPrompt)
	},
}

type csvAdGroup struct {
	adGroup illumioapi.ConsumingSecurityPrincipals
	csvLine int
}

// ImportLabels imports IP Lists to a target PCE from a CSV file
func ImportADGroups(pce illumioapi.PCE, inputFile string, updatePCE, noPrompt bool) {

	// Log command execution
	utils.LogStartCommand("adgroup-import")

	// Get the CSV data
	csvData, err := utils.ParseCSV(inputFile)
	if err != nil {
		utils.LogErrorf("parsing csv - %s", err)
	}

	// Get all the existing AD groups
	apiResps, err := pce.Load(illumioapi.LoadInput{ConsumingSecurityPrincipals: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Set headers
	headers := make(map[string]*int)

	// Set slices for create and update
	var adGroupsToCreate, adGroupsToUpdate []csvAdGroup

	for index, row := range csvData {
		// Set the headers
		if index == 0 {
			for colIndex, col := range row {
				x := colIndex
				headers[col] = &x
			}
			// Validate the headers
			if headers[HeaderSid] == nil || headers[HeaderName] == nil {
				utils.LogErrorf("headers must contain %s and %s", HeaderName, HeaderSid)
			}
			continue
		}

		// Parse each entry in CSV
		if pceAdGroup, exists := pce.ConsumingSecurityPrincipals[row[*headers[HeaderSid]]]; exists {

			update := false
			// Check name
			if pceAdGroup.Name != row[*headers[HeaderName]] {
				utils.LogInfof(true, "csv row %d - name to be updated from %s to %s", index+1, pceAdGroup.Name, row[*headers[HeaderName]])
				pceAdGroup.Name = row[*headers[HeaderName]]
				update = true
			}

			// Check Description
			if headers[HeaderDescription] != nil && pceAdGroup.Description != row[*headers[HeaderDescription]] {
				utils.LogInfof(true, "csv row %d - description to be updated from %s to %s", index+1, pceAdGroup.Description, row[*headers[HeaderDescription]])
				pceAdGroup.Description = row[*headers[HeaderDescription]]
				update = true
			}

			// Add to update slice
			if update {
				adGroupsToUpdate = append(adGroupsToUpdate, csvAdGroup{csvLine: index + 1, adGroup: pceAdGroup})
			}
		} else {
			newAdGroup := illumioapi.ConsumingSecurityPrincipals{
				Name: row[*headers[HeaderName]],
				SID:  row[*headers[HeaderSid]],
			}
			if headers[HeaderDescription] != nil {
				newAdGroup.Description = row[*headers[HeaderDescription]]
			}
			adGroupsToCreate = append(adGroupsToCreate, csvAdGroup{csvLine: index + 1, adGroup: newAdGroup})
		}

	}

	// End run if we have nothing to do
	if len(adGroupsToCreate) == 0 && len(adGroupsToUpdate) == 0 {
		utils.LogInfo("nothing to be done.", true)
		utils.LogEndCommand("adgroup-import")
		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d ad groups to create and %d ad groups to update. See workloader.log for all identified changes. To do the import, run again using --update-pce flag", len(adGroupsToCreate), len(adGroupsToUpdate)), true)
		utils.LogEndCommand("adgroup-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("[PROMPT] - workloader will create %d ad groups and update %d ad groups in %s (%s). Do you want to run the import (yes/no)? ", len(adGroupsToCreate), len(adGroupsToUpdate), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))

		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("Prompt denied.", true)
			utils.LogEndCommand("adgroup-import")
			return
		}
	}

	// Create new labels
	var createdAdGroups, updatedAdGroups, skippedAdGroups int

	for _, newAdGroup := range adGroupsToCreate {
		adGroup, a, err := pce.CreateADUserGroup(newAdGroup.adGroup)
		utils.LogAPIRespV2("CreateADUserGroup", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("csv line %d - %s - ending run - %d ad groups created - %d ad groups updated", newAdGroup.csvLine, err, createdAdGroups, updatedAdGroups))
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("csv line %d - %s - 406 Not Acceptable - See workloader.log for more details", newAdGroup.csvLine, newAdGroup.adGroup.Name), true)
			utils.LogWarning(a.RespBody, false)
			skippedAdGroups++
		}
		if err == nil {
			utils.LogInfof(true, "csv line %d - %s - %s - created - status code %d", newAdGroup.csvLine, adGroup.Name, adGroup.Href, a.StatusCode)
			createdAdGroups++
		}
	}

	// Update AD User Groups
	for _, updateAdGroup := range adGroupsToUpdate {
		a, err := pce.UpdateADUserGroup(updateAdGroup.adGroup)
		utils.LogAPIRespV2("UpdateADUserGroup", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("csv line %d - %s - ending run - %d ad groups created - %d ad groups updated", updateAdGroup.csvLine, err, createdAdGroups, updatedAdGroups))
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("csv line %d - %s - 406 Not Acceptable - See workloader.log for more details", updateAdGroup.csvLine, updateAdGroup.adGroup.Name), true)
			utils.LogWarning(a.RespBody, false)
			skippedAdGroups++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("csv line %d - %s updated - status code %d", updateAdGroup.csvLine, updateAdGroup.adGroup.Name, a.StatusCode), true)
			updatedAdGroups++
		}
	}

	utils.LogEndCommand("adgroup-import")

}
