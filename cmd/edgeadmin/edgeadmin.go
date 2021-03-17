package edgeadmin

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brian1917/workloader/cmd/delete"
	"github.com/brian1917/workloader/cmd/wkldimport"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debug, doNotProvision, keepTempFile, delStaleUMWL bool
var csvFile, fromPCE, toPCE, outputFileName, edgeGroup, coreApp, coreEnv, coreLoc, refHeader string
var input wkldimport.Input

func init() {
	EdgeAdminCmd.Flags().StringVarP(&fromPCE, "from-pce", "f", "", "Name of the PCE with the existing Edge Group to copy to destination PCE. Required")
	EdgeAdminCmd.MarkFlagRequired("from-pce")
	EdgeAdminCmd.Flags().StringVarP(&toPCE, "to-pce", "t", "", "Name of the PCE to import Edge Admin Group endpoints as UMWL. Only required if using --update-pce flag")
	EdgeAdminCmd.Flags().StringVarP(&edgeGroup, "edge-group", "g", "", "Name of the Edge group to be copied to destination PCE. Required")
	EdgeAdminCmd.MarkFlagRequired("edge-group")
	EdgeAdminCmd.Flags().StringVarP(&coreApp, "core-app", "a", "", "Set App Label to be added to group when imported into PCE.")
	EdgeAdminCmd.Flags().StringVarP(&coreEnv, "core-env", "e", "", "Set Env Label to be added to group when imported into PCE.")
	EdgeAdminCmd.Flags().StringVarP(&coreLoc, "core-loc", "l", "", "Set Loc Label to be added to group when imported into PCE.")
	EdgeAdminCmd.Flags().StringVarP(&refHeader, "ref-head", "r", "workloader-", "String used to match UMWL added by tool.  Default \"workloader-\".  Can be a string 20 charcters or less.")
	EdgeAdminCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	EdgeAdminCmd.Flags().BoolVarP(&keepTempFile, "keep-temp-file", "k", false, "Do not delete the temp CSV file created to update/create workloads on destination PCE.")
	EdgeAdminCmd.Flags().BoolVarP(&delStaleUMWL, "del-stale-umwl", "d", false, "Remove unmanaged workloads previously created by workloader on destination PCE that dont have a match on source PCE.  Default will be to not delete.")

}

