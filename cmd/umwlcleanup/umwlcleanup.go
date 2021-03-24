package umwlcleanup

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug bool
var outFormat, outputFileName string

func init() {
	UMWLCleanUpCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

}

// UMWLCleanUpCmd runs the workload identifier
var UMWLCleanUpCmd = &cobra.Command{
	Use:   "umwl-cleanup",
	Short: "Create a CSV that identifies unmanaged workloads and managed workloads that have the same IP address",
	Long: `
Create a CSV that identifies unmanaged workloads and managed workloads that have the same IP address.

This command will help in the situation where you have created and labeled unmanaged workloads and later installed VENs on those workloads.

The unmanaged workload IP address is compared to managed workload's NIC with the default gateway. If an unmanaged workload has multiple IP addresses, the managed workload must contain all of them.

To label the managed workloads with the same labels on the matched unmanaged workload, the output file can be directly passed into the wkld-import command.

Additionally, the output can be passed into the delete command with the --header flag set to umwl_href to delete the no longer needed unmanaged workloads.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the viper values
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		umwlCleanUp()
	},
}

func umwlCleanUp() {

	// Log start of command
	utils.LogStartCommand("umwl-cleanup")

	// Get all workloads
	wklds, a, err := pce.GetAllWorkloadsQP(nil)
	utils.LogAPIResp("GetAllWorkloadsQP", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create the maps
	umwlDefaultIPMap := make(map[string]illumioapi.Workload)
	managedDefaultIPMap := make(map[string]illumioapi.Workload)
	allManagedIPMap := make(map[string]illumioapi.Workload)

	// Populate the maps
	for _, w := range wklds {
		if w.GetMode() == "unmanaged" {
			for _, i := range w.Interfaces {
				umwlDefaultIPMap[i.Address] = w
			}
		} else {
			for _, i := range w.Interfaces {
				allManagedIPMap[i.Address] = w
			}
			if w.GetIPWithDefaultGW() != "NA" {
				managedDefaultIPMap[w.GetIPWithDefaultGW()] = w
			}
		}
	}

	// Start our data slice
	data := [][]string{[]string{"managed_hostname", "umwl_hostname", "umwl_name", "managed_interfaces", "umwl_interfaces", "managed_role", "umwl_role", "managed_app", "umwl_app", "managed_env", "umwl_env", "managed_loc", "umwl_loc", "umwl_href", "managed_href", "href", "role", "app", "env", "loc"}}

	// Find managed workloads that have the same IP address of an unmanaged workload
workloads:
	for ipAddress, managedWkld := range managedDefaultIPMap {
		if umwl, check := umwlDefaultIPMap[ipAddress]; check {

			// Hit here if there's a match. First, check if all IPs match
			// Get IP strings
			umwlIPs, managedIPs := []string{}, []string{}
			for _, i := range umwl.Interfaces {
				if allManagedIPMap[i.Address].Href != managedWkld.Href {
					umwlIdentifier := []string{}
					if umwl.Hostname != "" {
						umwlIdentifier = append(umwlIdentifier, fmt.Sprintf("hostname: %s", umwl.Hostname))
					}
					if umwl.Name != "" {
						umwlIdentifier = append(umwlIdentifier, fmt.Sprintf("name: %s", umwl.Name))
					}
					utils.LogWarning(fmt.Sprintf("Unmanaged workload - %s - has multiple IP addresses. At least one matches managed workload %s, but others do not. Skipping.", strings.Join(umwlIdentifier, ";"), managedWkld.Hostname), true)
					continue workloads
				}
				umwlIPs = append(umwlIPs, fmt.Sprintf("%s:%s", i.Name, i.Address))
			}
			for _, i := range managedWkld.Interfaces {
				managedIPs = append(managedIPs, fmt.Sprintf("%s:%s", i.Name, i.Address))
			}
			//
			data = append(data, []string{managedWkld.Hostname, umwl.Hostname, umwl.Name, strings.Join(managedIPs, ";"), strings.Join(umwlIPs, ";"), managedWkld.GetRole(pce.Labels).Value, umwl.GetRole(pce.Labels).Value, managedWkld.GetApp(pce.Labels).Value, umwl.GetApp(pce.Labels).Value, managedWkld.GetEnv(pce.Labels).Value, umwl.GetEnv(pce.Labels).Value, managedWkld.GetLoc(pce.Labels).Value, umwl.GetLoc(pce.Labels).Value, umwl.Href, managedWkld.Href, managedWkld.Href, umwl.GetRole(pce.Labels).Value, umwl.GetApp(pce.Labels).Value, umwl.GetEnv(pce.Labels).Value, umwl.GetLoc(pce.Labels).Value})
		}
	}

	// Write the output
	if len(data) > 0 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-umwl-cleanup-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.LogInfo(fmt.Sprintf("%d matches found", len(data)-1), true)
		utils.WriteOutput(data, data, outputFileName)
	}

	// Log end of command
	utils.LogStartCommand("umwl-cleanup")
}
