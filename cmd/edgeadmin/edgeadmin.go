package edgeadmin

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/brian1917/workloader/cmd/deletehrefs"
	"github.com/brian1917/workloader/cmd/wkldimport"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var keepTempFile, delStaleUMWL bool
var fromPCE, toPCE, outputFileName, edgeGroup, coreRole, coreApp, coreEnv, coreLoc string
var input wkldimport.Input

func init() {
	EdgeAdminCmd.Flags().StringVarP(&fromPCE, "from-pce", "f", "", "Name of the PCE with the existing Edge Group used to copy to destination PCE. Required")
	EdgeAdminCmd.MarkFlagRequired("from-pce")
	EdgeAdminCmd.Flags().StringVarP(&toPCE, "to-pce", "t", "", "Name of the PCE to import into Edge Admin group endpoints as UMWL. Only required if using --update-pce flag")
	EdgeAdminCmd.Flags().StringVarP(&edgeGroup, "edge-group", "g", "", "Name of the Edge group to be copied to destination PCE. Required")
	EdgeAdminCmd.MarkFlagRequired("edge-group")
	EdgeAdminCmd.Flags().StringVarP(&coreRole, "core-role", "r", "", "Overide Edge Group Label with this Label when imported into PCE.")
	EdgeAdminCmd.Flags().StringVarP(&coreApp, "core-app", "a", "", "Set App Label to be added to UMWL when imported into PCE.")
	EdgeAdminCmd.Flags().StringVarP(&coreEnv, "core-env", "e", "", "Set Env Label to be added to UMWL when imported into PCE.")
	EdgeAdminCmd.Flags().StringVarP(&coreLoc, "core-loc", "l", "", "Set Loc Label to be added to UMWL when imported into PCE.")
	EdgeAdminCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	EdgeAdminCmd.Flags().BoolVarP(&keepTempFile, "keep-temp-file", "k", false, "Do not delete the temp CSV file created to update/create workloads on destination PCE.")
	EdgeAdminCmd.Flags().BoolVarP(&delStaleUMWL, "del-stale-umwl", "d", false, "Remove stale unmanaged endpoints from PCE that do not have a corresponding endpoint on Edge. Default - do not delete.")

}

