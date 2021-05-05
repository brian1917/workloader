package deletehrefs

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
var headerValue string
var err error

// Input is the input type for the Delete method
type Input struct {
	Hrefs     []string
	NoPrompt  bool
	Provision bool
	UpdatePCE bool
	PCE       illumioapi.PCE
}

var input Input

func init() {
	DeleteCmd.Flags().BoolVar(&input.Provision, "provision", false, "Provision provisionable objects after deleting them.")
	DeleteCmd.Flags().StringVar(&headerValue, "header", "", "header to find the column with the hrefs to delete. If it's blank, the first column is used.")
}

// DeleteCmd runs the unpair
var DeleteCmd = &cobra.Command{
	Use:   "delete [csv file with hrefs to delete or semi-colon separate list of hrefs]",
	Short: "Delete any object with an HREF (e.g., unmanaged workloads, labels, services, IPLists, etc.) from the PCE.",
	Long: `  
Delete any object with an HREF (e.g., unmanaged workloads, labels, services, IPLists, etc.) from the PCE.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {
		input.PCE, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		input.getHrefs(args[0])

		// Get persistent flags from Viper
		input.UpdatePCE = viper.Get("update_pce").(bool)
		input.NoPrompt = viper.Get("no_prompt").(bool)

		DeleteHrefs(input)
	},
}

// getHrefs takes the user input string and populates the input Hrefs
func (i *Input) getHrefs(userInput string) {

	// Get the HREFs from user input or the file
	if strings.Contains(userInput, "/orgs/") {
		if _, err := os.Stat(userInput); !os.IsNotExist(err) {
			utils.LogError("the provided input could be an href (contains \"/orgs/\") and is also a file. Rename the file for clarity.")
		}
		input.Hrefs = strings.Split(strings.ReplaceAll(userInput, "; ", ";"), ";")
	} else {
		// Parse the CSV data
		csvData, err := utils.ParseCSV(userInput)
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
		for i, line := range csvData {
			if i == 0 && !strings.Contains(line[col], "/orgs/") {
				utils.LogInfo(fmt.Sprintf("CSV Line - %d - first row is header - skipping", i+1), true)
				continue
			}
			input.Hrefs = append(input.Hrefs, line[col])

			// Log column
			utils.LogInfo(fmt.Sprintf("hrefs are in col %d", col+1), false)
		}
	}
}

// Delete runs the delete command
func DeleteHrefs(input Input) {

	// Log Start of the command
	utils.LogStartCommand("delete")

	var deleted, skipped int

	// Create the provision slice
	provisionMap := make(map[string]bool)

	// Make a map of unique types
	deleteCounts := make(map[string]int)

	// Iterate throguh the delete Hrefs
	for _, entry := range input.Hrefs {

		key := ""
		if strings.Contains(entry, "/labels/") {
			key = "labels"
		} else if strings.Contains(entry, "/ip_lists/") {
			key = "ip_lists"
		} else if strings.Contains(entry, "/services/") {
			key = "services"
		} else if strings.Contains(entry, "/virtual_services/") {
			key = "virtual_services"
		} else if strings.Contains(entry, "/virtual_servers/") {
			key = "virtual_servers"
		} else if strings.Contains(entry, "/pairing_profiles/") {
			key = "pairing_profiles"
		} else if strings.Contains(entry, "/sec_rules/") {
			key = "rules"
		} else if strings.Contains(entry, "/rule_sets/") {
			key = "rule_sets"
		} else if strings.Contains(entry, "/users/") {
			key = "users"
		} else if strings.Contains(entry, "/workloads/") {
			key = "unmanaged workloads"
		} else {
			x := strings.Split(entry, "/")
			x = x[:len(x)-1]
			key = strings.Join(x, "/")
		}
		// Add to the map
		deleteCounts[key] = deleteCounts[key] + 1

	}

	// Print out
	utils.LogInfo(fmt.Sprintf("%d records identified to be deleted:", len(input.Hrefs)), true)
	for key, value := range deleteCounts {
		utils.LogInfo(fmt.Sprintf("%s:%d", key, value), true)
	}

	// Log findings
	if !input.UpdatePCE {
		utils.LogInfo("Run command again with --update-pce to do the delete.", true)
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if input.UpdatePCE && !input.NoPrompt {
		var prompt string
		fmt.Printf("\r\n[PROMPT] - workloader identified %d objects to attempt to delete in %s (%s). Do you want to run the delete (yes/no)? ", len(input.Hrefs), input.PCE.FriendlyName, viper.Get(input.PCE.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied.", true)
			utils.LogEndCommand("delete")
			return
		}
	}

	// If we get here - we do the delete
	for _, href := range input.Hrefs {

		// For each other entry, delete the href
		a, _ := input.PCE.DeleteHref(href)
		utils.LogAPIResp("DeleteHref", a)
		if a.StatusCode != 204 {
			utils.LogWarning(fmt.Sprintf("%s - not deleted - status code %d", href, a.StatusCode), true)
			skipped++
		} else if a.StatusCode == 204 {
			// Increment the delete and log
			deleted++
			utils.LogInfo(fmt.Sprintf("%s - deleted - status code %d", href, a.StatusCode), true)
			// Check if we need to provision it
			if strings.Contains(href, "/ip_lists/") ||
				strings.Contains(href, "/services/") ||
				strings.Contains(href, "/rule_sets/") ||
				strings.Contains(href, "/label_groups/") ||
				strings.Contains(href, "/virtual_services/") ||
				strings.Contains(href, "/virtual_servers/") ||
				strings.Contains(href, "/firewall_settings/") ||
				strings.Contains(href, "/secure_connect_gateways/") {
				// If it's a rule, only provion the ruleset
				if strings.Contains(href, "/sec_rules/") {
					r := illumioapi.Rule{Href: href}
					provisionMap[r.GetRuleSetHrefFromRuleHref()] = true
				} else {
					provisionMap[href] = true
				}
			}
		}
	}

	// Turn the map into slice (so we have no dupes)
	provision := []string{}
	for p := range provisionMap {
		provision = append(provision, p)
	}

	// Log the deleted total
	utils.LogInfo(fmt.Sprintf("%d items deleted", deleted), true)
	utils.LogInfo(fmt.Sprintf("%d items skipped.", skipped), true)

	// Provision if needed
	if len(provision) > 0 && input.Provision {
		utils.LogInfo(fmt.Sprintf("provisioning deletion of %d provisionable objects.", len(provision)), true)
		a, err := input.PCE.ProvisionHref(provision, "deleted by workloader")
		utils.LogAPIResp("ProvisionHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning complete - status code %d", a.StatusCode), true)
	}

	utils.LogEndCommand("delete")
}
