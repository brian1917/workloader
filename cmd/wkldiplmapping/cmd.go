package wkldiplmapping

import (
	"bytes"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

type input struct {
	pce                 illumioapi.PCE
	outputFileName      string
	managedOnly         bool
	unmanagedOnly       bool
	role, app, env, loc string
	skipIPLs            string
	labelFile           string
}

var in input
var err error

func init() {
	WkldIPLMappingCmd.Flags().BoolVarP(&in.managedOnly, "managed-only", "m", false, "Only export managed workloads.")
	WkldIPLMappingCmd.Flags().BoolVarP(&in.unmanagedOnly, "unmanaged-only", "u", false, "Only export unmanaged workloads.")
	WkldIPLMappingCmd.Flags().StringVarP(&in.skipIPLs, "skip-iplists", "s", "", "semi-colon separated list of IP Lists to skip matching. Any (0.0.0.0/0 and ::/0) is always skipped.")
	WkldIPLMappingCmd.Flags().StringVarP(&in.role, "role", "r", "", "role label value. label flags are an \"and\" operator.")
	WkldIPLMappingCmd.Flags().StringVarP(&in.app, "app", "a", "", "app label value. label flags are an \"and\" operator.")
	WkldIPLMappingCmd.Flags().StringVarP(&in.env, "env", "e", "", "env label value. label flags are an \"and\" operator.")
	WkldIPLMappingCmd.Flags().StringVarP(&in.loc, "loc", "l", "", "loc label value. label flags are an \"and\" operator.")
	WkldIPLMappingCmd.Flags().StringVar(&in.labelFile, "label-file", "", "csv file with labels to filter query. the file should have 4 headers: role, app, env, and loc. The four columns in each row is an \"AND\" operation. Each row is an \"OR\" operation.")
	WkldIPLMappingCmd.Flags().StringVar(&in.outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	WkldIPLMappingCmd.Flags().SortFlags = false

}

// WkldExportCmd runs the workload identifier
var WkldIPLMappingCmd = &cobra.Command{
	Use:   "wkld-ipl-mapping",
	Short: "Create a CSV export showing how a workload maps to IP lists.",
	Long: `
Create a CSV export showing how a workload maps to IP lists.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		in.pce, err = utils.GetTargetPCE(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		wkldToIPLMapping(in)
	},
}

func ipCheck(ip string, ipl illumioapi.IPList) (bool, error) {
	providedIP := net.ParseIP(ip)
	if providedIP == nil {
		return false, fmt.Errorf("%s is not a valid IP address", ip)
	}

	for _, ipRange := range ipl.IPRanges {

		// If it's a valid CIDR and the IP address falls into it return true
		_, network, err := net.ParseCIDR(ipRange.FromIP)
		if err == nil && network.Contains(providedIP) {
			return true, nil
		}

		// Get here if FromIP is not a CIDR
		fromIP := net.ParseIP(ipRange.FromIP)
		toIP := net.ParseIP(ipRange.ToIP)
		if toIP == nil {
			toIP = fromIP
		}
		if bytes.Compare(providedIP, fromIP) >= 0 && bytes.Compare(providedIP, toIP) <= 0 {
			return true, nil
		}

	}
	return false, nil
}

func wkldToIPLMapping(input input) {

	// Log start
	utils.LogStartCommand("wkld-ipl-mapping")

	// Load the PCE
	if err := input.pce.Load(illumioapi.LoadInput{Labels: true, IPLists: true, Workloads: false}); err != nil {
		utils.LogError(err.Error())
	}

	// Process skippedIPLs
	skipIPLs := map[string]bool{"Any (0.0.0.0/0 and ::/0)": true}
	for _, s := range strings.Split(strings.Replace(input.skipIPLs, " ", "", -1), ";") {
		skipIPLs[s] = true
	}

	// Get all workloads
	qp := make(map[string]string)
	if input.unmanagedOnly {
		qp["managed"] = "false"
	}
	if input.managedOnly {
		qp["managed"] = "true"
	}

	// Process the file if provided
	if input.labelFile != "" {
		// Parse the CSV
		labelData, err := utils.ParseCSV(input.labelFile)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the labelQuery
		qp["labels"], err = input.pce.WorkloadQueryLabelParameter(labelData)
		if err != nil {
			utils.LogError(err.Error())
		}

	} else {
		providedValues := []string{input.role, input.app, input.env, input.loc}
		keys := []string{"role", "app", "env", "loc"}
		queryLabels := []string{}
		for i, labelValue := range providedValues {
			// Do nothing if the labelValue is blank
			if labelValue == "" {
				continue
			}
			// Confirm the label exists
			if label, ok := input.pce.Labels[keys[i]+labelValue]; !ok {
				utils.LogError(fmt.Sprintf("%s does not exist as a %s label", labelValue, keys[i]))
			} else {
				queryLabels = append(queryLabels, label.Href)
			}

		}

		// If we have query labels add to the map
		if len(queryLabels) > 0 {
			qp["labels"] = fmt.Sprintf("[[\"%s\"]]", strings.Join(queryLabels, "\",\""))
		}
	}

	if len(qp["labels"]) > 10000 {
		utils.LogError(fmt.Sprintf("the query is too large. the total character count is %d and the limit for this command is 10,000", len(qp["labels"])))
	}
	wklds, a, err := input.pce.GetAllWorkloadsQP(qp)
	utils.LogAPIResp("GetAllWorkloadsQP", a)
	if err != nil {
		utils.LogError(fmt.Sprintf("getting all workloads - %s", err))
	}

	csvData := [][]string{{"hostname", "interfaces", "matching_iplists", "policy_state", "role", "app", "env", "loc"}}

	// Iterate through all workloads
	for _, wkld := range wklds {

		// Create a slice to hold matched IP List names
		matchedIPLists := make(map[string]bool)

		// Iterate through each interface on that workload
		for _, netInt := range wkld.Interfaces {

			// Check each IP list to see if it fits in it
			for _, ipList := range input.pce.IPListsSlice {
				if skipIPLs[ipList.Name] {
					continue
				}
				check, err := ipCheck(netInt.Address, ipList)
				if err != nil {
					utils.LogError(err.Error())
				}
				if check {
					matchedIPLists[ipList.Name] = true
				}
			}
		}

		// Check if we have matches and append to our CSV output
		if len(matchedIPLists) > 0 {
			// Create a slice for matched
			var s []string
			for m := range matchedIPLists {
				s = append(s, m)
			}

			// Get interfaces
			interfaces := []string{}
			for _, i := range wkld.Interfaces {
				ipAddress := fmt.Sprintf("%s:%s", i.Name, i.Address)
				if i.CidrBlock != nil && *i.CidrBlock != 0 {
					ipAddress = fmt.Sprintf("%s:%s/%s", i.Name, i.Address, strconv.Itoa(*i.CidrBlock))
				}
				interfaces = append(interfaces, ipAddress)
			}
			csvData = append(csvData, []string{wkld.Hostname, strings.Join(interfaces, ";"), strings.Join(s, ";"), wkld.GetMode(), wkld.GetRole(input.pce.Labels).Value, wkld.GetApp(input.pce.Labels).Value, wkld.GetEnv(input.pce.Labels).Value, wkld.GetLoc(input.pce.Labels).Value})
		}
	}

	// Write the CSV data
	if len(csvData) > 1 {
		if input.outputFileName == "" {
			input.outputFileName = fmt.Sprintf("workloader-wkld-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, input.outputFileName)
		utils.LogInfo(fmt.Sprintf("%d mapped workloads exported", len(csvData)-1), true)
	} else {
		utils.LogInfo("no mapped workloads", true)
	}
	utils.LogEndCommand("wkld-ipl-mapping")

}
