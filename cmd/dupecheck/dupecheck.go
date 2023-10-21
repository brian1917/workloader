package dupecheck

import (
	"fmt"
	"strings"
	"time"

	ia "github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var pce ia.PCE
var caseSensitive, oneInterfaceMatch bool
var outputFileName string
var err error

func init() {
	DupeCheckCmd.Flags().BoolVarP(&caseSensitive, "case-sensitive", "c", false, "Require hostname/name matches to be case-sensitve.")
	DupeCheckCmd.Flags().BoolVar(&oneInterfaceMatch, "one-interface-match", false, "consider a match if at least one interface matches. default requires all interfaces to match.")
	DupeCheckCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	DupeCheckCmd.Flags().SortFlags = false
}

// DupeCheckCmd summarizes flows
var DupeCheckCmd = &cobra.Command{
	Use:   "dupecheck",
	Short: "Identifies duplicate hostnames and IP addresses in the PCE.",
	Long: `
Identifies unmanaged workloads with hostnames, names, or IP addresses also assigned to managed workloads.

Interfaces with a default gateway are used on managed workloads.

The --update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		dupeCheck()
	},
}

func dupeCheck() {

	// Get all workloads
	apiResps, err := pce.Load(ia.LoadInput{Workloads: true, Labels: true, LabelDimensions: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	ldSlice := []string{}
	for _, ld := range pce.LabelDimensionsSlice {
		ldSlice = append(ldSlice, ld.Key)
	}

	headers := append([]string{"href", "hostname", "name", "interfaces"}, ldSlice...)
	// Get workload export data
	wkldExport := wkldexport.WkldExport{
		PCE:     &pce,
		Headers: headers,
	}
	csvDataMap := wkldExport.MapData()

	// Get all managed workloads
	managedHostNameMap := make(map[string]ia.Workload)
	managedIPAddressMap := make(map[string]ia.Workload)
	unmanagedWklds := []ia.Workload{}
	for _, w := range pce.WorkloadsSlice {
		if w.GetMode() == "unmanaged" {
			unmanagedWklds = append(unmanagedWklds, w)
		} else {
			if caseSensitive {
				managedHostNameMap[ia.PtrToVal(w.Hostname)] = w
			} else {
				managedHostNameMap[strings.ToLower(ia.PtrToVal(w.Hostname))] = w
			}
			for _, ip := range w.GetIsPWithDefaultGW() {
				managedIPAddressMap[ip] = w
			}
		}
	}

	// Start the header
	outputData := [][]string{append(headers, "reason")}

	// Iterate through unmanaged workloads
	for _, umwl := range unmanagedWklds {
		// Start our reason list. If this has a length 0 at the end, we don't have a dupe.
		reason := []string{}

		// Check managed hostnames
		hostname := strings.ToLower(ia.PtrToVal(umwl.Hostname))
		if caseSensitive {
			hostname = ia.PtrToVal(umwl.Hostname)
		}
		if val, ok := managedHostNameMap[hostname]; ok {
			reason = append(reason, fmt.Sprintf("unmanaged workload hostname matches with hostname of managed workload %s", val.Href))
		}

		// Check managed names
		name := strings.ToLower(ia.PtrToVal(umwl.Name))
		if caseSensitive {
			name = ia.PtrToVal(umwl.Name)
		}
		if val, ok := managedHostNameMap[name]; ok {
			reason = append(reason, fmt.Sprintf("unmanaged workload name matches with hostname of managed workload %s", val.Href))
		}

		// Check interfaces - all must exist in managed workloads to count.
		ifaceMatchCount := 0
		matches := []string{}
		for _, iface := range ia.PtrToVal(umwl.Interfaces) {
			if val, ok := managedIPAddressMap[iface.Address]; ok {
				ifaceMatchCount++
				matches = append(matches, val.Href)
			}
		}
		if ifaceMatchCount == len(ia.PtrToVal(umwl.Interfaces)) {
			reason = append(reason, fmt.Sprintf("unmanaged interfaces match to managed IP addresses of %s", strings.Join(matches, "; ")))
		} else if ifaceMatchCount == 1 && oneInterfaceMatch {
			reason = append(reason, fmt.Sprintf("one-interface-match set to true and unmanaged interfaces matches to one of a managed IP addresses of %s", strings.Join(matches, "; ")))
		}

		// If we have reasons, append
		if len(reason) > 0 {
			outputCsvRow := []string{}
			for _, header := range headers {
				outputCsvRow = append(outputCsvRow, csvDataMap[umwl.Href][header])
			}
			outputCsvRow = append(outputCsvRow, strings.Join(reason, ";"))
			outputData = append(outputData, outputCsvRow)
		}

	}
	// Write the output
	if len(outputData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-dupecheck-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(outputData, outputData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d unmanaged workloads found. The output file can be used as input to workloader delete command.", len(outputData)-1), true)
	} else {
		utils.LogInfo("No duplicates found", true)
	}

	// Log End
	utils.LogEndCommand("dupecheck")
}
