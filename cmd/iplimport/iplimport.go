package iplimport

import (
	"fmt"
	"os"
	"sort"
	"strings"

	ia "github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce ia.PCE
var err error
var provision, debug, updatePCE, noPrompt bool
var csvFile string

func init() {
	IplImportCmd.Flags().BoolVarP(&provision, "provision", "p", false, "Provision IP Lists after creating and/or updating.")
}

// IplImportCmd runs the iplist import command
var IplImportCmd = &cobra.Command{
	Use:   "ipl-import [csv file to import]",
	Short: "Create and update IP Lists from a CSV.",
	Long: `
Create and update IP lists from a CSV file.

This command creates or updates multiple IP Lists at once with each IP list on a single line. To update (or create) a single IP list with IP entries on multiple lines, use the ipl-replace command.

The input should have a header row as the first row will be skipped. An example input file is below.

The input file requires headers and matches fields to header values. The following headers can be used:
` + "\r\n- " + HeaderName + "\r\n" +
		"- " + HeaderHref + "\r\n" +
		"- " + HeaderDescription + "\r\n" +
		"- " + HeaderInclude + "\r\n" +
		"- " + HeaderExclude + "\r\n" +
		"- " + HeaderFqdns + "\r\n" +
		"- " + HeaderExternalDataSet + "\r\n" +
		"- " + HeaderExternalDataRef + "\r\n" + `

If an href value is provided the the IPL with the matching href will be updated. If href value is not provided, the name is used to match. If the name exists it in the PCE, the IPL is updated. If the name does not exist it is created.

An example input format for multiple iplists is below.

+-----------+-------------+---------------------------+-------------------------------------------------------------------+-----------------+------+
|   name    | description |          include          |                              exclude                              |      fqdns      | href |
+-----------+-------------+---------------------------+-------------------------------------------------------------------+-----------------+------+
| Public    |             | 0.0.0.0/0                 | 10.0.0.0/8;172.16.0.0/12;169.254.0.0/16;224.0.0.0-239.255.255.255 |                 |      |
| Internal  |             | 10.0.0.0/8;172.16.0.0/12  |                                                                   |                 |      |
| Multicast |             | 224.0.0.0-239.255.255.255 |                                                                   |                 |      |
| Microsoft |             |                           |                                                                   | *.microsoft.com |      |
+-----------+-------------+---------------------------+-------------------------------------------------------------------+-----------------+------+

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, ipl-import will create the IP lists with a  user prompt. To disable the prompt, use --no-prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(false)
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
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		ImportIPLists(pce, csvFile, updatePCE, noPrompt, debug, provision)
	},
}

// ImportIPLists imports IP Lists to a target PCE from a CSV file
func ImportIPLists(pce ia.PCE, csvFile string, updatePCE, noPrompt, debug, provision bool) {

	// Log command execution
	utils.LogStartCommand("ipl-import")

	// Parse the CSV
	csvData, err := utils.ParseCSV(csvFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create a map for our CSV ip lists
	type entry struct {
		IPL     ia.IPList
		csvLine int
	}

	csvIPLs := []entry{}

	// Create the headers
	headers := make(map[string]*int)

	// Iterate through the CSV
csvEntries:
	for i, line := range csvData {
		csvLine := i + 1

		// If it's the first row, process the headers
		if i == 0 {
			for i, l := range line {
				x := i
				headers[l] = &x
			}
			continue
		}

		// Create array of ranges
		ranges := []ia.IPRange{}
		var includeCSV, excludeCSV, fqdns []string

		// Include
		if val, ok := headers[HeaderInclude]; ok && line[*val] != "" {
			includeCSV = strings.Split(strings.ReplaceAll(line[*val], " ", ""), ";")
			for _, i := range includeCSV {
				// Process description
				i = strings.Replace(i, " #", "#", -1)
				iSplit := strings.Split(i, "#")
				i = iSplit[0]
				desc := ""
				if len(iSplit) == 2 {
					desc = iSplit[1]
				}
				// Validate the IP
				if !ValidateIplistEntry(i) {
					utils.LogWarning(fmt.Sprintf("csv line %d - %s is not a valid ip list entry. skipping csv line.", csvLine, i), true)
					continue csvEntries
				}
				iprange := ia.IPRange{Description: desc}
				if strings.Contains(i, "-") {
					iprange.FromIP = strings.Split(i, "-")[0]
					iprange.ToIP = strings.Split(i, "-")[1]
				} else {
					iprange.FromIP = i
				}
				ranges = append(ranges, iprange)
			}
		}

		// Exclude
		if val, ok := headers[HeaderExclude]; ok && line[*val] != "" {
			excludeCSV = strings.Split(strings.ReplaceAll(line[*val], " ", ""), ";")
			for _, i := range excludeCSV {
				if len(excludeCSV) == 1 && len(excludeCSV[0]) == 0 {
					continue
				}
				// Remove the description
				i = strings.Replace(i, " #", "#", -1)
				iSplit := strings.Split(i, "#")
				i = iSplit[0]
				desc := ""
				if len(iSplit) == 2 {
					desc = iSplit[1]
				}
				// Validate the IP
				if !ValidateIplistEntry(i) {
					utils.LogWarning(fmt.Sprintf("csv line %d - %s is not a valid ip list entry. skipping csv line.", csvLine, i), true)
					continue csvEntries
				}
				iprange := ia.IPRange{Description: desc}
				if strings.Contains(i, "-") {
					iprange.FromIP = strings.Split(i, "-")[0]
					iprange.ToIP = strings.Split(i, "-")[1]
				} else {
					iprange.FromIP = i
				}
				iprange.Exclusion = true
				ranges = append(ranges, iprange)
			}
		}

		// FQDNs
		fqdnsEntry := []ia.FQDN{}
		if val, ok := headers[HeaderFqdns]; ok && line[*val] != "" {
			fqdns = strings.Split(strings.ReplaceAll(line[*val], " ", ""), ";")
			for _, i := range fqdns {
				if i != "" {
					fqdnsEntry = append(fqdnsEntry, ia.FQDN{FQDN: i})
				}
			}
		}

		// Create the IP list
		ipl := ia.IPList{IPRanges: &ranges, FQDNs: &fqdnsEntry}
		if val, ok := headers[HeaderName]; ok {
			ipl.Name = line[*val]
		}
		if val, ok := headers[HeaderDescription]; ok {
			ipl.Description = ia.Ptr(line[*val])
		}
		if val, ok := headers[HeaderExternalDataRef]; ok {
			ipl.ExternalDataReference = ia.Ptr(line[*val])
		}
		if val, ok := headers[HeaderExternalDataSet]; ok {
			ipl.ExternalDataSet = ia.Ptr(line[*val])
		}
		if val, ok := headers[HeaderHref]; ok {
			ipl.Href = line[*val]
		}
		// Add our IPlist to our CSV Map
		csvIPLs = append(csvIPLs, entry{csvLine: csvLine, IPL: ipl})
	}

	// Get all IP lists in the pce
	apiResps, err := pce.Load(ia.LoadInput{IPLists: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create a map of CSV IP ranges
	csvRangeMap := make(map[string]bool)
	for _, ipl := range csvIPLs {
		if ipl.IPL.IPRanges != nil {
			for _, iplr := range *ipl.IPL.IPRanges {
				csvRangeMap[fmt.Sprintf("%s%s%s%t%s", ipl.IPL.Name, iplr.FromIP, iplr.ToIP, iplr.Exclusion, iplr.Description)] = true
			}
		}
		if ipl.IPL.FQDNs != nil {
			for _, f := range *ipl.IPL.FQDNs {
				csvRangeMap[fmt.Sprintf("%s%s", ipl.IPL.Name, f.FQDN)] = true
			}
		}
	}

	// Create a map of Existing IP ranges
	existingRangeMap := make(map[string]bool)
	for _, ipl := range pce.IPListsSlice {
		if ipl.IPRanges != nil {
			for _, iplr := range *ipl.IPRanges {
				existingRangeMap[fmt.Sprintf("%s%s%s%t%s", ipl.Name, iplr.FromIP, iplr.ToIP, iplr.Exclusion, iplr.Description)] = true
			}
		}
		if ipl.FQDNs != nil {
			for _, f := range *ipl.FQDNs {
				existingRangeMap[fmt.Sprintf("%s%s", ipl.Name, f.FQDN)] = true
			}
		}
	}

	// Create slice to hold new IPLs and IPLs that need update
	var IPLsToCreate, IPLsToUpdate []entry

	// Iterate through each CSV IP list and see what we need to do
	for _, csvIPL := range csvIPLs {

		var existingIPL ia.IPList
		var ok bool

		// If an href is provided, verify it exists. If it doesn't skip the entry.
		if csvIPL.IPL.Href != "" {
			if existingIPL, ok = pce.IPLists[csvIPL.IPL.Href]; !ok {
				utils.LogWarning(fmt.Sprintf("csv line %d - %s does not exist in the PCE. skipping.", csvIPL.csvLine, csvIPL.IPL.Href), true)
				continue
			}
			// If no href is provided, search the IPL by name. If it doesn't exist, create it.
		} else if existingIPL, ok = pce.IPLists[csvIPL.IPL.Name]; !ok {
			utils.LogInfo(fmt.Sprintf("csv line %d - %s does not exist and will be created.", csvIPL.csvLine, csvIPL.IPL.Name), false)
			IPLsToCreate = append(IPLsToCreate, csvIPL)
			continue
		}

		// Get here if we are updating and start the log message
		logMsg := fmt.Sprintf("csv line %d - %s exists in the PCE.", csvIPL.csvLine, existingIPL.Href)

		// Set the update value to false
		update := false

		// Check the name
		if existingIPL.Name != csvIPL.IPL.Name {
			update = true
			logMsg = fmt.Sprintf("%s name to be updated from %s to %s.", logMsg, existingIPL.Name, csvIPL.IPL.Name)
		}

		// Check the description
		if ia.PtrToVal(existingIPL.Description) != ia.PtrToVal(csvIPL.IPL.Description) {
			update = true
			logMsg = fmt.Sprintf("%s descrption to be updated from %s to %s.", logMsg, ia.PtrToVal(existingIPL.Description), ia.PtrToVal(csvIPL.IPL.Description))
		}

		// Check that all IP ranges from CSV are in the PCE.
		if csvIPL.IPL.IPRanges != nil {
			for _, r := range *csvIPL.IPL.IPRanges {
				if !existingRangeMap[fmt.Sprintf("%s%s%s%t%s", existingIPL.Name, r.FromIP, r.ToIP, r.Exclusion, r.Description)] { // use existingIPL.Name in case name is being changed
					rangeTxt := r.FromIP
					if r.ToIP != "" {
						rangeTxt = fmt.Sprintf("%s-%s", r.FromIP, r.ToIP)
					}
					if r.Exclusion {
						rangeTxt = fmt.Sprintf("!%s", rangeTxt)
					}
					if r.Description != "" {
						rangeTxt = fmt.Sprintf("%s#%s", rangeTxt, r.Description)
					}
					logMsg = fmt.Sprintf("%s %s will be added to the ip list.", logMsg, rangeTxt)
					update = true
				}
			}
		}

		// Check that all IP ranges from PCE are in CSV
		if existingIPL.IPRanges != nil {
			for _, r := range *existingIPL.IPRanges {
				if !csvRangeMap[fmt.Sprintf("%s%s%s%t%s", csvIPL.IPL.Name, r.FromIP, r.ToIP, r.Exclusion, r.Description)] { // use csvIPL.IPL.Name in case name is being changed
					rangeTxt := r.FromIP
					if r.ToIP != "" {
						rangeTxt = fmt.Sprintf("%s-%s", r.FromIP, r.ToIP)
					}
					if r.Exclusion {
						rangeTxt = fmt.Sprintf("!%s", rangeTxt)
					}
					if r.Description != "" {
						rangeTxt = fmt.Sprintf("%s#%s", rangeTxt, r.Description)
					}
					logMsg = fmt.Sprintf("%s %s will be removed from the ip list.", logMsg, rangeTxt)
					update = true
				}
			}
		}

		// Check that FQDNs in the CSV are in the PCE
		if csvIPL.IPL.FQDNs != nil {
			for _, f := range *csvIPL.IPL.FQDNs {
				if !existingRangeMap[fmt.Sprintf("%s%s", existingIPL.Name, f.FQDN)] { // use existingIPL.Name in case name is being changed
					logMsg = fmt.Sprintf("%s %s will be added to the ip list.", logMsg, f.FQDN)
					update = true
				}
			}
		}

		// Check that FQDNs in the PCE are in the CSV
		if existingIPL.FQDNs != nil {
			for _, f := range *existingIPL.FQDNs {
				if !csvRangeMap[fmt.Sprintf("%s%s", csvIPL.IPL.Name, f.FQDN)] { // use csvIPL.IPL.Name in case name is being changed
					logMsg = fmt.Sprintf("%s %s will be removed from the ip list.", logMsg, f.FQDN)
					update = true
				}
			}
		}

		// Log the log message
		if update {
			utils.LogInfo(logMsg, false)
		}

		// If we need to update, add it to our slice
		if update {
			csvIPL.IPL.Href = existingIPL.Href
			IPLsToUpdate = append(IPLsToUpdate, csvIPL)
		}
	}

	// End run if we have nothing to do
	if len(IPLsToCreate) == 0 && len(IPLsToUpdate) == 0 {
		utils.LogInfo("nothing to be done.", true)
		utils.LogEndCommand("ipl-import")
		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d ip-lists to create and %d ip-lists to update. see workloader.log for all identified changes. to do the import, run again using --update-pce flag", len(IPLsToCreate), len(IPLsToUpdate)), true)
		utils.LogEndCommand("ipl-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("[PROMPT] - workloader will create %d iplists and update %d iplists in %s (%s). do you want to run the import (yes/no)? ", len(IPLsToCreate), len(IPLsToUpdate), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))

		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("prompt denied for creating %d iplists and updating %d iplists.", len(IPLsToCreate), len(IPLsToUpdate)), true)
			utils.LogEndCommand("ipl-import")
			return
		}
	}

	// Sort our slices by the CSV line
	sort.SliceStable(IPLsToCreate, func(i, j int) bool { return IPLsToCreate[i].csvLine < IPLsToCreate[j].csvLine })
	sort.SliceStable(IPLsToUpdate, func(i, j int) bool { return IPLsToUpdate[i].csvLine < IPLsToUpdate[j].csvLine })

	// Create new IPLs
	var updatedIPLs, createdIPLs, skippedIPLs int
	provisionableIPLs := []string{}

	for _, newIPL := range IPLsToCreate {
		ipl, a, err := pce.CreateIPList(newIPL.IPL)
		utils.LogAPIRespV2("CreateIPList", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("ending run - %d ip lists created - %d ip lists updated.", createdIPLs, updatedIPLs))
			utils.LogError(err.Error())
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("csv line %d - %s - 406 not acceptable - see workloader.log for more details", newIPL.csvLine, newIPL.IPL.Name), true)
			utils.LogWarning(a.RespBody, false)
			skippedIPLs++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("csv line %d - %s created - status code %d", newIPL.csvLine, ipl.Name, a.StatusCode), true)
			createdIPLs++
			provisionableIPLs = append(provisionableIPLs, ipl.Href)
		}
	}

	// Update IPLs
	for _, updateIPL := range IPLsToUpdate {
		a, err := pce.UpdateIPList(updateIPL.IPL)
		utils.LogAPIRespV2("UpdateIPList", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("ending run - %d ip lists created - %d ip lists updated.", createdIPLs, updatedIPLs))
			utils.LogError(err.Error())
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("csv line %d - %s - 406 not acceptable - see workloader.log for more details", updateIPL.csvLine, updateIPL.IPL.Name), true)
			utils.LogWarning(a.RespBody, false)
			skippedIPLs++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("csv line %d - %s updated - status code %d", updateIPL.csvLine, updateIPL.IPL.Name, a.StatusCode), true)
			updatedIPLs++
			provisionableIPLs = append(provisionableIPLs, updateIPL.IPL.Href)
		}
	}

	// Provision
	if provision {
		a, err := pce.ProvisionHref(provisionableIPLs, "workloader wkld-to-ipl")
		utils.LogAPIRespV2("ProvisionHrefs", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning successful - status code %d", a.StatusCode), true)
	}

	utils.LogEndCommand("ipl-import")

}
