package edgeadmin

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/workloader/cmd/wkldimport"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debug, doNotProvision bool
var csvFile, fromPCE, toPCE, outputFileName, edgeGroup, coreApp, coreEnv, coreLoc string
var input wkldimport.Input

func init() {
	EdgeAdminCmd.Flags().StringVarP(&fromPCE, "from-pce", "f", "", "Name of the PCE with the existing Edge Group to copy over. Required")
	EdgeAdminCmd.MarkFlagRequired("from-pce")
	EdgeAdminCmd.Flags().StringVarP(&toPCE, "to-pce", "t", "", "Name of the PCE to receive Edge Admin Group endpoint info. Only required if using --update-pce flag")
	EdgeAdminCmd.Flags().StringVarP(&edgeGroup, "edge-group", "g", "", "Name of the Edge group to be copied to Core PCE")
	EdgeAdminCmd.MarkFlagRequired("edge-group")
	EdgeAdminCmd.Flags().StringVarP(&coreApp, "core-app", "a", "", "Name of the Edge group to be copied to Core PCE")
	EdgeAdminCmd.MarkFlagRequired("core-app")
	EdgeAdminCmd.Flags().StringVarP(&coreEnv, "core-env", "e", "", "Name of the Edge group to be copied to Core PCE")
	EdgeAdminCmd.MarkFlagRequired("core-env")
	EdgeAdminCmd.Flags().StringVarP(&coreLoc, "core-loc", "l", "", "Name of the Edge group to be copied to Core PCE")
	EdgeAdminCmd.MarkFlagRequired("core-loc")
	EdgeAdminCmd.Flags().BoolVarP(&doNotProvision, "do-not-prov", "x", false, "Do not provision created/updated IP Lists. Will provision by default.")
	EdgeAdminCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

}

// EdgeAdminCmd runs the upload command
var EdgeAdminCmd = &cobra.Command{
	Use:   "edge-admin ",
	Short: "Copy Edge Admin group to Core PCE for authenticated Admin Access to Core Workloads.",
	Long: `
Copy every endpoint DN infrmation within specified Edge group from Edge PCE to Core PCE. Every endpoint must have a valid ipsec certificate to be copied over.  
The Edge group once copied can be used in policy using MachineAuth option to protect access requiring certificate validated ipsec connections.  
No IP address information will be copied from Edge to Core so only MachineAuth rules will work.
`,

	Run: func(cmd *cobra.Command, args []string) {

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		// Disable stdout
		viper.Set("output_format", "csv")
		if err := viper.WriteConfig(); err != nil {
			utils.LogError(err.Error())
		}
		// Get the debug value from viper
		input.UpdatePCE = viper.Get("update_pce").(bool)
		input.NoPrompt = viper.Get("no_prompt").(bool)
		input.Umwl = true
		edgeadmin()
	},
}

