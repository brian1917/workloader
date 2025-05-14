package cspiplist

import (
	"fmt"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var csp, ipListUrl string

//var ignoreCase, updatePCE bool

// init initializes the command line flags for the command
func init() {
	CspIplistCmd.Flags().StringVarP(&csp, "csp", "c", "", "Enter which csp (aws, azure, ) you want to get the ip list for.")
	CspIplistCmd.Flags().StringVarP(&ipListUrl, "url", "u", "", "If you want to override the default url for the csp ip list.")

	//CspIplistCmd.Flags().BoolVar(&ignoreCase, "ignore-case", false, "ignore case on the match string.")
	CspIplistCmd.MarkFlagRequired("csp")
	CspIplistCmd.Flags().SortFlags = false
}

// TrafficCmd runs the workload identifier
var CspIplistCmd = &cobra.Command{
	Use:   "csp-iplist",
	Short: "Get the IP list for a given CSP via well-known urls.",
	Long: `
This command will get the IP list for a given CSP via default well-known urls.
The following CSPs are supported:
- AWS
- Azure

You can use the --url flag to override the default url for the csp ip list.

The ip list will be optimized for the given csp by removing duplicates and consolidated consecutive ranges. The ip list will be saves as a csv
file in the current directory. The file will be named <csp>-iplist-<timestamp>.csv. The timestamp is in the format YYYYMMDD-HHMMSS.

`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err := utils.GetTargetPCE(false)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting pce - %s", err.Error()))
		}

		updatePCE := viper.Get("update_pce").(bool)
		noPrompt := viper.Get("no_prompt").(bool)

		cspiplist(updatePCE, noPrompt, csp, ipListUrl, &pce)
	},
}
