package nicmanage

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error
var outputFileName, csvFile string

func init() {
	NICManageCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
}

// NICManageCmd produces a report of all network interfaces
var NICManageCmd = &cobra.Command{
	Use:   "nic-manage [csv file to import]",
	Short: "Manage interfaces for managed or unmanaged workloads by setting ignored field to true or false.",
	Long: `
Manage interfaces for managed or unmanaged workloads by setting ignored field to true or false.

Head input CSV requires a header row with at least two headers: wkld_href and ignored. Other columns can be present as well. It is recommended to run worklodaer nic-export and  modify the ignored column in that output.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		nicManage()
	},
}

func nicManage() {

	// Log Start
	utils.LogStartCommand("nic-manage")

	// Parse the CSV file
	csvData, err := utils.ParseCSV(csvFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get the headers
	csvHeaders := findHeaders(csvData[0])

	// Get all the workloads from the PCE
	wklds, a, err := pce.GetAllWorkloadsQP(nil)
	utils.LogAPIResp("GetAllWorkloadsQP", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	wkldHrefMap := make(map[string]illumioapi.Workload)
	for _, w := range wklds {
		wkldHrefMap[w.Href] = w
	}

	// Create a map where they key is the concatenated value of the workload href and the nicName
	wkldInterfaceMap := make(map[string]bool)
	for _, w := range wklds {
		// Populate all interfaces as false since they are not ignored
		for _, iFace := range w.Interfaces {
			wkldInterfaceMap[w.Href+iFace.Name] = false
		}
		// Override the ignored interfaces
		for _, nicName := range *w.IgnoredInterfaceNames {
			wkldInterfaceMap[w.Href+nicName] = true
		}
	}

	// Create a map to hold wkldhref and interface name.
	csvInterfaces := make(map[string]bool)

	// Create a slice of workloads that need to be updated
	updatedWkldsMap := make(map[string]illumioapi.Workload)
	updatedWklds := []illumioapi.Workload{}

	// Iterate through the CSV input
	interfaceChangeCount := 0
	for rowNum, dataRow := range csvData {

		// Skip the first row
		if rowNum == 0 {
			continue
		}

		// Check if this interface has already been in the input
		if csvInterfaces[dataRow[csvHeaders.wkldHref]+dataRow[csvHeaders.interfaceName]] {
			utils.LogError(fmt.Sprintf("CSV row %d - this interface name appears twice. If using output from workloader nic-export, use the -c argument to consolidate to a single line", rowNum+1))
		} else {
			// Add it to the map if it's the first time we see it
			csvInterfaces[dataRow[csvHeaders.wkldHref]+dataRow[csvHeaders.interfaceName]] = true
		}

		// Convert the CSV value to a boolean.
		csvBool, err := strconv.ParseBool(dataRow[csvHeaders.ignored])
		if err != nil {
			utils.LogError(fmt.Sprintf("CSV row %d - %s", rowNum+1, err.Error()))
		}

		// Check if the value in the CSV matches the value in the PCE
		var pceBool, ok bool
		if pceBool, ok = wkldInterfaceMap[dataRow[csvHeaders.wkldHref]+dataRow[csvHeaders.interfaceName]]; !ok {
			utils.LogError(fmt.Sprintf("CSV row %d - interface %s does not exist on workload %s or the workload does not exist.", rowNum+1, dataRow[csvHeaders.interfaceName], dataRow[csvHeaders.wkldHref]))
		}

		// Check if the workload and CSV value match
		if pceBool != csvBool {
			utils.LogInfo(fmt.Sprintf("CSV row %d - interface %s needs to be updated from %t to %t", rowNum+1, dataRow[csvHeaders.interfaceName], pceBool, csvBool), false)
			interfaceChangeCount++
			// If they don't match and the csvBool is true, we need to append it
			newWkld := illumioapi.Workload{}
			if val, ok := updatedWkldsMap[dataRow[csvHeaders.wkldHref]]; ok {
				newWkld = val
			} else {
				newWkld = wkldHrefMap[dataRow[csvHeaders.wkldHref]]
			}
			if csvBool {
				x := append(*newWkld.IgnoredInterfaceNames, dataRow[csvHeaders.interfaceName])
				newWkld.IgnoredInterfaceNames = &x
				updatedWkldsMap[newWkld.Href] = newWkld
			} else {
				// If they dont match and csvBool is false, we need to remove it
				updatedInterfaces := []string{}
				for _, iFace := range *newWkld.IgnoredInterfaceNames {
					if iFace == dataRow[csvHeaders.interfaceName] {
						continue
					}
					updatedInterfaces = append(updatedInterfaces, iFace)
				}
				newWkld.IgnoredInterfaceNames = &updatedInterfaces
				updatedWkldsMap[newWkld.Href] = newWkld
			}
		}
	}

	// Convert the update map to the update slice
	for _, w := range updatedWkldsMap {
		updatedWklds = append(updatedWklds, w)
	}

	// End run there are no updates required
	if interfaceChangeCount == 0 {
		utils.LogInfo("no changes identified", true)
		utils.LogEndCommand("nic-manage")
		return
	}

	// Log the results
	utils.LogInfo(fmt.Sprintf("workloader identified %d interfaces on %d workloads that require updates.", interfaceChangeCount, len(updatedWklds)), true)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo("See workloader.log for more details. To implement the changes, run again using --update-pce flag.", true)
		utils.LogEndCommand("nic-manage")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("\r\n%s [PROMPT] - Do you want to run the import to %s at %s (yes/no)?", time.Now().Format("2006-01-02 15:04:05 "), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("prompt denied to update %d workloads.", len(updatedWklds)), true)
			utils.LogEndCommand("nic-manage")
			return
		}
	}

	// Run the updates
	api, err := pce.BulkWorkload(updatedWklds, "update", true)
	for _, a := range api {
		utils.LogAPIResp("BulkWorkloadUpdate", a)
	}
	if err != nil {
		utils.LogError(fmt.Sprintf("bulk updating workloads - %s", err))
	}
	utils.LogInfo(fmt.Sprintf("bulk update workload successful for %d workloads - status code %d", len(updatedWklds), api[0].StatusCode), true)

}