// EdgeAdminCmd runs the upload command
var EdgeAdminCmd = &cobra.Command{
	Use:   "edge-admin ",
	Short: "Copy Edge group endpoint info including DN information into specified PCE as UMWL for certificate authenticated Admin Access to Core Workloads.",
	Long: `
Copy Edge group endpoint information including DN to Core PCE. Every endpoint must have a valid and PCE discovered ipsec certificate to be copied over.  
The Edge group once copied to PCE can be used in policy using MachineAuth option.  Using MachineAuth in policy limits connections to only those endpoints that have valid, known certificates.  
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

	//Used to make sure the column names are correctly set.
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

	//Make sure label is found...have user reenter if so
	slabel := sPce.Labels["role"+edgeGroup]
	if slabel.Value == "" {
		utils.LogError(fmt.Sprintf("error finding Edge group - %s.  Please reenter with exact Edge group name", edgeGroup))
	}

	//get all UMWL with the correct Admin Group label.
	queryP := map[string]string{"labels": fmt.Sprintf("[[\"%s\"]]", slabel.Href)}
	queryP["managed"] = "true"
	swklds, a, err := sPce.GetAllWorkloadsQP(queryP)
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	//Create a map of all UMWL on destination PCE so you can find stale UMWL
	toPCEonlywklds := make(map[string]string)

	//Check to see if you need to push UMWL or clean up on destination PCE
	if toPCE != "" {

		// Get the source pce
		input.PCE, err = utils.GetPCEbyName(toPCE, true)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting to pce - %s", err))
		}

		//create filter to get all workloads that are unmanaged and already labe the group and set labels,
		//queryP := map[string]string{"labels": "[[\"" + strings.Join(labelfilter, "\",\"") + "\"]]"}
		queryP := map[string]string{"managed": "false"}
		// Get all workloads from the destination PCE
		tmpdwklds, a, err := input.PCE.GetAllWorkloadsQP(queryP)
		utils.LogAPIResp("GetAllWorkloads", a)
		if err != nil {
			utils.LogError(err.Error())
		}

		for _, dw := range tmpdwklds {
			tmp := strings.Index(dw.ExternalDataReference, refHeader)
			if tmp == 0 {
				toPCEonlywklds[dw.ExternalDataReference] = dw.Href
			}
		}
		//Build a map so we can match find extra workloads left over.
	}

	csvOut := [][]string{{"hostname", "name", "role", "app", "env", "loc", "interfaces", "public_ip", "href", "description", "os_id", "os_detail", "datacenter", "external_data_set", "external_data_reference", "machine_authentication_id"}}

	for _, w := range swklds {

		// Output the CSV
		if len(swklds) > 0 {

			// Skip deleted workloads or endpoints without DNs
			if *w.Deleted {
				continue
			}
			if w.DistinguishedName == "" {
				continue
			}

			//label := w.GetRole(sPce.Labels).Value
			//remove workloads that are on both fromPCE and toPCE leaving only stale toPCE UMWL.
			tmpedgeGroup, tmpcoreApp, tmpcoreEnv, tmpcoreLoc := edgeGroup, coreApp, coreEnv, coreLoc
			if _, ok := toPCEonlywklds[refHeader+w.Hostname+w.Agent.Href]; ok {
				toPCEonlywklds[refHeader+w.Hostname+w.Agent.Href] = ""
				tmpedgeGroup, tmpcoreApp, tmpcoreEnv, tmpcoreLoc = "", "", "", ""
			}

			csvOut = append(csvOut, []string{w.Hostname, w.Name, tmpedgeGroup, tmpcoreApp, tmpcoreEnv, tmpcoreLoc, "", w.PublicIP, w.Href, w.Description, w.OsID, w.OsDetail, w.DataCenter, w.ExternalDataSet, refHeader + w.Hostname + w.Agent.Href, w.DistinguishedName})
		} else {
			utils.LogInfo("no Workloads created.", true)
		}
	}

	//Flatten and remove matched workloads on destination PCE to create delete list of WKLDS
	var listumwldel []string
	for _, href := range toPCEonlywklds {
		if href != "" {
			listumwldel = append(listumwldel, href)
		}
	}

	if outputFileName == "" {
		outputFileName = "workloader-edge-admin-output-" + time.Now().Format("20060102_150405") + ".csv"
	}
	input.ImportFile = outputFileName
	utils.WriteOutput(csvOut, csvOut, outputFileName)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !input.UpdatePCE {
		utils.LogInfo(fmt.Sprintln("\"--update-pce\" not specified.  See the output file for Workloads that would be created/updated. Run again using --update-pce flags to create the UMWLs."), true)
		utils.LogEndCommand("edge-admin")
		return
	}

	if toPCE == "" {
		utils.LogInfo(fmt.Sprintln("\"--to-pce\" not specified.  See the output file for Workloads that would be created/updated. Run again using --to-pce flags to create the IP lists."), true)
		utils.LogEndCommand("edge-admin")
		return
	}

	// If we get here, create the workloads on dest PCE using wkld-import
	utils.LogInfo(fmt.Sprintf("wkld-import called by edge-admin to import %s to %s", outputFileName, toPCE), true)
	wkldimport.ImportWkldsFromCSV(input)
	if !keepTempFile {
		if err := os.Remove(outputFileName); err != nil {
			utils.LogWarning(fmt.Sprintf("Could not delete %s", outputFileName), true)
		} else {
			utils.LogInfo(fmt.Sprintf("Deleted %s", outputFileName), false)
		}
	}

	//Check to see if you should remove old UMWL on dest PCE...end if not otherwise continue
	if len(listumwldel) > 0 {
		if !delStaleUMWL {
			utils.LogEndCommand("edge-admin")
			return
		}

		var delInput delete.Input
		delInput.PCE = input.PCE
		delInput.UpdatePCE = input.UpdatePCE
		delInput.NoPrompt = input.NoPrompt
		delInput.Hrefs = listumwldel

		utils.LogInfo(fmt.Sprintf("delete cmd called by edge-admin."), true)
		//Remove all HREFs that have not been matched on only if SYNC option is select
		delete.Delete(delInput)
	}
	utils.LogEndCommand("edge-admin")
}
