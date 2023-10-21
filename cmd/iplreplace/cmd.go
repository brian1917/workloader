package iplreplace

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brian1917/workloader/cmd/iplexport"

	"github.com/brian1917/workloader/cmd/iplimport"

	ia "github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce ia.PCE
var err error
var ipCol, ipDescCol, fqdnCol int
var create, provision, noHeaders, noBackup, updatePCE, noPrompt bool
var iplCsvFile, fqdnCsvFile, iplName string

func init() {
	IplReplaceCmd.Flags().StringVarP(&iplCsvFile, "ip-file-name", "i", "", "name of file with ip entries")
	IplReplaceCmd.Flags().StringVarP(&fqdnCsvFile, "fqdn-file-name", "f", "", "name of file with fqdn entries")
	IplReplaceCmd.MarkFlagRequired("name")
	IplReplaceCmd.Flags().BoolVarP(&create, "create", "c", false, "create ip list if it does not exist")
	IplReplaceCmd.Flags().IntVar(&ipCol, "ip-col", 1, "column with ip entries. first column is 1.")
	IplReplaceCmd.Flags().IntVar(&ipDescCol, "ip-desc-col", -1, "column with ip entry descriptions. first column is 1. negative number means column is not presesent.")
	IplReplaceCmd.Flags().IntVar(&fqdnCol, "fqdn-col", 1, "column with fqdn entries. first column is 1.")
	IplReplaceCmd.Flags().BoolVarP(&noHeaders, "no-headers", "x", false, "process the first row since there are no headers.")
	IplReplaceCmd.Flags().BoolVar(&noBackup, "no-backup", false, "will not create a backup file of the original ip list before making changes.")
	IplReplaceCmd.Flags().BoolVarP(&provision, "provision", "p", false, "provision ip list after replacing contents.")

	IplReplaceCmd.Flags().SortFlags = false
}

// IplImportCmd runs the iplist import command
var IplReplaceCmd = &cobra.Command{
	Use:   "ipl-replace [name of IPL to replace or create]",
	Short: "Replace all entries in an IP List with contents of a CSV file.",
	Long: `
Replace all entries in an IP List with contents of a CSV file or files. Two files can be provided: one for ip entries (-i or --ip-file-name) and one for fqdns (-f or --fqdn-file-name).

The command will process 2 columns in the IP file: ip entry and description. The IP entries can be individual addresses, ranges using a "-", or CIDRs. An "!" in front can be used for an exclude. (e.g., !192.168.0.0/16)

The command will process 1 column in the FQDN file: fqdn.

Other columns can be present but will be ignored. Column location and skipping headers can be set in the flags.

By default, the command will error if the provided IP List does not exist in the PCE. Use the --create (-c) flag to create the IP list if it does not exist.

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, ipl-import will create the IP lists with a  user prompt. To disable the prompt, use --no-prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("command requires 1 argument for the name of the IP list. See usage help.")
			os.Exit(0)
		}
		iplName = args[0]

		// Get the viper values
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		iplReplace()
	},
}

func iplReplace() {

	// Offset the columns by 1
	ipCol--
	ipDescCol--
	fqdnCol--

	// Parse the CSV
	var iplCsvData, fqdnCsvData [][]string
	if iplCsvFile != "" {
		iplCsvData, err = utils.ParseCSV(iplCsvFile)
		if err != nil {
			utils.LogError(err.Error())
		}
	}
	if fqdnCsvFile != "" {
		fqdnCsvData, err = utils.ParseCSV(fqdnCsvFile)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Get the IPL
	pceIPL, api, err := pce.GetIPListByName(iplName, "draft")
	utils.LogAPIRespV2("GetIPList", api)
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
	ranges := []ia.IPRange{}
	fqdns := []ia.FQDN{}
	var ipCount, fqdnCount int

	for i, line := range iplCsvData {

		if i == 0 && !noHeaders {
			continue
		}

		// Check exclusion
		ipRange := ia.IPRange{}
		if line[ipCol][0:1] == "!" {
			ipRange.Exclusion = true
			line[ipCol] = strings.Replace(line[ipCol], "!", "", 1)
		}

		// Validate the IP
		if !iplimport.ValidateIplistEntry(line[ipCol]) {
			utils.LogError(fmt.Sprintf("csv line %d - %s is not a valid ip list entry", i+1, line[ipCol]))
			continue
		}

		// Description
		if ipDescCol > 0 {
			ipRange.Description = line[ipDescCol]
		}

		// Process the entry
		if strings.Contains(line[ipCol], "-") {
			ipRange.FromIP = strings.Split(line[ipCol], "-")[0]
			ipRange.ToIP = strings.Split(line[ipCol], "-")[1]
		} else {
			ipRange.FromIP = line[ipCol]
		}
		ranges = append(ranges, ipRange)
		ipCount++

	}

	for i, line := range fqdnCsvData {
		if i == 0 && !noHeaders {
			continue
		}
		// Process the FQDNs
		fqdns = append(fqdns, ia.FQDN{FQDN: line[fqdnCol]})
		fqdnCount++
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		if iplToBeCreated {
			utils.LogInfo(fmt.Sprintf("workloader will create %s ip list with %d ip entries and %d. to do the create, run again using --update-pce flag", iplName, ipCount, fqdnCount), true)
		} else {
			utils.LogInfo(fmt.Sprintf("workloader identified %d ip entries and %d fqdn entries to replace the existing %d ip entries and %d fqdn entries in %s ip list. to do the replace, run again using --update-pce flag", ipCount, fqdnCount, len(*pceIPL.IPRanges), len(*pceIPL.FQDNs), pceIPL.Name), true)
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
			fmt.Printf("[PROMPT] - workloader identified %d ip entries and %d fqdn entries to replace the existing %d ip entries and %d fqdn entries in %s ip list in %s(%s). do you want to run the import (yes/no)? ", ipCount, fqdnCount, len(*pceIPL.IPRanges), len(*pceIPL.FQDNs), pceIPL.Name, pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		}
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied", true)
			utils.LogEndCommand("ipl-replace")
			return
		}
	}

	// Create backup
	if !noBackup {
		utils.LogInfo("creating backup file of original ip list ...", true)
		iplexport.ExportIPL(pce, pceIPL.Name, fmt.Sprintf("workloader-ip-list-backup-%s-%s.csv", pceIPL.Name, time.Now().Format("20060102_150405")))
	}

	// Edit the PCE IPL and update or create
	pceIPL.IPRanges = &ranges
	pceIPL.FQDNs = &fqdns

	if iplToBeCreated {
		pceIPL, api, err = pce.CreateIPList(pceIPL)
		utils.LogAPIRespV2("CreateIPList", api)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("%s create - status code %d", pceIPL.Name, api.StatusCode), true)
	} else {
		api, err = pce.UpdateIPList(pceIPL)
		utils.LogAPIRespV2("UpdateIPList", api)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("%s update - status code %d", pceIPL.Name, api.StatusCode), true)
	}

	// Provision
	if provision {
		a, err := pce.ProvisionHref([]string{pceIPL.Href}, "workloader ipl-replace")
		utils.LogAPIRespV2("ProvisionHrefs", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning - status code %d", a.StatusCode), true)
	}

}
