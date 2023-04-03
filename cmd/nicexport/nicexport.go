package nicexport

import (
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var consolidate bool
var pce illumioapi.PCE
var err error
var outputFileName string

func init() {
	NICExportCmd.Flags().BoolVarP(&consolidate, "consolidate", "c", false, "instead of one line per IP address, the output is one line per interface name. If an interface has multiple addresses, they are separated by a semi-colon. This is the format if you plan to edit ignored status and feed into workloads's nic-manage command.")
	NICExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
}

// NICExportCmd produces a report of all network interfaces
var NICExportCmd = &cobra.Command{
	Use:   "nic-export",
	Short: "Export all network interfaces for all managed and unmanaged workloads.",
	Long: `
Export all network interfaces for all managed and unmanaged workloads.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		nicExport()
	},
}

func nicExport() {

	// Log start of command
	utils.LogStartCommand("nic-export")

	// Build our CSV data output
	headerRow := []string{"wkld_hostname", "wkld_href", "wkld_polcy_state", "nic_name", "ignored", "address", "cidr", "ipv4_net_mask", "default_gw"}
	data := [][]string{headerRow}

	// Get all workloads
	wklds, a, err := pce.GetWklds(nil)
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// For each workload, iterate through the network interfaces and add to the data slice
	for _, w := range wklds {
		for _, i := range w.Interfaces {
			// Check if the interface is ignored
			ignored := false
			for _, ignoredInt := range *w.IgnoredInterfaceNames {
				if ignoredInt == i.Name {
					ignored = true
				}
			}
			// Convert the CIDR
			var cidr string
			if i.CidrBlock == nil {
				cidr = ""
			} else {
				cidr = fmt.Sprintf("%s/%d", i.Address, *i.CidrBlock)
			}
			data = append(data, []string{w.Hostname, w.Href, w.GetMode(), i.Name, strconv.FormatBool(ignored), i.Address, cidr, w.GetNetMask(i.Address), i.DefaultGatewayAddress})
		}
	}

	type mapEntry struct {
		stringSlice []string
		order       int
	}

	if consolidate {

		interfaceMap := make(map[string]mapEntry)
		for i, d := range data {
			// Skip the header row
			if i == 0 {
				continue
			}
			// If it's already in the map, edit the fields we need to concatenate and put in the consolidateData slice
			if val, ok := interfaceMap[d[1]+d[3]]; ok {
				interfaceMap[d[1]+d[3]].stringSlice[5] = d[5] + ";" + val.stringSlice[5]
				interfaceMap[d[1]+d[3]].stringSlice[6] = d[6] + ";" + val.stringSlice[6]
				interfaceMap[d[1]+d[3]].stringSlice[7] = d[7] + ";" + val.stringSlice[7]
				// If it's not in the map, add it to the map
			} else {
				interfaceMap[d[1]+d[3]] = mapEntry{stringSlice: d, order: i}
			}
		}

		// Add it to the consolidated data. This will be unordered - I want to fix this later.
		mapEntrySlice := []mapEntry{}
		for _, e := range interfaceMap {
			mapEntrySlice = append(mapEntrySlice, e)
		}

		// Sory the slice by order
		sort.SliceStable(mapEntrySlice, func(i, j int) bool {
			return mapEntrySlice[i].order < mapEntrySlice[j].order
		})

		// Replace data
		data = nil
		data = append(data, headerRow)
		for _, i := range mapEntrySlice {
			data = append(data, i.stringSlice)
		}
	}

	// Write the data
	if outputFileName == "" {
		outputFileName = fmt.Sprintf("workloader-nic-export-%s.csv", time.Now().Format("20060102_150405"))
	}
	utils.WriteOutput(data, data, outputFileName)

	// Log end of command
	utils.LogEndCommand("nic-export")
}
