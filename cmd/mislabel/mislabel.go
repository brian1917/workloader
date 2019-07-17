package mislabel

import (
	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var envFlag, appFlag, locFlag string
var pce illumioapi.PCE
var err error

func init() {
	MisLabelCmd.Flags().StringVarP(&appFlag, "app", "a", "", "App label.")
	MisLabelCmd.Flags().StringVarP(&envFlag, "env", "r", "", "Role label.")
	MisLabelCmd.Flags().StringVarP(&locFlag, "loc", "l", "", "Location label.")
	MisLabelCmd.Flags().SortFlags = false
}

// MisLabelCmd Finds workloads that have no communications within an App-Group....
var MisLabelCmd = &cobra.Command{
	Use:   "mislabel",
	Short: "Display all workloads that have no intra App-Group communications.",
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetPCE("pce.json")
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for mislabel command - %s", err)
		}

		misLabel()
	},
}

//misLabel - Figure out if worklaods in an app-group only communicate outside the app-group.
func misLabel() {

	var wklds []map[string]illumioapi.Workload
	var tmpw map[string]illumioapi.Workload
	workloads, _, _ := illumioapi.GetAllWorkloads(pce)
	for _, w := range workloads {
		if w.Agent.Href != "" {
			tmpw[w.Href] = w
			wklds = append(wklds, tmpw)
		}
	}

}