// EdgeAdminCmd runs the upload command
var EdgeAdminCmd = &cobra.Command{
	Use:   "edge-admin ",
	Short: "Copy endpoints in an Edge Admin group to a Core PCE as unmanaged workloads with the Distinguished Name from the Edge PCE to use machine authentication policy between Edge PCE and Core PCE.",
	Long: `
Copy endpoints in an Edge Admin group to a Core PCE as unmanaged workloads with the Distinguished Name from the Edge PCE to use machine authentication policy between Edge PCE and Core PCE.
Every endpoint must have a valid and PCE discovered IPSec certificate to be copied.  
The Edge group once copied to the Core PCE can be used in policy using MachineAuth. Using MachineAuth in policy limits connections to only those endpoints that have valid, known certificates.  
No IP address information will be copied from Edge to Core so only MachineAuth rules will work.`,

	Run: func(cmd *cobra.Command, args []string) {

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

	// Log start of run
	utils.LogStartCommand("edge-admin")

	// Check if we have destination PCE if we need it
	if input.UpdatePCE && toPCE == "" {
		utils.LogError("need --to-pce (-t) flag set if using update-pce")
	}

	// Get the source pce
	srcPCE, err := utils.GetPCEbyName(fromPCE, true)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Make sure label is found...have user reenter if so
	edgeRoleLabel := srcPCE.Labels["role"+edgeGroup]
	if edgeRoleLabel.Value == "" {
		utils.LogError(fmt.Sprintf("edge group %s does not exist in %s", edgeGroup, fromPCE))
	}

	// If there is no core role assigned, used the Edge Group
	if coreRole == "" {
		coreRole = edgeGroup
	}

	// Get all unmanaged workloads with the correct Admin Group label and that are managed
	srcWklds, a, err := srcPCE.GetAllWorkloadsQP(map[string]string{"labels": fmt.Sprintf("[[\"%s\"]]", edgeRoleLabel.Href), "managed": "true"})
	utils.LogAPIResp("GetAllWorkloadsQP", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	//Create a map of all UMWL on destination PCE so you can find stale UMWL
	toPCEonlywklds := make(map[string]string)

	// Check to see if you need to push UMWL or clean up on destination PCE
	if toPCE != "" {

		// Set the destination PCE
		input.PCE, err = utils.GetPCEbyName(toPCE, true)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting to pce - %s", err))
		}

		// Get all UMWL workloads from the destination PCE
		destExistingWklds, a, err := input.PCE.GetAllWorkloadsQP(map[string]string{"managed": "false"})
		utils.LogAPIResp("GetAllWorkloadsQP", a)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Iterate through each existing destination workload
		for _, dw := range destExistingWklds {
			if strings.Index(dw.ExternalDataReference, edgeGroup) == 0 {
				toPCEonlywklds[dw.ExternalDataReference] = dw.Href
			}
		}
	}

	csvOut := [][]string{{"hostname", "name", "role", "app", "env", "loc", "interfaces", "public_ip", "href", "description", "os_id", "os_detail", "datacenter", "external_data_set", "external_data_reference", "machine_authentication_id"}}

	// If the source PCE has no workloads, log it.
	if len(srcWklds) == 0 {
		utils.LogInfo("no workloads in --from-pce match criteria", true)
	}

	for _, w := range srcWklds {

		// Skip deleted workloads or endpoints without DNs
		if *w.Deleted || w.DistinguishedName == "" {
			continue
		}

		// Set role, app, env, and location to assigned values
		role, app, env, loc := coreRole, coreApp, coreEnv, coreLoc

		// Check if the endpoint has been created already in the toPCE.
		if _, ok := toPCEonlywklds[edgeGroup+"%"+w.Hostname+w.Agent.Href]; ok {
			// Override the  labels with blank values so nothing changes
			role, app, env, loc = "", "", "", ""
			// Remove value from the map so it's not deleted later.
			delete(toPCEonlywklds, edgeGroup+"%"+w.Hostname+w.Agent.Href)
		}

		// Append to the CSV Data
		csvOut = append(csvOut, []string{w.Hostname, w.Name, role, app, env, loc, "", w.PublicIP, w.Href, w.Description, w.OsID, w.OsDetail, w.DataCenter, w.ExternalDataSet, edgeGroup + "%" + w.Hostname + w.Agent.Href, w.DistinguishedName})
	}

	//Flatten and remove matched workloads on destination PCE to create delete list of WKLDS
	var umwlsToDelete []string
	for _, href := range toPCEonlywklds {
		umwlsToDelete = append(umwlsToDelete, href)
	}

	// Write the update workloads output
	if outputFileName == "" {
		outputFileName = "workloader-edge-admin" + time.Now().Format("20060102_150405") + ".csv"
	}
	input.ImportFile = outputFileName
	utils.WriteOutput(csvOut, csvOut, outputFileName)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !input.UpdatePCE {
		utils.LogInfo(fmt.Sprintln("\"--update-pce\" not specified.  See the output file for workloads that would be created/updated. Run again using --update-pce flags to create the unmanaged workloads."), true)
		utils.LogEndCommand("edge-admin")
		return
	}

	if toPCE == "" {
		utils.LogInfo(fmt.Sprintln("\"--to-pce\" not specified.  See the output file for workloads that would be created/updated. Run again using --to-pce flags to create the unmanaged workloads."), true)
		utils.LogEndCommand("edge-admin")
		return
	}

	// If we get here, create the workloads on dest PCE using wkld-import
	utils.LogInfo(fmt.Sprintf("wkld-import called by edge-admin to import %s to %s", outputFileName, toPCE), true)
	wkldimport.ImportWkldsFromCSV(input)
	if !keepTempFile {
		if err := os.Remove(outputFileName); err != nil {
			utils.LogWarning(fmt.Sprintf("could not delete %s", outputFileName), true)
		} else {
			utils.LogInfo(fmt.Sprintf("deleted %s", outputFileName), false)
		}
	}

	//Check to see if you should remove old UMWL on dest PCE...end if not otherwise continue
	if len(umwlsToDelete) > 0 && delStaleUMWL {

		utils.LogInfo("delete cmd called by edge-admin.", true)

		deletehrefs.DeleteHrefs(deletehrefs.Input{
			PCE:       input.PCE,
			UpdatePCE: input.UpdatePCE,
			NoPrompt:  input.NoPrompt,
			Hrefs:     umwlsToDelete})
	}

	utils.LogEndCommand("edge-admin")
}
