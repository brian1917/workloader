package mislabel

import (
	"fmt"
	"time"

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

	debug := true
	ignoreloc := true

	ignorewkld := make(map[string]bool)
	labelmap, err := illumioapi.GetLabelMapH(pce)
	//fmt.Println(labelsAPI, apiResp, err)
	if err != nil {
		utils.Logger.Fatal(err)
	}
	if debug == true {
		utils.Log(2, fmt.Sprintf("Get All Labels in a map using HREF as key.\r\n"))
	}

	hrefwkld, _ := illumioapi.GetWkldHrefMap(pce)
	for _, w := range hrefwkld {
		if !ignorewkld[w.Href] {
			tq := illumioapi.TrafficQuery{
				StartTime:      time.Date(2013, 1, 1, 0, 0, 0, 0, time.UTC),
				EndTime:        time.Date(2020, 12, 30, 0, 0, 0, 0, time.UTC),
				PolicyStatuses: []string{"allowed", "potentially_blocked", "blocked"},
				SourcesInclude: []string{w.Href},
				MaxFLows:       100000}

			// If an app is provided, run with that app as the consumer.
			// tq.SourcesInclude = []string{w}

			traffic, apiResp, err := illumioapi.GetTrafficAnalysis(pce, tq)
			if err != nil {
				utils.Logger.Fatal(err)
			}
			if debug == true {
				utils.Log(2, fmt.Sprintf("Get All Labels API HTTP Request: %s %v \r\n", apiResp.Request.Method, apiResp.Request.URL))
				utils.Log(2, fmt.Sprintf("Get All Labels API HTTP Reqest Header: %v \r\n", apiResp.Request.Header))
				utils.Log(2, fmt.Sprintf("Get All Labels API Response Status Code: %d \r\n", apiResp.StatusCode))
				utils.Log(0, fmt.Sprintf("Get All Labels API Response Body: \r\n %s \r\n", apiResp.RespBody))
			}
			dstwkld := make(map[string]string)
			for _, ta := range traffic {
				fmt.Println(w.Hostname, ta.Dst.Workload.Hostname)
				if ta.Dst.Workload != nil {
					if w.GetApp(labelmap).Value != ta.Dst.Workload.GetApp(labelmap).Value && w.GetEnv(labelmap).Value != ta.Dst.Workload.GetEnv(labelmap).Value {
						if ignoreloc {
							if w.GetLoc(labelmap).Value != ta.Dst.Workload.GetLoc(labelmap).Value {
								fmt.Println("match")
								dstwkld[ta.Dst.Workload.Hostname] = w.Hostname
							}
						} else {
							ignorewkld[ta.Dst.Workload.Href] = true
							//continue
						}
					}
				}
			}
		}
	}
}
