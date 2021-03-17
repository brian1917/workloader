package wkldtoipl

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/cmd/iplimport"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debug, updatePCE, noPrompt, doNotProvision bool
var csvFile, fromPCE, toPCE, outputFileName string

func init() {
	WorkloadToIPLCmd.Flags().StringVarP(&fromPCE, "from-pce", "f", "", "Name of the PCE with the existing workloads. Required")
	WorkloadToIPLCmd.MarkFlagRequired("from-pce")
	WorkloadToIPLCmd.Flags().StringVarP(&toPCE, "to-pce", "t", "", "Name of the PCE to create or update the IPlists from the workloads. Only required if using --update-pce flag")
	WorkloadToIPLCmd.Flags().BoolVarP(&doNotProvision, "do-not-prov", "x", false, "Do not provision created/updated IP Lists. Will provision by default.")
	WorkloadToIPLCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

}

// WorkloadToIPLCmd runs the upload command
var WorkloadToIPLCmd = &cobra.Command{
	Use:   "wkld-to-ipl [csv file]",
	Short: "Create IP lists based on workloads labels and input file.",
	Long: `
Create IP lists in a PCE (--to-pce, -t) based on workload labels in a different PCE (--from-pce, -f) and an input file.

The input file must match the following format and include headers (all columns must be present):
+-------+------+---------+----------+-----+
| name  | role |   app   |   env    | loc |
+-------+------+---------+----------+-----+
| IPL-1 | WEB  | CRM     | PROD     | BOS |
| IPL-2 |      | CRM;ERP | PROD;DEV | BOS |
|       |      |         | DEV      | BOS |
+-------+------+---------+----------+-----+

Note - if the name is left blank, the name will be the provided labels separated by dashes. See example 3 below.

Examples:
1) The first row will create an IPList named IPL-1 with all servers that are lableled WEB, CRM, PROD, and BOS.
2) The second row will create an IPList named IPL-2 with all servers that are labeled (CRM or ERP) and (PROD or DEV).
3) the third row will create an IPList named DEV-BOS with all servers that are labeled DEV, and BOS.

This command creates a CSV that is automatically passed into workloader ipl-import.
`,

	Run: func(cmd *cobra.Command, args []string) {

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		// Disable stdout
		viper.Set("output_format", "csv")
		if err := viper.WriteConfig(); err != nil {
			utils.LogError(err.Error())
		}

		wkldtoipl()
	},
}

func wkldtoipl() {

	// Log start of run
	utils.LogStartCommand("wkld-to-ipl")

	// Check if we have destination PCE if we need it
	if updatePCE && toPCE == "" {
		utils.LogError("need --to-pce (-t) flag set if using update-pce")
	}

	// Get the source pce
	sPce, err := utils.GetPCEbyName(fromPCE, true)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get all workloads from the source PCE
	wklds, a, err := sPce.GetAllWorkloads()
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Parse the input csv
	csvData, err := utils.ParseCSV(csvFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create a slice to hold IP Lists
	ipls := []illumioapi.IPList{}

	// Iterate the CSV file
	for row, d := range csvData {

		// Skip header
		if row == 0 {
			continue
		}

		// Parse the provided labels by separating at the semicolon
		roleMap := make(map[string]int)
		appMap := make(map[string]int)
		envMap := make(map[string]int)
		locMap := make(map[string]int)

		mapList := []map[string]int{roleMap, appMap, envMap, locMap}
		for i := 0; i < 4; i++ {
			for _, l := range strings.Split(d[i+1], ";") {
				mapList[i][l] = 1
			}
		}

		// Get the name
		name := d[0]
		if name == "" {
			labels := []string{}
			for i := 1; i < 5; i++ {
				if d[i] != "" {
					labels = append(labels, strings.Split(d[i], ";")...)
				}
			}
			name = strings.Join(labels, "-")
		}

		// Create a map to check IP addresses so we don't duplicate
		addresses := make(map[string]int)
		// Create the IP list
		ipl := illumioapi.IPList{Name: name}
	workloads:
		for _, w := range wklds {
			// Check each label to see if matches
			labels := []string{w.GetRole(sPce.Labels).Value, w.GetApp(sPce.Labels).Value, w.GetEnv(sPce.Labels).Value, w.GetLoc(sPce.Labels).Value}
			for i, l := range labels {
				if d[i+1] != "" && mapList[i][l] != 1 {
					continue workloads
				}
			}
			// Only get here the workload should be included
			// Iterate through each nic on the workload and add it to the IPList
			for _, nic := range w.Interfaces {
				// Check the address if it's been put in already
				if _, ok := addresses[nic.Address]; !ok {
					// Add it to the map
					addresses[nic.Address] = 1
					// Add it to the IPList range
					ipl.IPRanges = append(ipl.IPRanges, &illumioapi.IPRange{FromIP: nic.Address})
				}
			}
		}
		// Add the slice if we have some ip ranges in
		if len(ipl.IPRanges) > 0 {
			ipls = append(ipls, ipl)
		}
	}

	// Output the CSV
	if len(ipls) > 0 {
		csvOut := [][]string{[]string{"name", "description", "include", "exclude", "external_ds", "external_ds_ref"}}
		for _, i := range ipls {
			// Build the include string
			includes := []string{}
			for _, ip := range i.IPRanges {
				if ip.Exclusion {
					continue
				}
				includes = append(includes, ip.FromIP)
			}
			csvOut = append(csvOut, []string{i.Name, i.Description, strings.Join(includes, ";"), "", "", ""})
		}

		if outputFileName == "" {
			outputFileName = "workloader-wkld-to-ipl-output-" + time.Now().Format("20060102_150405") + ".csv"
		}
		utils.WriteOutput(csvOut, csvOut, outputFileName)

		// If updatePCE is disabled, we are just going to alert the user what will happen and log
		if !updatePCE {
			utils.LogInfo(fmt.Sprintln("See the output file for IP Lists that would be created. Run again using --to-pce and --update-pce flags to create the IP lists."), true)
			utils.LogEndCommand("wkld-to-ipl")
			return
		}

		// If we get here, create the IP lists in the destination PCE using ipl-import
		utils.LogInfo(fmt.Sprintf("calling workloader ipl-import to import %s to %s", outputFileName, toPCE), true)
		dPce, err := utils.GetPCEbyName(toPCE, false)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting to pce - %s", err))
		}
		iplimport.ImportIPLists(dPce, outputFileName, updatePCE, noPrompt, debug, !doNotProvision)
	} else {
		utils.LogInfo("no IP lists created.", true)
	}

	utils.LogEndCommand("wkld-to-ipl")
}
