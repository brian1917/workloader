package netscalersync

import (
	"github.com/brian1917/illumioapi"
	"github.com/brian1917/ns"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Global variables
var pce illumioapi.PCE
var netscaler ns.NetScaler
var externalDataSet string
var cleanup, updatePCE, noPrompt bool
var err error

func init() {

	NetScalerSyncCmd.Flags().StringVarP(&netscaler.Server, "netscaler-server", "n", "", "netscaler server in format server.com:8080")
	NetScalerSyncCmd.Flags().StringVarP(&netscaler.User, "netscaler-user", "u", "", "netscaler user")
	NetScalerSyncCmd.Flags().StringVarP(&netscaler.Password, "netscaler-pwd", "p", "", "netscaler password")
	NetScalerSyncCmd.Flags().StringVarP(&externalDataSet, "externalDataSet", "e", "workloader-netscaler-sync", "external data set")
	NetScalerSyncCmd.Flags().BoolVarP(&cleanup, "cleanup", "c", true, "clean up virtual services (VIPs) and unmanaged workloads (SNAT IPs) in external data set that are no longer in netscaler.")
	NetScalerSyncCmd.Flags().SortFlags = false

}

// NetScalerSyncCmd runs the NetScalerSync command
var NetScalerSyncCmd = &cobra.Command{
	Use:   "netscaler-sync",
	Short: "Create an Illumio Virtual Service for each Citrix virtual server and an unmanaged workload for each SNAT IP.",
	Long: `
Create an Illumio Virtual Service for each Citrix virtual server and an unmanaged workload for each SNAT IP.

This version only supports single IP VIPs.

Recommended to run without --update-pce first to log of what will change.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Login in to the netscaler
		_, err := netscaler.Login()
		if err != nil {
			utils.LogError(err.Error())
		}

		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		utils.LogWarning("this command has been marked for deprecation. please open an issue on GitHub if you use it and want it preserved.", true)

		nsSync(pce, netscaler)
	},
}
