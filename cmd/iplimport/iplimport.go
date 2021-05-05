package iplimport

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce illumioapi.PCE
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
Create and update IPlists from a CSV file. 

The input should have a header row as the first row will be skipped. An example input file is below.
	
The default import format is below. It matches the columns of the workloader ipl-export command to easily export workloads, edit, and reimport.

+------------+-------------+-----------------------------------------+----------------------------------------------------------------------------------+-----------------+-------------------+-------------------+
|    name    | description |                 include                 |                                     exclude                                      |      fqdns      | external_data_set | external_data_ref |
+------------+-------------+-----------------------------------------+----------------------------------------------------------------------------------+-----------------+-------------------+-------------------+
| Internet   |             | 0.0.0.0/0                               | 10.0.0.0/8;172.16.0.0/12;192.168.0.0/16;169.254.0.0/16;224.0.0.0-239.255.255.255 |                 |                   |                   |
| Link Local |             | 169.254.0.0/16                          |                                                                                  |                 |                   |                   |
| RFC 1918   |             | 10.0.0.0/8;172.16.0.0/12;192.168.0.0/16 |                                                                                  |                 |                   |                   |
| Multicast  |             | 224.0.0.0-239.255.255.255               |                                                                                  |                 |                   |                   |
| Microsoft  |             |                                         |                                                                                  | *.microsoft.com |                   |                   |
+------------+-------------+-----------------------------------------+----------------------------------------------------------------------------------+-----------------+-------------------+-------------------+
	
Recommended to run without --update-pce first to log of what will change. If --update-pce is used, ipl-import will create the IP lists with a  user prompt. To disable the prompt, use --no-prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
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
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		ImportIPLists(pce, csvFile, updatePCE, noPrompt, debug, provision)
	},
}

