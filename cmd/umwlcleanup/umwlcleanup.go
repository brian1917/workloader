package umwlcleanup

import (
	"fmt"
	"strings"
	"time"

	ia "github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce ia.PCE
var err error
var oneInterfaceMatch bool
var outputFileName string

func init() {
	UMWLCleanUpCmd.Flags().BoolVar(&oneInterfaceMatch, "one-interface-match", false, "consider a match if at least one interface matches. default requires all interfaces to match.")
	UMWLCleanUpCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

}

// UMWLCleanUpCmd runs the workload identifier
var UMWLCleanUpCmd = &cobra.Command{
	Use:   "umwl-cleanup",
	Short: "Create a CSV that identifies unmanaged workloads and managed workloads that have the same IP address",
	Long: `
Create a CSV that identifies unmanaged workloads and managed workloads that have the same IP address.

This command will help in the situation where you have created and labeled unmanaged workloads and later installed VENs on those workloads.

The unmanaged workload IP address is compared to managed workload's NIC with the default gateway. If an unmanaged workload has multiple IP addresses, the managed workload must contain all of them.

To label the managed workloads with the same labels on the matched unmanaged workload, the output file can be directly passed into the wkld-import command.

Additionally, the output can be passed into the delete command with the --header flag set to umwl_href to delete the no longer needed unmanaged workloads.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		umwlCleanUp()
	},
}

func umwlCleanUp() {

	// Log start of command
	utils.LogStartCommand("umwl-cleanup")

	// Get all workloads, labels and label dimensions
	apiResps, err := pce.Load(ia.LoadInput{Workloads: true, LabelDimensions: true, Labels: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	ldSlice := []string{}
	for _, ld := range pce.LabelDimensionsSlice {
		ldSlice = append(ldSlice, ld.Key)
	}

	// Create the maps
	umwlDefaultIPMap := make(map[string]ia.Workload)
	managedDefaultIPMap := make(map[string]ia.Workload)
	allManagedIPMap := make(map[string]ia.Workload)

	// Populate the maps
	for _, w := range pce.WorkloadsSlice {
		if w.GetMode() == "unmanaged" {
			for _, i := range ia.PtrToVal(w.Interfaces) {
				umwlDefaultIPMap[i.Address] = w
			}
		} else {
			for _, i := range ia.PtrToVal(w.Interfaces) {
				allManagedIPMap[i.Address] = w
			}
			if w.GetIPWithDefaultGW() != "NA" {
				defaultGWIPs := w.GetIsPWithDefaultGW()
				for _, ip := range defaultGWIPs {
					managedDefaultIPMap[ip] = w
				}

			}
		}
	}

	// Start our data slice
	var managedLabels, unmanagedLabels []string
	for _, ld := range ldSlice {
		managedLabels = append(managedLabels, "managed_"+ld)
		unmanagedLabels = append(unmanagedLabels, "umwl_"+ld)
	}
	data := [][]string{{"managed_hostname", "umwl_hostname", "umwl_name", "managed_interfaces", "umwl_interfaces", "managed_href", "unmanaged_href", "managed_external_data_set", "managed_external_data_ref", "umwl_external_data_set", "umwl_external_data_ref"}}
	data[0] = append(append(data[0], managedLabels...), unmanagedLabels...)
	data[0] = append(append(data[0], "href"), ldSlice...)

	// Find managed workloads that have the same IP address of an unmanaged workload
workloads:
	for ipAddress, managedWkld := range managedDefaultIPMap {
		if umwl, check := umwlDefaultIPMap[ipAddress]; check {

			// Hit here if there's a match. First, check if all IPs match
			// Get IP strings
			umwlIPs, managedIPs := []string{}, []string{}
			for _, i := range ia.PtrToVal(umwl.Interfaces) {
				if allManagedIPMap[i.Address].Href != managedWkld.Href && !oneInterfaceMatch {
					umwlIdentifier := []string{}
					if ia.PtrToVal(umwl.Hostname) != "" {
						umwlIdentifier = append(umwlIdentifier, fmt.Sprintf("hostname: %s", ia.PtrToVal(umwl.Hostname)))
					}
					if ia.PtrToVal(umwl.Name) != "" {
						umwlIdentifier = append(umwlIdentifier, fmt.Sprintf("name: %s", ia.PtrToVal(umwl.Name)))
					}
					utils.LogWarning(fmt.Sprintf("Unmanaged workload - %s - has multiple IP addresses. At least one matches managed workload %s, but others do not. Skipping.", strings.Join(umwlIdentifier, ";"), ia.PtrToVal(managedWkld.Hostname)), true)
					continue workloads
				}
				umwlIPs = append(umwlIPs, fmt.Sprintf("%s:%s", i.Name, i.Address))
			}
			for _, i := range ia.PtrToVal(managedWkld.Interfaces) {
				managedIPs = append(managedIPs, fmt.Sprintf("%s:%s", i.Name, i.Address))
			}
			//
			dataRow := []string{ia.PtrToVal(managedWkld.Hostname), ia.PtrToVal(umwl.Hostname), ia.PtrToVal(umwl.Name), strings.Join(managedIPs, ";"), strings.Join(umwlIPs, ";"), managedWkld.Href, umwl.Href, ia.PtrToVal(managedWkld.ExternalDataSet), ia.PtrToVal(managedWkld.ExternalDataReference), ia.PtrToVal(umwl.ExternalDataSet), ia.PtrToVal(umwl.ExternalDataReference)}

			// append the managed labels
			for _, ld := range ldSlice {
				dataRow = append(dataRow, managedWkld.GetLabelByKey(ld, pce.Labels).Value)
			}
			// append the unmanaged labels
			for _, ld := range ldSlice {
				dataRow = append(dataRow, umwl.GetLabelByKey(ld, pce.Labels).Value)
			}
			// append the managed href and the unmanaged labels so they can be passed into wkld-import to be applied
			dataRow = append(dataRow, managedWkld.Href)
			for _, ld := range ldSlice {
				dataRow = append(dataRow, umwl.GetLabelByKey(ld, pce.Labels).Value)
			}
			data = append(data, dataRow)

		}
	}

	// Write the output
	if len(data) > 0 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-umwl-cleanup-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.LogInfo(fmt.Sprintf("%d matches found", len(data)-1), true)
		utils.WriteOutput(data, data, outputFileName)
	}

	// Log end of command
	utils.LogEndCommand("umwl-cleanup")
}
