package vmsync

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var vcenter, datacenter, cluster, folder, userID, secret string

var csvFile string
var ignoreState, umwl, keepFile, keepFQDNHostname, deprecated, insecure, allIPs, vcName, ipv6 bool
var updatePCE, noPrompt bool
var vc VCenter
var maxCreate, maxUpdate int

// Init builds the commands
func init() {

	// Disable sorting
	cobra.EnableCommandSorting = false

	//awsimport options
	VCenterSyncCmd.Flags().StringVarP(&vcenter, "vcenter", "v", "", "fqdn or ip of vcenter instance (e.g., vcenter.illumio.com)")
	VCenterSyncCmd.Flags().StringVarP(&userID, "user", "u", "", "vcenter username with access to vcenter api")
	VCenterSyncCmd.Flags().StringVarP(&secret, "password", "p", "", "vcenter password with access to vcenter api")
	VCenterSyncCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "ignore vcenter ssl certificate validation.")
	VCenterSyncCmd.Flags().StringVarP(&datacenter, "datacenter", "d", "", "limit sync a specific vcenter catacenter.")
	VCenterSyncCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "limit sync to a specific vcenter cluster.")
	VCenterSyncCmd.Flags().StringVarP(&folder, "folder", "f", "", "limit sync to a specific vcenter folder.")
	VCenterSyncCmd.Flags().BoolVarP(&umwl, "umwl", "", false, "create unmanaged workloads for VMs that do not exist in the PCE. future updtaes will only update labels.")
	VCenterSyncCmd.Flags().BoolVarP(&allIPs, "all-int", "a", false, "enable syncing multiple interfaces")
	VCenterSyncCmd.Flags().BoolVarP(&ipv6, "ipv6", "", false, "used with all-int to sync ipv6 addresses.")
	VCenterSyncCmd.Flags().BoolVarP(&ignoreState, "ignore-state", "", false, "sync workloads in states other than \"Powered_on\".")
	VCenterSyncCmd.Flags().BoolVarP(&vcName, "vcentername", "", false, "match on vcenter VM name vs. using VMTools hostname.")
	VCenterSyncCmd.Flags().BoolVarP(&keepFile, "keep-file", "", false, "keep the csv file used for assigning labels.")
	VCenterSyncCmd.Flags().BoolVarP(&keepFQDNHostname, "keep-fqdn", "", false, "keep fqdn vs. removing the domain from the hostname.")
	//VCenterSyncCmd.Flags().BoolVarP(&deprecated, "deprecated", "", false, "Use this option if you are running an older version of the API (VCenter 6.5-7.0.u2")
	VCenterSyncCmd.Flags().IntVar(&maxCreate, "max-create", -1, "maximum number of unmanaged workloads that can be created. -1 is unlimited.")
	VCenterSyncCmd.Flags().IntVar(&maxUpdate, "max-update", -1, "maximum number of workloads that can be updated. -1 is unlimited.")

	VCenterSyncCmd.MarkFlagRequired("userID")
	VCenterSyncCmd.MarkFlagRequired("secret")
	VCenterSyncCmd.MarkFlagRequired("vcenter")
	VCenterSyncCmd.Flags().SortFlags = false

}

// VCenterSyncCmd checks if the keyfilename is entered.
var VCenterSyncCmd = &cobra.Command{
	Use:   "vmsync [csv with mapping]",
	Short: "Sync VCenter VM tags with PCE workload labels.",
	Long: `
Sync VCenter VM tags with PCE workload labels.

The first argument is the location of a csv file that maps VCenter Categories to PCE label keys. The csv skips the first row for headers. The VCenter category should be in the first column and the corredsponding illumio label key in the second.
	
Support VCenter version > 7.0.u2`,

	Run: func(cmd *cobra.Command, args []string) {

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		if (!umwl && (allIPs || ipv6)) || (umwl && (ipv6 && !allIPs)) {
			fmt.Println("Cannot use \"--allintf\" or \"--ipv6\" without \"--uwml\" with \"vmsync\".  \"--ipv6\" requires \"--allintf\"")
			os.Exit(0)
		}
		//Get the debug value from viper
		//debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		//load keymapfile, This file will have the Catagories to Label Type mapping
		keyMap := readKeyFile(csvFile)

		vc.KeyMap = keyMap
		vc.VCenterURL = vcenter
		vc.User = userID
		vc.Secret = secret
		vc.DisableTLSChecking = insecure
		vc.Header = make(map[string]string)

		vc.setupVCenterSession()

		vc.compileVMData(keyMap)
		//Sync VMs to Workloads or create UMWL VMs for all machines in VCenter not running VEN

	},
}
