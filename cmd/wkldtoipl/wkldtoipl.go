package wkldtoipl

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/workloader/cmd/iplimport"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var incRole, incApp, incEnv, incLoc, debug, updatePCE, noPrompt bool
var fromPCE, toPCE string

func init() {
	WorkloadToIPLCmd.Flags().BoolVarP(&incRole, "role", "r", false, "Include role in workload aggregation.")
	WorkloadToIPLCmd.Flags().BoolVarP(&incApp, "app", "a", false, "Include app in workload aggregation.")
	WorkloadToIPLCmd.Flags().BoolVarP(&incEnv, "env", "e", false, "Include env in workload aggregation.")
	WorkloadToIPLCmd.Flags().BoolVarP(&incLoc, "loc", "l", false, "Include loc in workload aggregation.")
	WorkloadToIPLCmd.Flags().StringVarP(&fromPCE, "from-pce", "f", "", "Name of the PCE with the existing workloads. Required")
	WorkloadToIPLCmd.MarkFlagRequired("from-pce")
	WorkloadToIPLCmd.Flags().StringVarP(&toPCE, "to-pce", "t", "", "Name of the PCE to create or update the IPlists from the workloads. Only required if using --update-pce flag")
}

// WorkloadToIPLCmd runs the upload command
var WorkloadToIPLCmd = &cobra.Command{
	Use:   "wkld-to-ipl",
	Short: "Create IP lists based on workloads labels.",
	Long: `
Create IP lists based on workloads labels.

The --role (-r), --app (-r), --env (-e), and --loc (-l) flags are used to specify how to aggregate the IP lists.

Examples:
workloader wkld-to-ipl -rael -f default-pce will create an IP list for every unique role-app-env-loc combination (csv-only).
workloader wkld-to-ipl -e -f default-pce will create an IP list for every envrionment (csv-only).
workloader wkld-to-ipl rae -f default-pce -t endpoint-pce --update-pce will create an IP list for every unique role-app-env combination and create the IP lists in the endpoint-pce after a user propt.`,

	Run: func(cmd *cobra.Command, args []string) {

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
	utils.LogInfo("started wkld-to-ipl command")

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

	// Create the map of IPLists
	ipAddressMap := make(map[string][]string)

	// Iterate through each workload
	count := 0
	for _, w := range wklds {

		keyVals := []string{}

		// Create the map key. If a workload is missing an included label, it's skipped and logged.
		if incRole {
			if w.GetRole(sPce.LabelMapH).Value == "" {
				utils.LogInfo(fmt.Sprintf("skipping %s because does not have role label", w.Hostname))
				continue
			}
			keyVals = append(keyVals, w.GetRole(sPce.LabelMapH).Value)
		}
		if incApp {
			if w.GetApp(sPce.LabelMapH).Value == "" {
				utils.LogInfo(fmt.Sprintf("skipping %s because does not have app label", w.Hostname))
				continue
			}
			keyVals = append(keyVals, w.GetApp(sPce.LabelMapH).Value)
		}
		if incEnv {
			if w.GetEnv(sPce.LabelMapH).Value == "" {
				utils.LogInfo(fmt.Sprintf("skipping %s because does not have env label", w.Hostname))
				continue
			}
			keyVals = append(keyVals, w.GetEnv(sPce.LabelMapH).Value)
		}
		if incLoc {
			if w.GetLoc(sPce.LabelMapH).Value == "" {
				utils.LogInfo(fmt.Sprintf("skipping %s because does not have location label", w.Hostname))
				continue
			}
			keyVals = append(keyVals, w.GetLoc(sPce.LabelMapH).Value)
		}
		key := strings.Join(keyVals, " | ")
		if key == "" {
			key = "All servers"
		}

		// Get the list of IP addresses
		ipAddresses := []string{}
		for _, nic := range w.Interfaces {
			ipAddresses = append(ipAddresses, nic.Address)
		}
		count = count + len(ipAddresses)

		// Check if the key exists in the map. If it does, append to value. If not, create
		if val, ok := ipAddressMap[key]; ok {
			ipAddressMap[key] = append(val, ipAddresses...)
		} else {
			ipAddressMap[key] = ipAddresses
		}

	}

	// Output the CSV
	csvData := [][]string{[]string{"name", "description", "include", "exclude", "external_ds", "external_ds_ref"}}
	for name, i := range ipAddressMap {
		csvData = append(csvData, []string{name, "Created from workloader wkld-to-ipl", strings.Join(i, ";"), "", "", ""})
	}

	fileName := "workloader-wkld-to-ipl-output-" + time.Now().Format("20060102_150405") + ".csv"
	utils.WriteOutput(csvData, csvData, fileName)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		fmt.Printf("%d iplists identifed with %d ip addresses. See %s for details. To create the IPlists in another PCE, run again using --update-pce flag. The --no-prompt flag will bypass the prompt if used with --update-pce.\r\n", len(ipAddressMap), count, fileName)
		utils.LogInfo("completed running wkld-to-ipl command")
		return
	}

	// If we get here, create the IP lists in the destination PCE using ipl-import
	fmt.Printf("[INFO] calling workloader ipl-import to import %s to %s\r\n", fileName, toPCE)
	dPce, err := utils.GetPCEbyName(toPCE, false)
	if err != nil {
		utils.LogError(fmt.Sprintf("error getting to pce - %s", err))
	}
	iplimport.ImportIPLists(dPce, fileName, updatePCE, noPrompt, debug)
}
