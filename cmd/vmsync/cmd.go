package vmsync

import (
	"fmt"
	"os"

	"github.com/brian1917/workloader/utils"
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
	VCenterSyncCmd.Flags().StringVarP(&vcenter, "vcenter", "v", "", "Required - FQDN or IP of VCenter instance - e.g vcenter.illumio.com")
	VCenterSyncCmd.Flags().StringVarP(&userID, "user", "u", "", "Required - username of account with access to VCenter REST API")
	VCenterSyncCmd.Flags().StringVarP(&secret, "password", "p", "", "Required - password of account with access to VCenter REST API")
	VCenterSyncCmd.Flags().StringVarP(&datacenter, "datacenter", "d", "", "Sync VMs that reside in a certain VCenter Datacenter object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "Sync VMs that reside in a certain VCenter cluster object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&folder, "folder", "f", "", "Sync VMs that reside in a certain VCenter folder object. (default - \"\"")
	VCenterSyncCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Ignore SSL certificate validation when communicating with VCenter.")
	VCenterSyncCmd.Flags().BoolVarP(&umwl, "umwl", "", false, "Import VCenter VMs that dont have worloader in the PCE.  Once imported only labels updates will occur.  No Ip address/interface updates.")
	VCenterSyncCmd.Flags().BoolVarP(&allIPs, "allintf", "a", false, "Use this flag if VM has more than one IP address")
	VCenterSyncCmd.Flags().BoolVarP(&ipv6, "ipv6", "", false, "Use this flag in additon to \"--allintf\" if you want the IPv6 address also included")
	VCenterSyncCmd.Flags().BoolVarP(&ignoreState, "ignore-state", "", false, "Currently only finds VCenter VMs 'Powered_on'")
	VCenterSyncCmd.Flags().BoolVarP(&vcName, "vcentername", "", false, "Use this flag if you want to match on VCenter VM name vs using VMTools Hostname")
	VCenterSyncCmd.Flags().BoolVarP(&keepFile, "keepfile", "", false, "Do not delete the temp CSV file downloaded from Vcenter Sync")
	VCenterSyncCmd.Flags().BoolVarP(&keepFQDNHostname, "keepfqdn", "", false, "By default hostnames will have the domain removed when matching.  This option will match with FQDN names.")
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
	Use:   "vmsync",
	Short: "Integrate Azure VMs into PCE.",
	Long: `
	Copy VCenter VM Tags with PCE workload Labels.  The command requires a CSV file that maps VCenter Categories to PCE label types.  There are options to filter the VMs from VCenter using VCenter objects(datacenter, clusters, folders, power state).  By default mtaching will use VMtools Hostname to PCE hostnames.  You can match just on the VCenter VM name.   By default hostname domains will be removed during match.  There is an option to keep the domain.
	
	There is also an UMWL option, "--umwl", that finds all VMs that do not have an existing workload(unmanaged or managed) found in the PCE.  Any VCenter VM not matching a PCE workload hostname will be considered an UMWL.  UMWL creation requires an IP address which VMTools discovers.  If VMtools is not installed the VM will be skipped. By default the tool only gets the primary IP address. There is an option to get all unique IP addresses across all interfaces on a VM via "--allintf" option.  Capturing IPv6 addresses requires both the "--allintf" and "--ipv6" options.
	
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

		utils.LogStartCommand("vcenter-sync")

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
