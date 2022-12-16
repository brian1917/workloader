package wkldreplicate

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pceList, skipSources, outputFileName string
var updatePCE, noPrompt bool

func init() {
	WkldReplicate.Flags().StringVarP(&pceList, "pce-list", "p", "", "comma-separated list of pce names (not fqdns). see workloader pce-list for options.")
	WkldReplicate.Flags().StringVarP(&skipSources, "skip-source", "s", "", "comma-separated list of pce names (not fqdns) to skip as a source. the pces still received workloads from other pces.")
	WkldReplicate.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename. there will be a prefix added to each provided filename.")
}

// WkldReplicate runs the wkld-replicate command
var WkldReplicate = &cobra.Command{
	Use:   "wkld-replicate",
	Short: "Replicate workloads between multiple PCEs.",
	Long: `
Replicate workloads between multiple PCEs.

Managed and unmanaged workloads are replicated across all PCEs. The command creates and deletes unmanaged workloads. Unmanaged workloads are deleted in the following scenarios:
1. The managed workload it was replicated from is unpaired.
2. The original unmanaged workload it was replicated from is deleted.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)
		wkldReplicate()
	},
}

type replicateWkld struct {
	pce      illumioapi.PCE
	workload illumioapi.Workload
}

func processLabel(l illumioapi.Label) string {
	if l.Key == "" {
		return "wkld-replicate-remove"
	} else {
		return l.Value
	}
}

func wkldReplicate() {

	// Process the pce list
	var pces []illumioapi.PCE
	for _, pce := range strings.Split(strings.Replace(pceList, " ", "", -1), ",") {
		p, err := utils.GetPCEbyName(pce, true)
		if err != nil {
			utils.LogError(err.Error())
		}
		pces = append(pces, p)
	}

	skipPCENameMap := make(map[string]bool)
	if skipSources != "" {
		for _, pce := range strings.Split(strings.Replace(skipSources, " ", "", -1), ",") {
			p, err := utils.GetPCEbyName(pce, true)
			if err != nil {
				utils.LogError(err.Error())
			}
			skipPCENameMap[p.FriendlyName] = true
		}
	}
	// Create maps for workloads
	managedWkldMap := make(map[string]replicateWkld)
	unmanagedWkldMap := make(map[string]replicateWkld)

	// Start the csv data
	wkldImportCsvData := [][]string{{wkldexport.HeaderHostname, wkldexport.HeaderDescription, wkldexport.HeaderRole, wkldexport.HeaderApp, wkldexport.HeaderEnv, wkldexport.HeaderLoc, wkldexport.HeaderInterfaces, wkldexport.HeaderExternalDataSet, wkldexport.HeaderExternalDataReference}}
	wkldDeleteCsvdata := [][]string{{"href", "pce_fqdn", "pce_name"}}
	deleteHrefMap := make(map[string][]string)

	// Iterate through the PCEs and do initial processing of workloads
	for _, p := range pces {
		// If it's  a skip source, skip it
		if skipPCENameMap[p.FriendlyName] {
			continue
		}

		// Start the delete slice
		deleteHrefMap[p.FQDN] = []string{}

		// Get the workloads
		utils.LogInfo(fmt.Sprintf("getting workloads for %s (%s)", p.FriendlyName, p.FQDN), true)
		_, a, err := p.GetWklds(nil)
		utils.LogAPIResp("GetWkld", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("%d workloads in %s (%s)", len(p.WorkloadsSlice), p.FriendlyName, p.FQDN), true)

		utils.LogInfo("------------------------------", true)

		// Iterate over all managed and unmanaged workloads separately
		for _, w := range p.WorkloadsSlice {
			// Managed workloads
			if w.GetMode() != "unmanaged" {
				// Put it in the map
				managedWkldMap[p.FQDN+w.Hostname] = replicateWkld{pce: p, workload: w}
				// Edit the external data reference section
				w.ExternalDataSet = utils.StrToPtr("wkld-replicate")
				w.ExternalDataReference = utils.StrToPtr(p.FQDN + "-managed-wkld-" + w.Href)
				// Add to the CSV output
				wkldImportCsvData = append(wkldImportCsvData, []string{w.Hostname, fmt.Sprintf("managed ven on %s", p.FQDN), processLabel(w.GetRole(p.Labels)), processLabel(w.GetApp(p.Labels)), processLabel(w.GetEnv(p.Labels)), processLabel(w.GetLoc(p.Labels)), strings.Join(wkldexport.InterfaceToString(w, true), ";"), utils.PtrToStr(w.ExternalDataSet), utils.PtrToStr(w.ExternalDataReference)})
			}

			// Unmanaged - just put in the map. Needs to be processed after maps are complete.
			if w.GetMode() == "unmanaged" {
				unmanagedWkldMap[p.FQDN+w.Hostname] = replicateWkld{pce: p, workload: w}
			}
		}
	}

	// Iterate through all the unmanaged workloads
	for _, wkld := range unmanagedWkldMap {
		// If it's not in the dataset yet, update the external data reference and add it to the csv
		if utils.PtrToStr(wkld.workload.ExternalDataSet) != "wkld-replicate" {
			wkld.workload.ExternalDataSet = utils.StrToPtr("wkld-replicate")
			wkld.workload.ExternalDataReference = utils.StrToPtr(wkld.pce.FQDN + "-unmanaged-wkld-" + wkld.workload.Href)
			wkldImportCsvData = append(wkldImportCsvData, []string{wkld.workload.Hostname, fmt.Sprintf("unmanaged workload on %s", wkld.pce.FQDN), processLabel(wkld.workload.GetRole(wkld.pce.Labels)), processLabel(wkld.workload.GetApp(wkld.pce.Labels)), processLabel(wkld.workload.GetEnv(wkld.pce.Labels)), processLabel(wkld.workload.GetLoc(wkld.pce.Labels)), strings.Join(wkldexport.InterfaceToString(wkld.workload, true), ";"), utils.PtrToStr(wkld.workload.ExternalDataSet), utils.PtrToStr(wkld.workload.ExternalDataReference)})
			continue
		}

		// If it's from the origin PCE, add it to the CSV
		if wkld.pce.FQDN == strings.Split(utils.PtrToStr(wkld.workload.ExternalDataReference), "-unmanaged-wkld-")[0] {
			wkldImportCsvData = append(wkldImportCsvData, []string{wkld.workload.Hostname, fmt.Sprintf("unmanaged workload on %s", wkld.pce.FQDN), processLabel(wkld.workload.GetRole(wkld.pce.Labels)), processLabel(wkld.workload.GetApp(wkld.pce.Labels)), processLabel(wkld.workload.GetEnv(wkld.pce.Labels)), processLabel(wkld.workload.GetLoc(wkld.pce.Labels)), strings.Join(wkldexport.InterfaceToString(wkld.workload, true), ";"), utils.PtrToStr(wkld.workload.ExternalDataSet), utils.PtrToStr(wkld.workload.ExternalDataReference)})

		}

		// If in the dataset and from a managed workload and it doesn't exist in the managed workload map, set it to be deleted
		if utils.PtrToStr(wkld.workload.ExternalDataSet) == "wkld-replicate" && strings.Contains(utils.PtrToStr(wkld.workload.ExternalDataReference), "-managed-wkld-") {
			if _, exists := managedWkldMap[strings.Split(utils.PtrToStr(wkld.workload.ExternalDataReference), "-managed-wkld-")[0]+wkld.workload.Hostname]; !exists {
				wkldDeleteCsvdata = append(wkldDeleteCsvdata, []string{wkld.workload.Href, wkld.pce.FQDN, wkld.pce.FriendlyName})
				deleteHrefMap[wkld.pce.FQDN] = append(deleteHrefMap[wkld.pce.FQDN], wkld.workload.Href)
			}
			continue
		}

		// Clean up UMWLs
		if utils.PtrToStr(wkld.workload.ExternalDataSet) == "wkld-replicate" && strings.Contains(utils.PtrToStr(wkld.workload.ExternalDataReference), "-unmanaged-wkld-") {
			if _, exists := unmanagedWkldMap[strings.Split(utils.PtrToStr(wkld.workload.ExternalDataReference), "-unmanaged-wkld-")[0]+wkld.workload.Hostname]; !exists {
				wkldDeleteCsvdata = append(wkldDeleteCsvdata, []string{wkld.workload.Href, wkld.pce.FQDN, wkld.pce.FriendlyName})
				deleteHrefMap[wkld.pce.FQDN] = append(deleteHrefMap[wkld.pce.FQDN], wkld.workload.Href)
			}
		}
	}

	// Export the wkld-import CSV
	var wkldCsvFileName string
	if len(wkldImportCsvData) > 1 {
		if outputFileName == "" {
			wkldCsvFileName = fmt.Sprintf("workloader-wkld-replicate-wkld-import-%s.csv", time.Now().Format("20060102_150405"))
		} else {
			wkldCsvFileName = "wkld-import-" + outputFileName
		}
		utils.WriteOutput(wkldImportCsvData, wkldImportCsvData, wkldCsvFileName)
		utils.LogInfo(fmt.Sprintf("%d workloads to be imported", len(wkldImportCsvData)-1), true)
	}

	// Export the wklds to delete
	var deleteCsvFileName string
	if len(wkldDeleteCsvdata) > 1 {
		if outputFileName == "" {
			deleteCsvFileName = fmt.Sprintf("workloader-wkld-replicate-wkld-delete-%s.csv", time.Now().Format("20060102_150405"))
		} else {
			deleteCsvFileName = "wkld-delete-" + outputFileName
		}
		utils.WriteOutput(wkldDeleteCsvdata, wkldDeleteCsvdata, deleteCsvFileName)
		utils.LogInfo(fmt.Sprintf("%d workloads to be deleted", len(wkldDeleteCsvdata)-1), true)
	}

	utils.LogInfo("------------------------------", true)

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo("see workloader.log for more details. to do the import, run again using --update-pce flag.", true)
		utils.LogEndCommand("wkld-replicate")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("\r\n%s [PROMPT] - do you want to run the replicate (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied", true)
			utils.LogEndCommand("wkld-replicate")
			return
		}
	}

	// Run the actions against PCEs
	for _, p := range pces {
		if len(wkldImportCsvData) > 1 {
			utils.LogInfo(fmt.Sprintf("running wkld-import for %s (%s) with %s", p.FriendlyName, p.FQDN, wkldCsvFileName), true)
			wkldimport.ImportWkldsFromCSV(wkldimport.Input{
				PCE:             p,
				ImportFile:      wkldCsvFileName,
				RemoveValue:     "wkld-replicate-remove",
				Umwl:            true,
				UpdatePCE:       true,
				NoPrompt:        true,
				UpdateWorkloads: true,
			})
		}

		// Delete the hrefs
		if len(wkldDeleteCsvdata) > 1 {
			utils.LogInfo(fmt.Sprintf("running delete api for %s (%s)", p.FriendlyName, p.FQDN), true)
			for _, deleteHref := range deleteHrefMap[p.FQDN] {
				a, err := p.DeleteHref(deleteHref)
				utils.LogAPIResp("DeleteHref", a)
				if err != nil {
					utils.LogError(err.Error())
				}
				utils.LogInfo(fmt.Sprintf("%s is in %s delete - %d", deleteHref, p.FQDN, a.StatusCode), true)
			}
		}

		utils.LogInfo("------------------------------", true)
	}

	utils.LogEndCommand("wkld-replicate")

}
