package vmsync

import (
	"fmt"
	"os"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var vcenter, datacenter, cluster, folder, userID, secret string

var csvFile string
var ignoreState, umwl, keepTempFile, keepFQDNHostname, deprecated, insecure, allIPs, vcName bool
var updatePCE, noPrompt bool
var pce illumioapi.PCE
var vc VCenter
var maxCreate, maxUpdate int
var err error

// Init builds the commands
func init() {

	// Disable sorting
	cobra.EnableCommandSorting = false

	//awsimport options
	VCenterSyncCmd.Flags().StringVarP(&datacenter, "datacenter", "d", "", "Sync VMs that reside in a certain VCenter Datacenter object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "Sync VMs that reside in a certain VCenter cluster object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&folder, "folder", "f", "", "Sync VMs that reside in a certain VCenter folder object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&vcenter, "vcenter", "v", "", "Required - FQDN or IP of VCenter instance - e.g vcenter.illumio.com")
	VCenterSyncCmd.Flags().StringVarP(&userID, "user", "u", "", "Required - username of account with access to VCenter REST API")
	VCenterSyncCmd.Flags().StringVarP(&secret, "password", "p", "", "Required - password of account with access to VCenter REST API")
	VCenterSyncCmd.Flags().BoolVarP(&ignoreState, "ignore-state", "i", false, "Currently only finds VCenter VMs in a 'RunningState'")
	VCenterSyncCmd.Flags().BoolVarP(&umwl, "umwl", "", false, "Import VCenter VMs that dont have worloader in the PCE.  Once imported these will only update labels.")
	VCenterSyncCmd.Flags().BoolVarP(&keepTempFile, "keep-temp-file", "k", false, "Do not delete the temp CSV file downloaded from Vcenter Sync")
	VCenterSyncCmd.Flags().BoolVarP(&keepFQDNHostname, "keepfqdn", "", false, "By default hostnames will have the domain removed when matching.  This option will keep FQDN hostnames (e.g., test.domain.com will not become test). ")
	VCenterSyncCmd.Flags().BoolVarP(&allIPs, "allintf", "a", false, "Use this flag if VM has more than one IP address")
	VCenterSyncCmd.Flags().BoolVarP(&vcName, "vcname", "", false, "Use this flag if you want to match on VCenter VM name vs using VMTools Hostname")
	VCenterSyncCmd.Flags().BoolVarP(&insecure, "insecure", "", false, "Ignore SSL certificate validation when communicating with PAN.")
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
	Long: `Sync VCenter VM Tags with PCE workload Labels.  The command requires a CSV file that maps VCenter Categories to PCE label types.  There are options to filter the VMs from VCenter using VCenter objects(datacenter, clusters, folders, power state).  You have an option to match PCE hostnames with VM hostnames discovered via VMTools(must be installed) or you can match just on the VCenter VM name.   By default hostname domains will be removed to match with VCenter VMs.  There is an option to keep the domain.
	
	There is also an UMWL option to find all VMs that do not have an existing workload(unmanaged or managed) found in the PCE.  Any VCenter VM not matching a PCE workload will	be considered as an UMWL.  UMWL creation requires an IP address and if VMtools is not installed on the VM this tool cannot discover the IP. By default only a single IP that is shown in the VCenter display is used. There is an option to get all interfaces and all IPs.
	
	Support VCenter version > 7.0.u2`,

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

		vc.KeyMap = keyMap
		vc.VCenterURL = vcenter
		vc.User = userID
		vc.Secret = secret
		vc.DisableTLSChecking = insecure
		vc.Header = make(map[string]string)

		vc.buildVCTagMap(keyMap)

		vc.compileVMData()
		//Sync VMs to Workloads or create UMWL VMs for all machines in VCenter not running VEN
		buildWkldImport(keyMap, &pce)
	},
}

// buildWkldImport - Function that gets the data structure to build a wkld import file and import.
func buildWkldImport(keyMap map[string]string, pce *illumioapi.PCE) {

	var outputFileName string
	// Set up the csv headers
	csvData := [][]string{{"hostname", "description"}}
	if umwl {
		csvData[0] = append(csvData[0], "interfaces")
	}
	for _, illumioLabelType := range keyMap {
		csvData[0] = append(csvData[0], illumioLabelType)
	}

	//csvData := [][]string{{"hostname", "role", "app", "env", "loc", "interfaces", "name"}
	for _, vm := range vc.VCVMs {
		csvRow := []string{vm.Name, vm.VMID}
		var tmpInf string
		if umwl {
			for c, inf := range vm.Interfaces {
				if c != 0 {
					tmpInf = tmpInf + ";"
				}
				tmpInf = tmpInf + fmt.Sprintf("%s:%s", inf[0], inf[1])
			}
			csvRow = append(csvRow, tmpInf)
		}
		for index, header := range csvData[0] {

			// Skip hostname and interfaces if umwls ...they are statically added above
			if index < 2 {
				continue
			} else if umwl && index == 2 {
				continue
			}
			//process hostname by finding Name TAG
			csvRow = append(csvRow, vm.Tags[header])
		}
		csvData = append(csvData, csvRow)
	}

	if len(vc.VCVMs) <= 0 {
		utils.LogInfo("no Vcenter vms found", true)
	} else {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-vcenter-sync-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d VCenter vms with label data exported", len(csvData)-1), true)

		utils.LogInfo("passing output into wkld-import...", true)

		wkldimport.ImportWkldsFromCSV(wkldimport.Input{
			PCE:             *pce,
			ImportFile:      outputFileName,
			RemoveValue:     "vcenter-label-delete",
			Umwl:            umwl,
			UpdateWorkloads: true,
			UpdatePCE:       updatePCE,
			NoPrompt:        noPrompt,
			MaxUpdate:       maxUpdate,
			MaxCreate:       maxCreate,
		})

		// Delete the temp file
		if !keepTempFile {
			if err := os.Remove(outputFileName); err != nil {
				utils.LogWarning(fmt.Sprintf("Could not delete %s", outputFileName), true)
			} else {
				utils.LogInfo(fmt.Sprintf("Deleted %s", outputFileName), false)
			}
		}
	}

	utils.LogEndCommand(fmt.Sprintf("%s-sync", "vcenter"))
}