// ImportIPLists imports IP Lists to a target PCE from a CSV file
func ImportIPLists(pce illumioapi.PCE, csvFile string, updatePCE, noPrompt, debug, provision bool) {

	// Log command execution
	utils.LogStartCommand("ipl-import")

	// Hard code in some columns so we can make flags later
	nameCol := 1
	descCol := 2
	incCol := 3
	excCol := 4
	fqdnsCol := 5
	extDsCol := 6
	extDrCol := 7

	// Lower the hard-coded values by 1
	nameCol--
	descCol--
	incCol--
	excCol--
	fqdnsCol--
	extDsCol--
	extDrCol--

	// Create a map for our CSV ip lists
	type entry struct {
		IPL     illumioapi.IPList
		csvLine int
	}

	csvIPLs := make(map[string]entry)

	// Open CSV File
	file, err := os.Open(csvFile)
	if err != nil {
		utils.LogError(err.Error())
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
			utils.LogError(err.Error())
		}

		// Skip the header row
		if i == 1 {
			continue
		}

		// Create our array of ranges
		ranges := []*illumioapi.IPRange{}

		// Build our ranges for include
		includeCSV := strings.Split(strings.ReplaceAll(line[incCol], " ", ""), ";")
		if includeCSV[0] == "" {
			includeCSV = nil
		}
		excludeCSV := strings.Split(strings.ReplaceAll(line[excCol], " ", ""), ";")
		if excludeCSV[0] == "" {
			excludeCSV = nil
		}
		fqdns := strings.Split(strings.ReplaceAll(line[fqdnsCol], " ", ""), ";")
		if fqdns[0] == "" {
			fqdns = nil
		}

		// Iterate the includes
		for _, i := range includeCSV {
			iprange := illumioapi.IPRange{}
			if strings.Contains(i, "-") {
				iprange.FromIP = strings.Split(i, "-")[0]
				iprange.ToIP = strings.Split(i, "-")[1]
			} else {
				iprange.FromIP = i
			}
			ranges = append(ranges, &iprange)
		}

		// Iterate the excludes
		for _, i := range excludeCSV {
			if len(excludeCSV) == 1 && len(excludeCSV[0]) == 0 {
				continue
			}
			iprange := illumioapi.IPRange{}
			if strings.Contains(i, "-") {
				iprange.FromIP = strings.Split(i, "-")[0]
				iprange.ToIP = strings.Split(i, "-")[1]
			} else {
				iprange.FromIP = i
			}
			iprange.Exclusion = true
			ranges = append(ranges, &iprange)
		}

		// Iterate the FQDNs
		fqdnsEntry := []*illumioapi.FQDN{}
		for _, i := range fqdns {
			if i != "" {
				fqdnsEntry = append(fqdnsEntry, &illumioapi.FQDN{FQDN: i})
			}
		}

		// Add our IPlist to our CSV Map
		csvIPLs[line[nameCol]] = entry{
			csvLine: i,
			IPL:     illumioapi.IPList{Name: line[nameCol], Description: line[descCol], IPRanges: ranges, FQDNs: fqdnsEntry, ExternalDataSet: line[extDsCol], ExternalDataReference: line[extDrCol]}}
	}

	// Get all IP lists in the pce
	existingIPLsSlice, a, err := pce.GetAllDraftIPLists()
	utils.LogAPIResp("GetAllDraftIPLists", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	existingIPLs := make(map[string]illumioapi.IPList)
	for _, e := range existingIPLsSlice {
		existingIPLs[e.Name] = e
	}

	// Create a map of CSV IP ranges
	csvRangeMap := make(map[string]bool)
	for _, ipl := range csvIPLs {
		for _, iplr := range ipl.IPL.IPRanges {
			csvRangeMap[fmt.Sprintf("%s%s%s%t", ipl.IPL.Name, iplr.FromIP, iplr.ToIP, iplr.Exclusion)] = true
		}
		for _, f := range ipl.IPL.FQDNs {
			csvRangeMap[fmt.Sprintf("%s%s", ipl.IPL.Name, f.FQDN)] = true
		}
	}

	// Create a map of Existing IP ranges
	existingRangeMap := make(map[string]bool)
	for _, ipl := range existingIPLs {
		for _, iplr := range ipl.IPRanges {
			existingRangeMap[fmt.Sprintf("%s%s%s%t", ipl.Name, iplr.FromIP, iplr.ToIP, iplr.Exclusion)] = true
		}
		for _, f := range ipl.FQDNs {
			existingRangeMap[fmt.Sprintf("%s%s", ipl.Name, f.FQDN)] = true
		}
	}

	// Create slice to hold new IPLs and IPLs that need update
	var IPLsToCreate, IPLsToUpdate []entry

	// Iterate through each CSV IP list and see what we need to do
	for n, csvIPL := range csvIPLs {
		if existingIPL, ok := existingIPLs[n]; !ok {
			utils.LogInfo(fmt.Sprintf("CSV Line %d - %s does not exist and will be created.", csvIPL.csvLine, csvIPL.IPL.Name), false)
			IPLsToCreate = append(IPLsToCreate, csvIPL)
		} else {
			// The IP List does exist in the PCE.
			// Start the log message
			logMsg := fmt.Sprintf("%s exists in the PCE.", csvIPL.IPL.Name)

			// Set the update value to false
			update := false

			// Check the description
			if existingIPL.Description != csvIPL.IPL.Description {
				update = true
				logMsg = fmt.Sprintf("%s Descrption requires updating.", logMsg)
			}

			// Check that all IP ranges from CSV are in the PCE
			for _, r := range csvIPL.IPL.IPRanges {
				if !existingRangeMap[fmt.Sprintf("%s%s%s%t", csvIPL.IPL.Name, r.FromIP, r.ToIP, r.Exclusion)] {
					rangeTxt := r.FromIP
					if r.ToIP != "" {
						rangeTxt = fmt.Sprintf("%s-%s", r.FromIP, r.ToIP)
					}
					if r.Exclusion {
						rangeTxt = fmt.Sprintf("!%s", rangeTxt)
					}
					logMsg = fmt.Sprintf("%s %s is in the CSV but not in the PCE IP List. It will be added to the IP List in the PCE.", logMsg, rangeTxt)
					update = true
				}
			}

			// Check that all IP ranges from PCE are in CSV
			for _, r := range existingIPL.IPRanges {
				if !csvRangeMap[fmt.Sprintf("%s%s%s%t", existingIPL.Name, r.FromIP, r.ToIP, r.Exclusion)] {
					rangeTxt := r.FromIP
					if r.ToIP != "" {
						rangeTxt = fmt.Sprintf("%s-%s", r.FromIP, r.ToIP)
					}
					if r.Exclusion {
						rangeTxt = fmt.Sprintf("!%s", rangeTxt)
					}
					logMsg = fmt.Sprintf("%s %s is not in the CSV but is existing in the PCE. It will be removed from the IP List in the PCE.", logMsg, rangeTxt)
					update = true
				}
			}

			// Check that FQDNs in the CSV are in the PCE
			for _, f := range csvIPL.IPL.FQDNs {
				if !existingRangeMap[fmt.Sprintf("%s%s", csvIPL.IPL.Name, f.FQDN)] {
					logMsg = fmt.Sprintf("%s %s is in the CSV but not in the PCE FQDN list. It will be added to the IP List in the PCE.", logMsg, f.FQDN)
					update = true
				}
			}

			// Check that FQDNs in the PCE are in the CSV
			for _, f := range existingIPL.FQDNs {
				if !csvRangeMap[fmt.Sprintf("%s%s", existingIPL.Name, f.FQDN)] {
					logMsg = fmt.Sprintf("%s %s is not in the CSV but is existing in the PCE FQDN list. It will be removed from the IP List in the PCE.", logMsg, f.FQDN)
					update = true
				}
			}

			// If we don't need to update, adjust the log message
			if !update {
				logMsg = fmt.Sprintf("%s No updates required", logMsg)
			}

			// Log the log message
			utils.LogInfo(fmt.Sprintf("CSV Line %d - %s", csvIPL.csvLine, logMsg), false)

			// If we need to update, add it to our slice
			if update {
				csvIPL.IPL.Href = existingIPL.Href
				IPLsToUpdate = append(IPLsToUpdate, csvIPL)
			}
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
		utils.LogInfo(fmt.Sprintf("workloader identified %d ip-lists to create and %d ip-lists to update. See workloader.log for all identified changes. To do the import, run again using --update-pce flag", len(IPLsToCreate), len(IPLsToUpdate)), true)
		utils.LogEndCommand("ipl-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("[PROMPT] - workloader will create %d iplists and update %d iplists in %s (%s). Do you want to run the import (yes/no)? ", len(IPLsToCreate), len(IPLsToUpdate), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))

		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("Prompt denied for creating %d iplists and updating %d iplists.", len(IPLsToCreate), len(IPLsToUpdate)), true)
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
		utils.LogAPIResp("CreateIPList", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("Ending run - %d IP Lists created - %d IP Lists updated.", createdIPLs, updatedIPLs))
			utils.LogError(err.Error())
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("CSV Line %d - %s - 406 Not Acceptable - See workloader.log for more details", newIPL.csvLine, newIPL.IPL.Name), true)
			utils.LogWarning(a.RespBody, false)
			skippedIPLs++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("CSV Line %d - %s created - status code %d", newIPL.csvLine, ipl.Name, a.StatusCode), true)
			createdIPLs++
			provisionableIPLs = append(provisionableIPLs, ipl.Href)
		}
	}

	// Update IPLs
	for _, updateIPL := range IPLsToUpdate {
		a, err := pce.UpdateIPList(updateIPL.IPL)
		utils.LogAPIResp("UpdateIPList", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("Ending run - %d IP Lists created - %d IP Lists updated.", createdIPLs, updatedIPLs))
			utils.LogError(err.Error())
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("CSV Line %d - %s - 406 Not Acceptable - See workloader.log for more details", updateIPL.csvLine, updateIPL.IPL.Name), true)
			utils.LogWarning(a.RespBody, false)
			skippedIPLs++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("CSV Line %d - %s updated - status code %d", updateIPL.csvLine, updateIPL.IPL.Name, a.StatusCode), true)
			updatedIPLs++
			provisionableIPLs = append(provisionableIPLs, updateIPL.IPL.Href)
		}
	}

	// Provision
	if provision {
		a, err := pce.ProvisionHref(provisionableIPLs, "workloader wkld-to-ipl")
		utils.LogAPIResp("ProvisionHrefs", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("Provisioning successful - status code %d", a.StatusCode), true)
	}

	utils.LogEndCommand("ipl-import")

}
