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

var csp, ipListUrl, fileName, iplName, iplCsvFile, cspFilter string
var testIPs, includev6, create, provision bool

//var ignoreCase, updatePCE bool

// init initializes the command line flags for the command
func init() {
	CspIplistCmd.Flags().StringVarP(&csp, "csp", "", "", "Enter which csp (aws, azure, gcp,file) you want to get the ip list for.")
	CspIplistCmd.Flags().StringVarP(&ipListUrl, "url", "u", "", "If you want to override the default url for the csp ip list.")
	CspIplistCmd.Flags().BoolVarP(&testIPs, "test-ips", "t", false, "After consolidating/merging all the IP ranges validate that original subnets are part of some IP range.")
	CspIplistCmd.Flags().BoolVarP(&includev6, "ipv6", "", false, "Include ipv6 addresses. By default all ipv6 will be ignored.")
	CspIplistCmd.Flags().StringVarP(&fileName, "filename", "f", "", "Include filename if you enter \"file\" for as csp option.")
	CspIplistCmd.Flags().StringVarP(&cspFilter, "csp-filter", "", "", "Filter filename used filter IP ranges by service and/or region.")
	CspIplistCmd.Flags().BoolVarP(&create, "create", "c", false, "create ip list if it does not exist")
	CspIplistCmd.Flags().BoolVarP(&provision, "provision", "p", false, "provision ip list after replacing contents.")
	CspIplistCmd.MarkFlagRequired("csp")
	CspIplistCmd.Flags().SortFlags = false
}

// TrafficCmd runs the workload identifier
var CspIplistCmd = &cobra.Command{
	Use:   "csp-iplist",
	Short: "Add/Update an IPList that consist of CSP ip ranges gathered from CSP website.",
	Long: `

This command will download the IP list for a given CSP via default, well-known urls.  Workloader will try to create/update the specified IP List with the IP Ranges of the CSP.  There is an option to filter.
the IP ranges by service and/or region. The command will also consolidate the IP ranges by removing duplicates and merging consecutive ranges.  By defalt the command will 
not include ipv6 addresses. If you want to include ipv6 addresses use the --ipv6 flag.   

 		'workloader csp-iplist --csp gcp <ip listname>'  or 'workloader csp-iplist --csp gcp --ipv6 <ip listname>' or 'workloader csp-iplist --csp gcp --csp-filter <filter filename> <ip listname>'
The following CSPs are supported:
- AWS
- Azure
- GCP

You can use the --url flag to override the default url for the csp ip range web location.  You can also use the --filename flag to specify a file that contains the IP ranges instead of accessing CSP IP range data.  It will 
also check for duplicates and consolidate.  By default no changes will be made to the PCE.  Please use --update-pce if you want to make changes.  If the IP List is not configured on the PCE, use the --create flag to create it.

* Azure leaves services that span many regions with a blank region.  This command will set those regions to "GLOBAL" so use "GLOBAL" in your filter file.
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

		cspiplist(&pce, updatePCE, noPrompt, csp, ipListUrl, iplName)
	},
}
