package cspiplist

import (
	"fmt"
	"os"

	ia "github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pce ia.PCE
var err error

var csp, ipListUrl, fileName, iplName string
var testIPs, includev6 bool

//var ignoreCase, updatePCE bool

// init initializes the command line flags for the command
func init() {
	CspIplistCmd.Flags().StringVarP(&csp, "csp", "c", "", "Enter which csp (aws, azure, file) you want to get the ip list for.")
	CspIplistCmd.Flags().StringVarP(&ipListUrl, "url", "u", "", "If you want to override the default url for the csp ip list.")
	CspIplistCmd.Flags().BoolVarP(&testIPs, "test-ips", "t", false, "After consolidating/merging all the IP ranges validate that original subnets are part of some IP range.")
	CspIplistCmd.Flags().BoolVarP(&includev6, "ipv6", "", false, "Include ipv6 addresses. By default all ipv6 will be ignored.")
	CspIplistCmd.Flags().StringVarP(&fileName, "filename", "f", "", "Include filename if you enter \"file\" for as csp option.")
	//CspIplistCmd.Flags().StringVarP(&iplName, "iplname", "i", "", "Name of Iplist on the PCE.")

	CspIplistCmd.MarkFlagRequired("csp")
	CspIplistCmd.Flags().SortFlags = false
}

// TrafficCmd runs the workload identifier
var CspIplistCmd = &cobra.Command{
	Use:   "csp-iplist",
	Short: "Add/Update an IPList that consist of CSP ip ranges gathered from CSP website.",
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

		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting pce - %s", err.Error()))
		}

		updatePCE := viper.Get("update_pce").(bool)
		noPrompt := viper.Get("no_prompt").(bool)

		// Set the CSV file
		if len(args) > 1 {
			fmt.Println("command only accepts 1 or no arguments for the ip list name. See usage help.")
			os.Exit(0)
		}
		iplName = ""
		if len(args) > 0 {
			iplName = args[0]
		} else {
			utils.LogError("Please provide a name for the IP list.")
		}

		cspiplist(updatePCE, noPrompt, csp, ipListUrl, iplName, &pce)
	},
}
