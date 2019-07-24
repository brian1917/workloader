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

		pce, err = utils.GetPCE()
		if err != nil {
			utils.Log(1, fmt.Sprintf("error getting pce - %s", err))
		}

		misLabel()
	},
}

//misLabel determines if workloads in an app-group only communicate outside the app-group.
func misLabel() {

	debug := true
	ignoreloc := false

	// Get the labelMap
	labelmap, err := illumioapi.GetLabelMapH(pce)
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting labelmap - %s", err))
	}

	if debug == true {
		utils.Log(2, fmt.Sprintf("got href label map with %d entries", len(labelmap)))
	}

	// Build the traffic query struct
	tq := illumioapi.TrafficQuery{
		StartTime:      time.Date(2013, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:        time.Now(),
		PolicyStatuses: []string{"allowed", "potentially_blocked", "blocked"},
		MaxFLows:       100000}

	// Get traffic from explorer API
	traffic, apiResp, err := illumioapi.GetTrafficAnalysis(pce, tq)
	if err != nil {
		utils.Log(1, fmt.Sprintf("error making traffic api call - %s", err))
	}

	// Eventually move this into an api debug function
	if debug {
		utils.Log(2, fmt.Sprintf("Get All Labels API HTTP Request: %s %v \r\n", apiResp.Request.Method, apiResp.Request.URL))
		utils.Log(2, fmt.Sprintf("Get All Labels API HTTP Reqest Header: %v \r\n", apiResp.Request.Header))
		utils.Log(2, fmt.Sprintf("Get All Labels API Response Status Code: %d \r\n", apiResp.StatusCode))
		utils.Log(0, fmt.Sprintf("Get All Labels API Response Body: \r\n %s \r\n", apiResp.RespBody))
	}

	srcwkld := make(map[string]bool)
	dstwkld := make(map[string]bool)
	for _, ta := range traffic {
		if ta.Src.Workload != nil {
			if ta.Dst.Workload != nil {
				if ta.Src.Workload.GetApp(labelmap).Value == ta.Dst.Workload.GetApp(labelmap).Value && ta.Src.Workload.GetEnv(labelmap).Value == ta.Dst.Workload.GetEnv(labelmap).Value {
					if !ignoreloc {
						if ta.Src.Workload.GetLoc(labelmap).Value == ta.Dst.Workload.GetLoc(labelmap).Value {
							srcwkld[ta.Src.Workload.Href] = true
							dstwkld[ta.Dst.Workload.Href] = true
						}

					} else {
						srcwkld[ta.Src.Workload.Href] = true
						dstwkld[ta.Dst.Workload.Href] = true
					}
				} else {
					if !srcwkld[ta.Src.Workload.Href] {
						srcwkld[ta.Src.Workload.Href] = false
					}
					if !dstwkld[ta.Dst.Workload.Href] {
						dstwkld[ta.Dst.Workload.Href] = false
					}
				}
			} else if !srcwkld[ta.Src.Workload.Href] {
				srcwkld[ta.Src.Workload.Href] = false
			}
		} else if !dstwkld[ta.Dst.Workload.Href] {
			dstwkld[ta.Dst.Workload.Href] = false
		}
	}

	wkldmap, err := illumioapi.GetWkldHrefMap(pce)

	for k, v := range srcwkld {
		if !v {
			fmt.Println(wkldmap[k].Hostname)
		}
	}
	for k, v := range dstwkld {
		if !v {
			fmt.Println(wkldmap[k].Hostname)
		}
	}
}
