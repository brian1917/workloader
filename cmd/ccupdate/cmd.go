package ccupdate

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/cwpexport"
	"github.com/brian1917/workloader/cmd/cwpimport"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var targetMode, pairingProfileName, containerCluster string
var updatePCE, noPrompt, skipBackup bool

func init() {
	ContainerClusterUpdateCmd.Flags().StringVarP(&targetMode, "enforcement-state", "e", "", "values can be full, visibility_only, or unmanaged.")
	ContainerClusterUpdateCmd.Flags().StringVarP(&pairingProfileName, "pairing-profile", "p", "", "pairing profile to update with target enforcement state. blank value will use same string as target container cluster. use \"skip\" to skip pairing profile update. auto skips if enforcement-state is unmanaged.")
	ContainerClusterUpdateCmd.Flags().BoolVarP(&skipBackup, "skip-backup", "s", false, "skips running a cwp-export to capture state first.")

	ContainerClusterUpdateCmd.MarkFlagRequired("enforcement-state")
}

// WkldExportCmd runs the workload identifier
var ContainerClusterUpdateCmd = &cobra.Command{
	Use:   "container-cluster-update [name of container cluster]",
	Short: "Update the enforcement state for a container cluster.",
	Long: `
Update the enforcement state or management for a container cluster. 

When enforcement-state sent to "visibility_only" or "full":
   - Workloads (C-VENs) moved to enforcement-state
   - All container workload profiles moved to enforcement-state. This includes the default container workload profile for new workloads.
   - Pairing profile set to enforcement-state. This can be skipped (see pairing-profile flag).

When enforcement-state sent to "unmanaged":
   - Container workload profiles (including the default value for new container workload profiles) will be updated to unmanaged. This includes removing role, app, env, and location labels as it's necessary for moving to unmanaged. Custom label types not support in this command yet.
`,
	Run: func(cmd *cobra.Command, args []string) {

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		containerCluster = args[0]

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Check enforcement state
		if targetMode != "full" && targetMode != "visibility_only" && targetMode != "unmanaged" {
			utils.LogError("enforcement-state must be full, visibility_only, or unmanaged.")
		}

		ContainerClusterUpdate(pce, containerCluster, updatePCE, noPrompt)
	},
}

