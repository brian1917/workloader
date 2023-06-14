package vmsync

import (
	"fmt"
	"os"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var vcenter, datacenter, cluster, folder, userID, secret string

var csvFile string
var ignoreState, umwl, updatePCE, noPrompt, keepTempFile, ignoreFQDNHostname, deprecated bool
var pce illumioapi.PCE
var err error

// Init builds the commands
func init() {

	// Disable sorting
	cobra.EnableCommandSorting = false

	//awsimport options
	VCenterSyncCmd.Flags().StringVarP(&datacenter, "datacenter", "d", "", "Sync VMs that reside in certain VCenter Datacenter object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "Sync VMs that reside in certain VCenter cluster object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&folder, "folder", "f", "", "Sync VMs that reside in certain VCenter folder object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&vcenter, "vcenter", "v", "", "Required - FQDN or IP of VCenter instance - e.g vcenter.illumio.com")
	VCenterSyncCmd.Flags().StringVarP(&userID, "user", "u", "", "Required - username of account with access to VCenter REST API")
	VCenterSyncCmd.Flags().StringVarP(&secret, "password", "p", "", "Required - password of account with access to VCenter REST API")
	VCenterSyncCmd.Flags().BoolVarP(&ignoreState, "ignore-state", "i", false, "By default only looks for workloads in a running state")
	VCenterSyncCmd.Flags().BoolVar(&umwl, "umwl", false, "Create unmanaged workloads for non-matches.")
	VCenterSyncCmd.Flags().BoolVarP(&keepTempFile, "keep-temp-file", "k", false, "Do not delete the temp CSV file downloaded from SerivceNow.")
	VCenterSyncCmd.Flags().BoolVarP(&ignoreFQDNHostname, "ignore-name-clean", "", false, "Convert FQDN hostnames reported by Illumio VEN to short hostnames by removing everything after first period (e.g., test.domain.com becomes test). ")
	VCenterSyncCmd.Flags().BoolVarP(&deprecated, "deprecated", "", false, "use this option if you are running an older version of the API (VCenter 6.5-7.0.u2")
	VCenterSyncCmd.MarkFlagRequired("userID")
	VCenterSyncCmd.MarkFlagRequired("secret")
	VCenterSyncCmd.MarkFlagRequired("vcenter")
	VCenterSyncCmd.Flags().SortFlags = false

}

// VCenterSyncCmd checks if the keyfilename is entered.
var VCenterSyncCmd = &cobra.Command{
	Use:   "vmsync",
	Short: "Integrate Azure VMs into PCE.",
	Long: `Sync VCenter VM Tags with PCE workload Labels.  The command requires a CSV file that maps VCenter Categories to PCE label types.
	There are options to filter the VMs from VCenter using VCenter objects(datacenter, clusters, folders, power state).  PCE hostnames and VM names
	are used to match PCE workloads to VCenter VMs.   There is an option to remove a FQDN hostname doamin to match with the VM name in VCenter
	
	There is also an UMWL option to find all VMs that are not running the Illumio VEN.  Any VCenter VM no matching a PCE workload will
	be considered as an UMWL.  To correctly configure the IP address for these UWML VMTools should be installed to pull that data from the 
	API.`,

	Run: func(cmd *cobra.Command, args []string) {

		//Get all the PCE data
		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(fmt.Sprintf("Error getting PCE - %s", err.Error()))
		}
		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		//Get the debug value from viper
		//debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		utils.LogStartCommand("vcenter-sync")

		//load keymapfile, This file will have the Catagories to Label Type mapping
		keyMap := readKeyFile(csvFile)

		//Make sure the keyMap file doesnt have incorrect labeltypes.  Exit if it does.
		validateKeyMap(keyMap)

		//Sync VMs to Workloads or create UMWL VMs for all machines in VCenter not running VEN
		callWkldImport(keyMap, &pce, vcenterBuildPCEInputData(keyMap))
	},
}
