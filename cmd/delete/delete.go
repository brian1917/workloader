package delete

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var hrefFile, headerValue string
var debug, updatePCE, noPrompt, noProv bool
var pce illumioapi.PCE
var err error

func init() {
	DeleteCmd.Flags().BoolVarP(&noPrompt, "no-prov", "x", false, "do not provision deletes for provisionable objects. By default, all deletions will be provisioned.")
	DeleteCmd.Flags().StringVar(&headerValue, "header", "", "header to find the column with the hrefs to delete. If it's blank, the first column is used.")
}

// DeleteCmd runs the unpair
var DeleteCmd = &cobra.Command{
	Use:   "delete [csv file with hrefs to delete]",
	Short: "Delete any object with an HREF (e.g., unmanaged workloads, labels, services, IPLists, etc.) from the PCE.",
	Long: `  
Delete any object with an HREF (e.g., unmanaged workloads, labels, services, IPLists, etc.) from the PCE.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		hrefFile = args[0]

		// Get persistent flags from Viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		delete()
	},
}

func delete() {

	utils.LogStartCommand("delete")

	// Get all HREFs from the CSV file
	provision := []string{}
	var deleted, skipped int

	// Parse the CSV data
	csvData, err := utils.ParseCSV(hrefFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Set the column to 0 for default.
	col := 0

	// If a headervalue is provided, set the column number to where that is
	match := false
	if headerValue != "" {
		for i, c := range csvData[0] {
			if c == headerValue {
				col = i
				match = true
				break
			}
		}
		if !match {
			utils.LogError(fmt.Sprintf("%s does not exist as a header", headerValue))
		}
	}

	utils.LogInfo(fmt.Sprintf("deleting hrefs in col %d", col), true)

	// Iterate throguh the CSV data
	for i, line := range csvData {

		if i == 0 && !strings.Contains(line[col], "/orgs/") {
			utils.LogInfo(fmt.Sprintf("CSV Line - %d - first row is header - skipping", i+1), true)
			continue
		}

		// For each other entry, delete the href
		a, err := pce.DeleteHref(line[col])
		utils.LogAPIResp("DeleteHref", a)
		if err != nil {
			utils.LogWarning(fmt.Sprintf("CSV line %d - not deleted - status code %d", i+1, a.StatusCode), true)
			skipped++
		} else {
			// Increment the delete and log
			deleted++
			utils.LogInfo(fmt.Sprintf("CSV line %d - deleted - status code %d", i+1, a.StatusCode), true)
			// Check if we need to provision it
			if strings.Contains(line[col], "/ip_lists/") ||
				strings.Contains(line[col], "/services/") ||
				strings.Contains(line[col], "/rule_sets/") ||
				strings.Contains(line[col], "/label_groups/") ||
				strings.Contains(line[col], "/virtual_services/") ||
				strings.Contains(line[col], "/virtual_servers/") ||
				strings.Contains(line[col], "/firewall_settings/") ||
				strings.Contains(line[col], "/secure_connect_gateways/") {
				provision = append(provision, line[col])
			}
		}
	}

	// Log the deleted total
	utils.LogInfo(fmt.Sprintf("%d items deleted", deleted), true)
	utils.LogInfo(fmt.Sprintf("%d items skipped.", skipped), true)

	// Provision if needed
	if len(provision) > 0 && !noProv {
		utils.LogInfo(fmt.Sprintf("provisioning deletion of %d provisionable objects.", len(provision)), true)
		a, err := pce.ProvisionHref(provision, "deleted by workloader")
		utils.LogAPIResp("ProvisionHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning complete - status code %d", a.StatusCode), true)
	}

	utils.LogEndCommand("delete")
}