func ContainerClusterUpdate(originalPce illumioapi.PCE, containerClusterName string, updatePCE, noPrompt bool) {

	// Backup the CWP data
	if !skipBackup {
		// Run the cwp-export
		backupPce := copyPce(originalPce)
		utils.LogInfo("---------------- cwp export for backup ----------------", true)
		cwpexport.ExportContainerProfiles(backupPce)
	}

	// CWPs
	utils.LogInfo("---------------- container workload profiles ----------------", true)
	cwpPce := copyPce(originalPce)

	// Get the container cluster
	var containerCluster illumioapi.ContainerCluster
	api, err := cwpPce.GetContainerClusters(map[string]string{"name": containerClusterName})
	utils.LogAPIRespV2("GetContainerClusters", api)
	if err != nil {
		utils.LogErrorf("getting container clusters - %s", err)
	}
	for _, cc := range cwpPce.ContainerClustersSlice {
		if cc.Name == containerClusterName {
			containerCluster = cc
			break
		}
	}

	// Get the CWPs
	api, err = cwpPce.GetContainerWkldProfiles(nil, containerCluster.ID())
	utils.LogAPIRespV2("GetContainerWkldProfiles", api)
	if err != nil {
		utils.LogErrorf("getting container workload profiles - %s", err)
	}

	// Create CWP csv data
	visLevels := make(map[string]string)
	visLevels["flow_summary"] = "blocked_allowed"
	visLevels["flow_drops"] = "blocked"
	visLevels["flow_off"] = "off"
	visLevels["enhanced_data_collection"] = "enhanced_data_collection"

	cwpCsvData := [][]string{{"container_cluster", "name", "description", "namespace", "enforcement", "visibility", "managed", "href"}}
	labelKeys := []string{"role", "app", "env", "loc"}
	cwpCsvData[0] = append(cwpCsvData[0], labelKeys...)
	restoreCwpCsvData := [][]string{cwpCsvData[0]}
	if targetMode == "unmanaged" {
		for _, cwp := range cwpPce.ContainerWorkloadProfilesSlice {
			row := []string{containerClusterName, illumioapi.PtrToVal(cwp.Name), illumioapi.PtrToVal(cwp.Description), cwp.Namespace, "idle", visLevels[*cwp.VisibilityLevel], "false", cwp.Href}
			restoreRow := []string{containerClusterName, illumioapi.PtrToVal(cwp.Name), illumioapi.PtrToVal(cwp.Description), cwp.Namespace, illumioapi.PtrToVal(cwp.EnforcementMode), visLevels[*cwp.VisibilityLevel], strconv.FormatBool(*cwp.Managed), cwp.Href}
			for range labelKeys {
				row = append(row, "DELETE")
			}
			for _, key := range labelKeys {
				restoreRow = append(restoreRow, cwp.GetLabelByKey(key))
			}

			cwpCsvData = append(cwpCsvData, row)
			restoreCwpCsvData = append(restoreCwpCsvData, restoreRow)
		}
	} else {
		for _, cwp := range cwpPce.ContainerWorkloadProfilesSlice {
			row := []string{containerClusterName, illumioapi.PtrToVal(cwp.Name), illumioapi.PtrToVal(cwp.Description), cwp.Namespace, targetMode, visLevels[*cwp.VisibilityLevel], strconv.FormatBool(*cwp.Managed), cwp.Href}
			for range labelKeys {
				row = append(row, "")
			}
			cwpCsvData = append(cwpCsvData, row)
		}
	}
	cwpFileName := utils.FileName("cwps")

	// Process the CWP data
	utils.WriteOutput(cwpCsvData, nil, cwpFileName)
	if targetMode == "unmanaged" {
		utils.WriteOutput(restoreCwpCsvData, nil, utils.FileName("restore_cwp_to_managed"))
	}
	cwpUpdatePce := copyPce(originalPce)
	cwpimport.ImportContainerProfiles(cwpUpdatePce, cwpFileName, "DELETE", updatePCE, noPrompt)

	// Create the csv to update the node enforcement values
	if targetMode != "unmanaged" {
		utils.LogInfo("---------------- container workloads (c-vens) ----------------", true)
		wkldCsvData := [][]string{{"hostname", "enforcement"}}
		for _, node := range illumioapi.PtrToVal(containerCluster.Nodes) {
			wkldCsvData = append(wkldCsvData, []string{node.Name, targetMode})
		}
		wkldFileName := utils.FileName("wklds")
		utils.WriteOutput(wkldCsvData, nil, wkldFileName)
		wkldUpdatePce := copyPce(originalPce)
		wkldimport.ImportWkldsFromCSV(wkldimport.Input{
			PCE:                     wkldUpdatePce,
			ImportFile:              wkldFileName,
			UpdatePCE:               updatePCE,
			NoPrompt:                noPrompt,
			AllowEnforcementChanges: true,
			UpdateWorkloads:         true,
			MaxUpdate:               -1,
			IgnoreCase:              true,
		})
	}

	// Get the pairing profile
	if pairingProfileName != "skip" && targetMode != "unmanaged" {
		pairingProfilePce := copyPce(originalPce)
		utils.LogInfo("---------------- pairing profile ----------------", true)

		var pairingProfile illumioapi.PairingProfile
		if pairingProfileName == "" {
			pairingProfileName = containerClusterName
		}
		pairingProfiles, api, err := pairingProfilePce.GetPairingProfiles(map[string]string{"name": pairingProfileName})
		utils.LogAPIRespV2("GetPairingProfiles", api)
		if err != nil {
			utils.LogErrorf("getting pairing profiles - %s", err)
		}
		if len(pairingProfiles) == 0 {
			utils.LogErrorf("pairing profile %s not found", pairingProfileName)
		}
		for _, pp := range pairingProfiles {
			if pp.Name == pairingProfileName {
				pairingProfile = pp
				break
			}
		}

		// Update target mode
		if targetMode == "full" {
			targetMode = "enforced"
		}
		if targetMode == "visibility_only" {
			targetMode = "illuminated"
		}

		// Update the pairing profile
		if pairingProfile.Mode != targetMode {
			utils.LogInfof(true, "%s pairing profile to be changed from %s to %s", pairingProfileName, pairingProfile.Mode, targetMode)
			pairingProfile.Mode = targetMode

			// If updatePCE is set, but not noPrompt, we will prompt the user.
			if updatePCE && !noPrompt {
				var prompt string
				fmt.Printf("[PROMPT] - workloader will update the %s pairing profile in %s (%s). Do you want to run the import (yes/no)? ", pairingProfileName, pairingProfilePce.FriendlyName, viper.Get(pairingProfilePce.FriendlyName+".fqdn").(string))

				fmt.Scanln(&prompt)
				if strings.ToLower(prompt) != "yes" {
					utils.LogInfo("Prompt denied.", true)
					return
				}
			}

			if updatePCE {
				api, err = pairingProfilePce.UpdatePairingProfile(pairingProfile)
				utils.LogAPIRespV2("UpdatePairingProfile", api)
				if err != nil {
					utils.LogErrorf("updating pairing profile - %s", err)
				}
			}
		} else {
			utils.LogInfof(true, "%s pairing profile already set to %s", pairingProfileName, targetMode)
		}
	}

}

// copyPCE returns a PCE object
func copyPce(pce illumioapi.PCE) illumioapi.PCE {
	return illumioapi.PCE{
		FriendlyName:       pce.FriendlyName,
		FQDN:               pce.FQDN,
		Port:               pce.Port,
		Org:                pce.Org,
		User:               pce.User,
		Key:                pce.Key,
		Proxy:              pce.Proxy,
		DisableTLSChecking: pce.DisableTLSChecking,
		Version:            pce.Version,
	}
}
