package secprincipalimport

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/cmd/secprincipalexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {}

type newSecAuthPrincipal struct {
	secAuthPrincipal illumioapi.AuthSecurityPrincipal
	csvLine          int
}

// SecPrincipalExportCmd runs the label-dimension-export command
var SecPrincipalImportCmd = &cobra.Command{
	Use:   "sec-principal-import",
	Short: "Create external users or groups from a csv file.",
	Long: `
Create external users or groups from a csv file. 

The following headers are required:
- display_name
- name
- type (user or group)
`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}

		importSecPrincipals(pce, args[0], viper.Get("update_pce").(bool), viper.Get("no_prompt").(bool))
	},
}

func importSecPrincipals(pce illumioapi.PCE, csvFile string, updatePCE, noPrompt bool) {

	// Parse the CSV
	csvData, err := utils.ParseCSV(csvFile)
	if err != nil {
		utils.LogErrorf("parsing csv - %s", err)
	}

	// Create the slice for new groups
	secPrincipals := []newSecAuthPrincipal{}

	// Iterate over the CSV
	headersMap := make(map[string]int)
	for rowIndex, row := range csvData {
		// parse the headers
		if rowIndex == 0 {
			for colIndex, header := range row {
				headersMap[header] = colIndex
			}
			continue
		}

		// Create the new sec principal
		secPrincipal := illumioapi.AuthSecurityPrincipal{}

		if colIndex, exists := headersMap[secprincipalexport.HeaderDisplayName]; exists {
			secPrincipal.DisplayName = row[colIndex]
		}
		if colIndex, exists := headersMap[secprincipalexport.HeaderName]; exists {
			secPrincipal.Name = row[colIndex]
		}
		if colIndex, exists := headersMap[secprincipalexport.HeaderType]; exists {
			secPrincipal.Type = row[colIndex]
		}

		// Check if it exists
		if val, exists := pce.AuthSecurityPrincipals[secPrincipal.Name]; exists {
			utils.LogWarningf(true, "csv row %d - %s exists (%s). skipping", rowIndex+1, val.Name, val.Href)
			continue
		}

		// Add to the slice
		secPrincipals = append(secPrincipals, newSecAuthPrincipal{secAuthPrincipal: secPrincipal, csvLine: rowIndex + 1})
	}

	// End run of nothing to do
	if len(secPrincipals) == 0 {
		utils.LogInfo("nothing to be done.", true)
		return
	}

	if !updatePCE {
		utils.LogInfof(true, "workloader identified %d security principals to create. See workloader.log for all identified changes. To do the import, run again using --update-pce flag", len(secPrincipals))
		return
	}

	if !noPrompt {
		var prompt string
		fmt.Printf("[PROMPT] - workloader will create %d security principals in %s (%s). Do you want to run the import (yes/no)? ", len(secPrincipals), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))

		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("Prompt denied.", true)
			return
		}
	}

	// Create the groups
	for _, secPrincipal := range secPrincipals {
		_, api, err := pce.CreateAuthSecurityPrincipal(secPrincipal.secAuthPrincipal)
		utils.LogAPIRespV2("CreateAuthSecurityPrincipal", api)
		if err != nil {
			utils.LogErrorf("csv line %d - error - api status code: %d, api resp: %s", secPrincipal.csvLine, api.StatusCode, api.RespBody)
		}
		utils.LogInfof(true, "csv line %d - created %s - %d", secPrincipal.csvLine, secPrincipal.secAuthPrincipal.DisplayName, api.StatusCode)
	}
}
