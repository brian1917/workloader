package iplimport

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

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug, updatePCE, noPrompt bool
var csvFile, outFormat string

// IplImportCmd runs the iplist import command
var IplImportCmd = &cobra.Command{
	Use:   "ipl-import [csv file to import]",
	Short: "Create and update IP Lists from a CSV.",
	Long: `
Create and update IPlists from a CSV file. 

The input should have a header row as the first row will be skipped. An example input file is below.
	
The default import format is below. It matches the columns of the workloader ipl-export command to easily export workloads, edit, and reimport.
	
+---------------+------------------+--------------------------------------------------+-----------------------------------------+------------------+-------------------+
|     name      |   description    |                     include                      |                 exclude                 | external_dataset | external_data_ref |
+---------------+------------------+--------------------------------------------------+-----------------------------------------+------------------+-------------------+
| RFC-1918      | Private IP space | 10.0.0.0/8;172.16.0.0/12;192.168.0.0/16          |                                         |                  |                   |
| Non-RFC-1918  | Public IP space  | 0.0.0.0/0                                        | 10.0.0.0/8;172.16.0.0/12;192.168.0.0/16 |                  |                   |
| Range Example | Random List      | 192.168.5.4;192.168.5.4-192.168.5.12;10.0.1.0/24 |                                         | Data set         | Reference         |
+---------------+------------------+--------------------------------------------------+-----------------------------------------+------------------+-------------------+
	
Recommended to run without --update-pce first to log of what will change. If --update-pce is used, ipl-import will create the IP lists with a  user prompt. To disable the prompt, use --no-prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetDefaultPCE(true)
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
		outFormat = viper.Get("output_format").(string)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		ImportIPLists(pce, csvFile, updatePCE, noPrompt, debug)
	},
}

// ImportIPLists imports IP Lists to a target PCE from a CSV file
func ImportIPLists(pce illumioapi.PCE, csvFile string, updatePCE, noPrompt, debug bool) {

	// Hard code in some columns so we can make flags later
	nameCol := 1
	descCol := 2
	incCol := 3
	excCol := 4
	extDsCol := 5
	extDrCol := 6

	// Lower the hard-coded values by 1
	nameCol--
	descCol--
	incCol--
	excCol--
	extDsCol--
	extDrCol--

	// Log command execution
	utils.LogInfo("running ipl-import command")

	// Create a map for our CSV ip lists
	csvIPLs := make(map[string]illumioapi.IPList)

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
		excludeCSV := strings.Split(strings.ReplaceAll(line[excCol], " ", ""), ";")

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

		// Add our IPlist to our CSV Map
		csvIPLs[line[nameCol]] = illumioapi.IPList{Name: line[nameCol], Description: line[descCol], IPRanges: ranges, ExternalDataSet: line[extDsCol], ExternalDataReference: line[extDrCol]}
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
		for _, iplr := range ipl.IPRanges {
			csvRangeMap[fmt.Sprintf("%s%s%s%t", ipl.Name, iplr.FromIP, iplr.ToIP, iplr.Exclusion)] = true
		}

	}

	// Create a map of Existing IP ranges
	existingRangeMap := make(map[string]bool)
	for _, ipl := range existingIPLs {
		for _, iplr := range ipl.IPRanges {
			existingRangeMap[fmt.Sprintf("%s%s%s%t", ipl.Name, iplr.FromIP, iplr.ToIP, iplr.Exclusion)] = true
		}

	}

	// Create slice to hold new IPLs and IPLs that need update
	var IPLsToCreate, IPLsToUpdate []illumioapi.IPList

	// Iterate through each CSV IP list and see what we need to do
	for n, csvIPL := range csvIPLs {
		if existingIPL, ok := existingIPLs[n]; !ok {
			utils.LogInfo(fmt.Sprintf("%s does not exist and will be created.", csvIPL.Name))
			IPLsToCreate = append(IPLsToCreate, csvIPL)
		} else {
			// The IP List does exist in the PCE.
			// Start the log message
			logMsg := fmt.Sprintf("%s exists in the PCE.", csvIPL.Name)

			// Set the update value to false
			update := false

			// Check the description
			if existingIPL.Description != csvIPL.Description {
				update = true
				logMsg = fmt.Sprintf("%s Descrption requires updating.", logMsg)
			}

			// Check that all IP ranges from CSV are in the PCE
			for _, r := range csvIPL.IPRanges {
				if !existingRangeMap[fmt.Sprintf("%s%s%s%t", csvIPL.Name, r.FromIP, r.ToIP, r.Exclusion)] {
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

			// If we don't need to update, adjust the log message
			if !update {
				logMsg = fmt.Sprintf("%s No updates required", logMsg)
			}

			// Log the log message
			utils.LogInfo(logMsg)

			// If we need to update, add it to our slice
			if update {
				csvIPL.Href = existingIPL.Href
				IPLsToUpdate = append(IPLsToUpdate, csvIPL)
			}
		}
	}

	// End run if we have nothing to do
	if len(IPLsToCreate) == 0 && len(IPLsToUpdate) == 0 {
		fmt.Println("[INFO] - Nothing to be done.")
		utils.LogInfo("nothing to be done. completed running ipl-import command.")
		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("import-ipl identified %d ip-lists to create and %d ip-lists to update.", len(IPLsToCreate), len(IPLsToUpdate)))
		fmt.Printf("[INFO] - import-ipl identified %d ip-lists to create and %d ip-lists to update.\r\n\r\nSee workloader.log for all identified changes. To do the import, run again using --update-pce flag\r\n", len(IPLsToCreate), len(IPLsToUpdate))
		utils.LogInfo("completed running ipl-import command")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("[INFO] - import-ipl will create %d iplists and update %d iplists. Do you want to run the import (yes/no)? ", len(IPLsToCreate), len(IPLsToUpdate))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("import identified %d iplists to be created and %d iplists requiring update. user denied prompt", len(IPLsToCreate), len(IPLsToUpdate)))
			fmt.Println("[INFO] - Prompt denied.")
			utils.LogInfo("completed running ipl-import command")
			return
		}
	}

	// Create new IPLs
	for _, newIPL := range IPLsToCreate {
		ipl, a, err := pce.CreateIPList(newIPL)
		utils.LogAPIResp("CreateIPList", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		fmt.Printf("[INFO] - %s created - status code %d\r\n", ipl.Name, a.StatusCode)
		utils.LogInfo(fmt.Sprintf("%s created - status code %d", ipl.Name, a.StatusCode))
	}

	// Update IPLs
	for _, updateIPL := range IPLsToUpdate {
		a, err := pce.UpdateIPList(updateIPL)
		utils.LogAPIResp("UpdateIPList", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		fmt.Printf("[INFO] - %s updated - status code %d\r\n", updateIPL.Name, a.StatusCode)
		utils.LogInfo(fmt.Sprintf("%s updated - status code %d", updateIPL.Name, a.StatusCode))
	}

	utils.LogInfo("completed running ipl-import")

}
