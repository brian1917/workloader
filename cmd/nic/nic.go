package nic

import (
	"fmt"
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

The created CSV includes the following headers: wkld_hostname, wkld_href, interface_name, ip_address, net_mask, cidr, default_gw`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.Log(1, err.Error())
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		nicExport()
	},
}

func nicExport() {

	// Log start of command
	utils.Log(0, "started nic command")

	// Build our CSV data output
	data := [][]string{[]string{"wkld_hostname", "wkld_href", "nic_name", "address", "cidr", "ipv4_net_mask", "default_gw"}}

	// Get all workloads
	wklds, a, err := pce.GetAllWorkloads()
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.Log(1, err.Error())
	}

	// For each workload, iterate through the network interfaces and add to the data slice
	for _, w := range wklds {
		for _, i := range w.Interfaces {
			// Convert the CIDR
			var cidr string
			if i.CidrBlock == nil {
				cidr = ""
			} else {
				cidr = fmt.Sprintf("%s/%d", i.Address, *i.CidrBlock)
			}
			data = append(data, []string{w.Hostname, w.Href, i.Name, i.Address, cidr, w.GetNetMask(i.Address), i.DefaultGatewayAddress})
		}
	}

	// Write the data
	utils.WriteOutput(data, data, fmt.Sprintf(fmt.Sprintf("workloader-nic-%s.csv", time.Now().Format("20060102_150405"))))

	// Log end of command
	utils.Log(0, "nic command completed")
}