func edgeadmin() {
	//Make sure to check the column names to correctly set.
	input.HostnameIndex = 99999
	input.AppIndex = 99999
	input.DatacenterIndex = 99999
	input.DescIndex = 99999
	input.EnvIndex = 99999
	input.LocIndex = 99999
	input.ExtDataRefIndex = 99999
	input.ExtDataSetIndex = 99999
	input.HrefIndex = 99999
	input.IntIndex = 99999
	input.NameIndex = 99999
	input.OSDetailIndex = 99999
	input.OSIDIndex = 99999
	input.PublicIPIndex = 99999
	input.RoleIndex = 99999
	input.MachineAuthIDIndex = 99999

	// Log start of run
	utils.LogStartCommand("edge-admin")

	// Check if we have destination PCE if we need it
	if input.UpdatePCE && toPCE == "" {
		utils.LogError("need --to-pce (-t) flag set if using update-pce")
	}

	// Get the source pce
	sPce, err := utils.GetPCEbyName(fromPCE, true)
	if err != nil {
		utils.LogError(err.Error())
	}

	//get Edge group Href for workload filter
	slabel, a, err := sPce.GetLabelbyKeyValue("role", edgeGroup)
	utils.LogAPIResp("GetLabelbyKeyValue", a)
	if err != nil {
		utils.LogError(err.Error())
	}
	//Make sure label is not empty...have user reenter if so
	if slabel.Value == "" {
		utils.LogError(fmt.Sprintf("error finding Edge group - %s.  Please reenter with exact Edge group name", edgeGroup))
	}

	queryP := map[string]string{"labels": fmt.Sprintf("[[\"%s\"]]", slabel.Href)}
	// Get all workloads from the source PCE
	swklds, a, err := sPce.GetAllWorkloadsQP(queryP)
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	if toPCE != "" {

		// Get the source pce
		input.PCE, err = utils.GetPCEbyName(toPCE, true)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting to pce - %s", err))
		}

		//Get all existing workloads already on dest PCE to check if there updates needed.
		var labelfilter []string
		var destvalues = []string{slabel.Value, coreApp, coreEnv, coreLoc}
		var destkeys = []string{"role", "app", "env", "loc"}
		for i, l := range destvalues {
			dlabel, a, err := input.PCE.GetLabelbyKeyValue(destkeys[i], l)
			utils.LogAPIResp("GetLabelbyKeyValue", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			if dlabel.Href != "" {
				labelfilter = append(labelfilter, dlabel.Href)
			}
		}
		//create filter to get all workloads that are unmanaged and already labe the group and set labels,
		tmpstr := "[[\"" + strings.Join(labelfilter, "\",\"") + "\"]]"
		queryP := map[string]string{"labels": tmpstr}
		queryP["managed"] = "false"
		// Get all workloads from the destination PCE
		dwklds, a, err := input.PCE.GetAllWorkloadsQP(queryP)
		utils.LogAPIResp("GetAllWorkloads", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		fmt.Print(dwklds)
	}

	csvOut := [][]string{{"hostname", "name", "role", "app", "env", "loc", "interfaces", "public_ip", "href", "description", "os_id", "os_detail", "datacenter", "external_data_set", "external_data_reference", "machine_authentication_id"}}
	stdOut := [][]string{{"hostname", "role", "app", "env", "loc", "distinguished_name"}}

	for _, w := range swklds {

		// Output the CSV
		if len(swklds) > 0 {
			// Skip deleted workloads
			if *w.Deleted {
				continue
			}

			// Get interfaces
			interfaces := []string{}
			for _, i := range w.Interfaces {
				ipAddress := fmt.Sprintf("%s:%s", i.Name, i.Address)
				if i.CidrBlock != nil && *i.CidrBlock != 0 {
					ipAddress = fmt.Sprintf("%s:%s/%s", i.Name, i.Address, strconv.Itoa(*i.CidrBlock))
				}
				interfaces = append(interfaces, ipAddress)
			}

			csvOut = append(csvOut, []string{w.Hostname, w.Name, w.GetRole(sPce.LabelMapH).Value, coreApp, coreEnv, coreLoc, "", w.PublicIP, w.Href, w.Description, w.OsID, w.OsDetail, w.DataCenter, w.ExternalDataSet, w.Hostname + w.Href, w.DistinguishedName})
			stdOut = append(stdOut, []string{w.Hostname, w.GetRole(sPce.LabelMapH).Value, coreApp, coreEnv, coreLoc, w.GetMode()})
		} else {
			utils.LogInfo("no Workloads created.", true)
		}
	}

	if outputFileName == "" {
		outputFileName = "workloader-edge-admin-output-" + time.Now().Format("20060102_150405") + ".csv"
	}
	input.ImportFile = outputFileName
	utils.WriteOutput(csvOut, csvOut, outputFileName)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !input.UpdatePCE {
		utils.LogInfo(fmt.Sprintln("See the output file for Workloads that would be created. Run again using --to-pce and --update-pce flags to create the IP lists."), true)
		utils.LogEndCommand("edge-admin")
		return
	}

	// If we get here, create the workloads on dest PCE using wkld-import
	utils.LogInfo(fmt.Sprintf("calling workloader edge-admin to import %s to %s", outputFileName, toPCE), true)
	wkldimport.ImportWkldsFromCSV(input)

	utils.LogEndCommand("edge-admin")
}
