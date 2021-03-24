package dupecheck

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pce illumioapi.PCE
var caseSensitive, debug bool
var outputFileName string
var err error

func init() {
	DupeCheckCmd.Flags().BoolVarP(&caseSensitive, "case-sensitive", "c", false, "Require hostname/name matches to be case-sensitve.")
	DupeCheckCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	DupeCheckCmd.Flags().SortFlags = false
}

// DupeCheckCmd summarizes flows
var DupeCheckCmd = &cobra.Command{
	Use:   "dupecheck",
	Short: "Identifies duplicate hostnames and IP addresses in the PCE.",
	Long: `
Identifies unmanaged workloads with hostnames, names, or IP addresses also assigned to managed workloads.

Interfaces with the default gateway are used on managed workloads.

The --update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		dupeCheck()
	},
}

func dupeCheck() {
	utils.LogStartCommand("dupecheck")

	// Get all workloads
	wklds, a, err := pce.GetAllWorkloads()
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get all managed workloads
	managedHostNameMap := make(map[string]illumioapi.Workload)
	managedIPAddressMap := make(map[string]illumioapi.Workload)
	unmanagedWklds := []illumioapi.Workload{}
	for _, w := range wklds {
		if w.GetMode() == "unmanaged" {
			unmanagedWklds = append(unmanagedWklds, w)
		} else {
			if caseSensitive {
				managedHostNameMap[w.Hostname] = w
			} else {
				managedHostNameMap[strings.ToLower(w.Hostname)] = w
			}
			managedIPAddressMap[w.GetIPWithDefaultGW()] = w
		}
	}

	// Start the header
	data := [][]string{[]string{"href", "hostname", "name", "interfaces", "role", "app", "env", "loc", "reason"}}

	// Iterate through unmanaged workloads
	for _, umwl := range unmanagedWklds {
		// Start our reason list. If this has a length 0 at the end, we don't have a dupe.
		reason := []string{}

		// Check managed hostnames
		hostname := strings.ToLower(umwl.Hostname)
		if caseSensitive {
			hostname = umwl.Hostname
		}
		if val, ok := managedHostNameMap[hostname]; ok {
			reason = append(reason, fmt.Sprintf("unmanaged workload hostname matches with hostname of managed workload %s", val.Href))
		}

		// Check managed names
		name := strings.ToLower(umwl.Name)
		if caseSensitive {
			name = umwl.Name
		}
		if val, ok := managedHostNameMap[name]; ok {
			reason = append(reason, fmt.Sprintf("unmanaged workload name matches with hostname of managed workload %s", val.Href))
		}

		// Check interfaces - all must exist in managed workloads to count.
		ifaceMatchCount := 0
		matches := []string{}
		interfaceList := []string{}
		for _, iface := range umwl.Interfaces {
			interfaceList = append(interfaceList, fmt.Sprintf("%s:%s", iface.Name, iface.Address))
			if val, ok := managedIPAddressMap[iface.Address]; ok {
				ifaceMatchCount++
				matches = append(matches, val.Href)
			}
		}
		if ifaceMatchCount == len(umwl.Interfaces) {
			reason = append(reason, fmt.Sprintf("unmanaged interfaces match to managed IP addresses of %s", strings.Join(matches, "; ")))
		}

		// If we have reasons, append
		if len(reason) > 0 {
			data = append(data, []string{umwl.Href, umwl.Hostname, umwl.Name, strings.Join(interfaceList, ";"), umwl.GetRole(pce.Labels).Value, umwl.GetApp(pce.Labels).Value, umwl.GetEnv(pce.Labels).Value, umwl.GetLoc(pce.Labels).Value, strings.Join(reason, ";")})
		}

	}
	// Write the output
	if len(data) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-dupecheck-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(data, data, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d unmanaged workloads found. See %s for output. The output file can be used as input to workloader delete command.", len(data)-1, outputFileName), true)
	} else {
		utils.LogInfo("No duplicates found", true)
	}

	// Log End
	utils.LogEndCommand("dupecheck")
}
