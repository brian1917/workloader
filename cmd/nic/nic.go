package nic

import (
	"fmt"
	"strconv"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debug bool
var pce illumioapi.PCE
var err error

// NICCmd produces a report of all network interfaces
var NICCmd = &cobra.Command{
	Use:   "nic",
	Short: "Export all network interfaces for all managed and unmanaged workloads.",
	Long: `
Export all network interfaces for all managed and unmanaged workloads.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		nicExport()
	},
}

func nicExport() {

	// Log start of command
	utils.LogStartCommand("nic")

	// Build our CSV data output
	data := [][]string{[]string{"wkld_hostname", "nic_name", "ignored", "address", "cidr", "ipv4_net_mask", "default_gw", "wkld_href"}}

	// Get all workloads
	wklds, a, err := pce.GetAllWorkloads()
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// For each workload, iterate through the network interfaces and add to the data slice
	for _, w := range wklds {
		for _, i := range w.Interfaces {
			// Check if the interface is ignored
			ignored := false
			for _, ignoredInt := range w.IgnoredInterfaceNames {
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
			data = append(data, []string{w.Hostname, i.Name, strconv.FormatBool(ignored), i.Address, cidr, w.GetNetMask(i.Address), i.DefaultGatewayAddress, w.Href})
		}
	}

	// Write the data
	utils.WriteOutput(data, data, fmt.Sprintf(fmt.Sprintf("workloader-nic-%s.csv", time.Now().Format("20060102_150405"))))

	// Log end of command
	utils.LogEndCommand("nic")
}
