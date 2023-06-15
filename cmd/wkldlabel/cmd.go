package wkldlabel

import (
	"fmt"
	"strings"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var labels, hostname string

func init() {
	WkldLabelCmd.Flags().StringVarP(&hostname, "hostname", "n", "", "hostname of the workload to label")
	WkldLabelCmd.Flags().StringVarP(&labels, "labels", "l", "", "comma-separated list of labels to apply to the workload. labels should be in the format of key:value (e.g., loc:bos,env:prod)")

	WkldLabelCmd.MarkFlagRequired("hostname")
	WkldLabelCmd.MarkFlagRequired("labels")

	WkldLabelCmd.Flags().SortFlags = false
}

// WkldLabelCmd runs wkld-label
var WkldLabelCmd = &cobra.Command{
	Use:   "wkld-label",
	Short: "Label a specific workload.",
	Long: `
Label a specific workload.

The command leverages the wkld-import command. The  workloader.log file will log as if it is a single entry in a csv.
`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting pce - %s", err.Error()))
		}

		updatePCE := viper.Get("update_pce").(bool)
		noPrompt := viper.Get("no_prompt").(bool)

		LabelWkld(&pce, hostname, labels, updatePCE, noPrompt)
	},
}

func LabelWkld(pce *illumioapi.PCE, hostname, labels string, updatePCE, noPrompt bool) {

	// Log Start
	utils.LogStartCommand("wkld-label")

	// Get the hostname
	wkld, api, err := pce.GetWkldByHostname(hostname)
	utils.LogAPIRespV2("GetWkldByHostname", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	if illumioapi.PtrToVal(wkld.Hostname) == "" {
		utils.LogError(fmt.Sprintf("no workload with hostname %s found", hostname))
	}

	// Load the PCEs labels
	api, err = pce.GetLabels(nil)
	utils.LogAPIRespV2("GetLabels", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create wkld-import data
	wkldImportData := [][]string{{"hostname"}, {illumioapi.PtrToVal(wkld.Hostname)}}

	// Parse the labels
	labels = strings.Replace(labels, ", ", ",", -1)
	for _, keyValue := range strings.Split(labels, ",") {
		wkldImportData[0] = append(wkldImportData[0], strings.Split(keyValue, ":")[0])
		wkldImportData[1] = append(wkldImportData[1], strings.Split(keyValue, ":")[1])
	}

	// Call wkld-import
	wkldimport.ImportWkldsFromCSV(wkldimport.Input{
		PCE:             *pce,
		ImportData:      wkldImportData,
		RemoveValue:     "nil",
		Umwl:            false,
		UpdateWorkloads: true,
		UpdatePCE:       updatePCE,
		NoPrompt:        noPrompt,
		MaxUpdate:       -1,
		MaxCreate:       -1,
	})

	// Log End
	utils.LogEndCommand("wkld-label")

}
