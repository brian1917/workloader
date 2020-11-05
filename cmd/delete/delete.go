package delete

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

// Set global variables for flags
var hrefFile string
var debug, updatePCE, noPrompt, noProv bool
var pce illumioapi.PCE
var err error

func init() {
	DeleteCmd.Flags().BoolVarP(&noPrompt, "no-prov", "x", false, "do not provision deletes for provisionable objects. By default, all deletions will be provisioned.")
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
	csvFile, _ := os.Open(hrefFile)
	reader := csv.NewReader(bufio.NewReader(csvFile))
	row := 0
	provision := []string{}
	var deleted, skipped int
	for {

		// Read the CSV
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.LogError(fmt.Sprintf("Reading CSV File - %s", err))
		}

		// Increment the row counter
		row++

		// Check if HREF or header in first row
		if row == 1 && !strings.Contains(line[0], "/orgs/") {
			utils.LogInfo(fmt.Sprintf("CSV Line - %d - first row is header - skipping", row), true)
			continue
		}

		// For each other entry, delete the href
		a, err := pce.DeleteHref(line[0])
		utils.LogAPIResp("DeleteHref", a)
		if err != nil {
			utils.LogWarning(fmt.Sprintf("CSV line %d - not deleted - status code %d", row, a.StatusCode), true)
			skipped++
		} else {
			// Increment the delete and log
			deleted++
			utils.LogInfo(fmt.Sprintf("CSV line %d - deleted - status code %d", row, a.StatusCode), true)
			// Check if we need to provision it
			if strings.Contains(line[0], "/ip_lists/") ||
				strings.Contains(line[0], "/services/") ||
				strings.Contains(line[0], "/rule_sets/") ||
				strings.Contains(line[0], "/label_groups/") ||
				strings.Contains(line[0], "/virtual_services/") ||
				strings.Contains(line[0], "/virtual_servers/") ||
				strings.Contains(line[0], "/firewall_settings/") ||
				strings.Contains(line[0], "/secure_connect_gateways/") {
				provision = append(provision, line[0])
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
