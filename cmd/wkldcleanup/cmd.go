package wkldcleanup

import (
	"fmt"
	"time"

	ia "github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var outputFileName string

func init() {
	WkldCleanUpCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
}

var WkldCleanUpCmd = &cobra.Command{
	Use:   "wkld-cleanup",
	Short: "Create a csv export of VENs that should be removed.",
	Long: `
Create a csv export of VENs that should be removed.

The criteria for identifying for removal are below:
1) Multiple VENs with the same hostname
2) Other VENs with the same hostname have a more recent heartbeat

The output of this command can be fed into the wkld-unpair command.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogErrorf("getting target pce - %s", err)
		}

		// Create the WkldCleanUp object
		w := WkldCleanUp{PCE: pce}
		w.Execute()
	},
}

// wkldCleanUp struct represents the object to do a workload cleanup
type WkldCleanUp struct {
	PCE ia.PCE
}

func (w *WkldCleanUp) Execute() {

	// Load input from the pce
	apiResps, err := w.PCE.Load(ia.LoadInput{Workloads: true, VENs: true, LabelDimensions: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogErrorf("error loading the pce - %s", err)
	}

	// Create a map with the hostname as the key and the value as a slice of workloads
	venHostnameMap := make(map[string][]ia.VEN)
	for _, ven := range w.PCE.VENsSlice {
		venHostnameMap[ia.PtrToVal(ven.Hostname)] = append(venHostnameMap[ia.PtrToVal(ven.Hostname)], ven)
	}

	// Create the results output
	type result struct {
		mostRecentVENHref   string
		mostRecentHeartbeat time.Time
		ven                 ia.VEN
		wkld                ia.Workload
	}

	removeResults := []result{}

	// Iterate through the map to find the one with the most recent heartbeat
	for _, venSlice := range venHostnameMap {
		// Do not need to process if there is only one VEN
		if len(venSlice) == 1 {
			continue
		}

		// Iterate one time to get the most recent VEN
		mostRecentTimeStamp := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		mostRecentVENHref := ""
		for _, ven := range venSlice {
			lastHbTime, err := time.Parse("2006-01-02T15:04:05.000Z", ven.LastHeartBeatAt)
			if err != nil {
				utils.LogErrorf("parsing last heartbeat time - %s", err)
			}
			if lastHbTime.After(mostRecentTimeStamp) {
				mostRecentTimeStamp, err = time.Parse("2006-01-02T15:04:05.000Z", ven.LastHeartBeatAt)
				if err != nil {
					utils.LogErrorf("parsing new most recent heartbeat time - %s", err)
				}
				mostRecentVENHref = ven.Href
			}
		}

		// Iterate a second time to produce the output
		for _, ven := range venSlice {
			// Skip the most recent VEN
			if ven.Href == mostRecentVENHref {
				continue
			}
			// Get the workload
			if len(ia.PtrToVal(ven.Workloads)) != 1 {
				utils.LogErrorf("ven %s does not have 1 workload attached", ven.Href)
			}
			if val, ok := w.PCE.Workloads[ia.PtrToVal(ven.Workloads)[0].Href]; !ok {
				utils.LogErrorf("cannot find %s attached to %s", ia.PtrToVal(ven.Workloads)[0].Href, ven.Href)
			} else {
				removeResults = append(removeResults, result{mostRecentVENHref: mostRecentVENHref, mostRecentHeartbeat: mostRecentTimeStamp, ven: ven, wkld: val})
			}
		}
	}

	// Create the label dimensions slice
	labelDimensions := []string{}
	for _, ld := range w.PCE.LabelDimensionsSlice {
		labelDimensions = append(labelDimensions, ld.Key)
	}

	// Process the CSV output
	outputData := [][]string{{"href", "hostname", "ven_type"}}
	outputData[0] = append(outputData[0], labelDimensions...)
	outputData[0] = append(outputData[0], "removal_reason")
	for _, result := range removeResults {
		row := []string{result.ven.Href, ia.PtrToVal(result.ven.Hostname), result.ven.VenType}
		for _, ld := range labelDimensions {
			row = append(row, result.wkld.GetLabelByKey(ld, w.PCE.Labels).Value)
		}
		row = append(row, fmt.Sprintf("most recent heartbeat at %s. %s has a more recent heartbeat at %s", result.ven.LastHeartBeatAt, result.mostRecentVENHref, result.mostRecentHeartbeat.String()))
		outputData = append(outputData, row)
	}

	// Write the CSV
	if len(outputData) == 1 {
		utils.LogInfo("no workloads to clean up", true)
		return
	}

	// Write the output
	if outputFileName == "" {
		outputFileName = fmt.Sprintf("workloader-label-export-%s.csv", time.Now().Format("20060102_150405"))
	}

	utils.WriteOutput(outputData, nil, outputFileName)
}
