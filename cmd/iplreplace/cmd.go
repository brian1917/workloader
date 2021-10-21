package iplreplace

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/workloader/cmd/iplimport"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var ipCol, fqdnCol int
var create, provision, noHeaders, updatePCE, noPrompt bool
var csvFile, iplName string

func init() {
	IplReplaceCmd.Flags().StringVarP(&iplName, "name", "n", "", "name of ip List to update")
	IplReplaceCmd.MarkFlagRequired("name")
	IplReplaceCmd.Flags().BoolVarP(&create, "create", "c", false, "create ip list if it does not exist")
	IplReplaceCmd.Flags().IntVarP(&ipCol, "ip-col", "i", 1, "column with ip entries. first column is 1. negative number means column is not presesent.")
	IplReplaceCmd.Flags().IntVarP(&fqdnCol, "fqdn-col", "f", -1, "column with fqdn entries. first column is 1. nevative number means column is not present")
	IplReplaceCmd.Flags().BoolVarP(&noHeaders, "no-headers", "x", false, "process the first row since there are no headers.")
	IplReplaceCmd.Flags().BoolVarP(&provision, "provision", "p", false, "provision ip list after replacing contents.")

	IplReplaceCmd.Flags().SortFlags = false
}

// IplImportCmd runs the iplist import command
var IplReplaceCmd = &cobra.Command{
	Use:   "ipl-replace [csv file to import]",
	Short: "Replace all entries in an IP List with contents of a CSV file.",
	Long: `
Replace all entries in an IP List with contents of a CSV file. Two columns will be processed: one for IPs and one for FQDNs.

The default expects IPs in the first column and no FQDNs to be provided. See flags for details to change this.

The IP addresses can be individual addresses, ranges using a "-", or CIDRs. An "!" in front can be used for an exclude. (e.g., !192.168.0.0/16)

By default, the command will error if the provided IP List does not exist in the PCE. Use the --create (-c) flag to create the IP list if it does not exist.

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, ipl-import will create the IP lists with a  user prompt. To disable the prompt, use --no-prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get the viper values
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		iplReplace()
	},
}

func iplReplace() {

	// Log command execution
	utils.LogStartCommand("ipl-replace")

	// Offset the columns by 1
	ipCol--
	fqdnCol--

	// Parse the CSV
	csvData, err := utils.ParseCSV(csvFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get the IPL
	pceIPL, api, err := pce.GetIPList(iplName, "draft")
	utils.LogAPIResp("GetIPList", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	// If the IPL doesn't exist create flag is not set, log error
	iplToBeCreated := false
	if pceIPL.Href == "" {
		if !create {
			utils.LogError(fmt.Sprintf("%s does not exist in the PCE as an ip list. use the --create (-c) flag to create it.", iplName))
		} else {
			iplToBeCreated = true
			pceIPL.Name = iplName
		}
	}

	// Create our new IPL Ranges and FQDNs
	ranges := []*illumioapi.IPRange{}
	fqdns := []*illumioapi.FQDN{}
	var ipCount, fqdnCount int

	for i, line := range csvData {

		if i == 0 && !noHeaders {
			continue
		}

		if ipCol > -1 && line[ipCol] != "" {
			// Check exclusion
			ipRange := illumioapi.IPRange{}
			if line[ipCol][0:1] == "!" {
				ipRange.Exclusion = true
				line[ipCol] = strings.Replace(line[ipCol], "!", "", 1)
			}

			// Validate the IP
			if !iplimport.ValidateIplistEntry(line[ipCol]) {
				utils.LogError(fmt.Sprintf("csv line %d - %s is not a valid ip list entry", i+1, line[ipCol]))
				continue
			}
			// Process the entry
			if strings.Contains(line[ipCol], "-") {
				ipRange.FromIP = strings.Split(line[ipCol], "-")[0]
				ipRange.ToIP = strings.Split(line[ipCol], "-")[1]
			} else {
				ipRange.FromIP = line[ipCol]
			}
			ranges = append(ranges, &ipRange)
			ipCount++
		}

		// Process the FQDNs
		if fqdnCol > -1 && line[fqdnCol] != "" {
			fqdns = append(fqdns, &illumioapi.FQDN{FQDN: line[fqdnCol]})
			fqdnCount++
		}
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		if iplToBeCreated {
			utils.LogInfo(fmt.Sprintf("workloader will create %s ip list with %d ip entries and %d. to do the create, run again using --update-pce flag", iplName, ipCount, fqdnCount), true)
		} else {
			utils.LogInfo(fmt.Sprintf("workloader identified %d ip entries and %d fqdn entries to replace the existing %d ip entries and %d fqdn entries in %s ip list. to do the replace, run again using --update-pce flag", ipCount, fqdnCount, len(pceIPL.IPRanges), len(pceIPL.FQDNs), pceIPL.Name), true)
		}
		utils.LogEndCommand("ipl-replace")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		if iplToBeCreated {
			fmt.Printf("[PROMPT] - workloader will create %s ip list with %d ip entries and %d in %s(%s). do you want to run the import (yes/no)? ", iplName, ipCount, fqdnCount, pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		} else {
			fmt.Printf("[PROMPT] - workloader identified %d ip entries and %d fqdn entries to replace the existing %d ip entries and %d fqdn entries in %s ip list in %s(%s). do you want to run the import (yes/no)? ", ipCount, fqdnCount, len(pceIPL.IPRanges), len(pceIPL.FQDNs), pceIPL.Name, pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		}
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied", true)
			utils.LogEndCommand("ipl-replace")
			return
		}
	}

	// Edit the PCE IPL and update or create
	pceIPL.IPRanges = ranges
	pceIPL.FQDNs = fqdns

	if iplToBeCreated {
		pceIPL, api, err = pce.CreateIPList(pceIPL)
		utils.LogAPIResp("CreateIPList", api)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("%s create - status code %d", pceIPL.Name, api.StatusCode), true)
	} else {
		api, err = pce.UpdateIPList(pceIPL)
		utils.LogAPIResp("UpdateIPList", api)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("%s update - status code %d", pceIPL.Name, api.StatusCode), true)
	}

	// Provision
	if provision {
		a, err := pce.ProvisionHref([]string{pceIPL.Href}, "workloader ipl-replace")
		utils.LogAPIResp("ProvisionHrefs", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning - status code %d", a.StatusCode), true)
	}

}
