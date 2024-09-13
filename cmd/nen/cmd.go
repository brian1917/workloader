package nen

import (
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var err error
var pce illumioapi.PCE
var templateFile string
var networkDeviceName, outFile string
var days, hours int

func init() {
	NENCmd.Flags().IntVarP(&days, "days", "d", 7, "How many days old before you want to rebuild the switch ACL?")
	NENCmd.Flags().IntVarP(&hours, "hours", "", 1, "How many hours old before you want to rebuild the switch ACL?")
	NENCmd.Flags().StringVarP(&outFile, "output-file", "o", "", "Enter the output file for the template processing.")
	NENCmd.Flags().StringVarP(&networkDeviceName, "name", "n", "", "Name of the NEN switch device you want to create an ACL file for")
}

// NenCmd builds a file with ACL information
var NENCmd = &cobra.Command{
	Use:   "nen",
	Short: "Create NEN ACL file.",
	Long: `
Create output file for different types of enforcement network equipment.
	
Recommended to run without --update-pce first to log of what will change. If --update-pce is used, svc-import will create the services with a  user prompt. To disable the prompt, use --no-prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		templateFile = args[0]

		// Get the services
		pce.Load(illumioapi.LoadInput{Workloads: true, Services: true, NetworkDevice: true}, utils.UseMulti())

		TranslateSwitchPolicy()

	},
}

// ProtocolNumToText - way to take the TCPIP protocol number to a the well know protocol name.
func ProtocolNumToText(numericProtocl string) string {

	switch numericProtocl {
	case "6":
		return "tcp"
	case "17":
		return "udp"
	case "1":
		return "icmp"
	default:
		return "any"
	}
}

// WaitForData
func BuildAndWaitForACLData(nd *illumioapi.NetworkDevice) string {

	nd.RequestNetworkDeviceACL(&pce)
	count := 0
	for {
		var tmpnd illumioapi.NetworkDevice
		api, err := nd.GetNetworkDevice(&pce, &tmpnd)
		utils.LogAPIRespV2("BuildAndWaitForACLData", api)
		if err != nil {
			utils.LogError(err.Error())

		}
		time.Sleep(3 * time.Second)
		// If the NEN is finished building the ACL Data return the Href to it.
		if !tmpnd.EnforcementInstructionsGenerationInProgress {
			return tmpnd.EnforcementInstructionsDataHref
		}
		count++
		if count >= 200 {
			utils.LogError("Timed out....Waited 10 minutes for JSON ACL to be calculated")
		}
	}

}

// GetNetworkDeviceACLData
func GetNetworkDeviceACLData(dataHref string) (switchAclData SwitchACLData, err error) {

	var wkldAcl BaseSwitchACLData
	var api illumioapi.APIResponse

	//Get the datafile from the specific switch and populate in JSON structure.
	api, err = pce.GetHref(dataHref, &wkldAcl)
	utils.LogAPIRespV2("GetNetworkDeviceACLData", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	//Cycle through all the workloads configured on the switch and get some additional metadata to help with
	//writing different policy definitions for different devices (PAN, Forinet, Juniper, Switches....)
	protoPort := make(map[string]ProtoPort)
	for index, each := range wkldAcl {
		var ips []string
		for _, intf := range *pce.Workloads[each.Href].Interfaces {
			ips = append(ips, intf.Address)
		}
		//get the ips of the UMWL on the switch port.
		wkldAcl[index].Ips = ips

		wkldAcl[index].SetsRuleCount = len(each.Rules.Inbound) + len(each.Rules.Outbound)
		//Build a set of different services across the entire switch.
		for ruleindex, inbound := range each.Rules.Inbound {
			//count all rules for inbound by finding all dst ips * number of interfaces on the device
			wkldAcl[index].RuleCount = wkldAcl[index].RuleCount + (len(inbound.Ips) * len(*pce.Workloads[each.Href].Interfaces))
			wkldAcl[index].Rules.Inbound[ruleindex].ProtocolTxt = ProtocolNumToText(inbound.ProtocolNum)
			if inbound.Port == "*" && inbound.ProtocolNum == "*" {
				continue
			}

			protoPort[ProtocolNumToText(inbound.ProtocolNum)+inbound.Port] = ProtoPort{Port: inbound.Port, ProtocolNum: inbound.ProtocolNum, ProtocolTxt: ProtocolNumToText(inbound.ProtocolNum)}
		}
		for ruleindex, outbound := range each.Rules.Outbound {
			//count all rules for outbound by finding all src ips * number of interfaces on the device
			wkldAcl[index].RuleCount = wkldAcl[index].RuleCount + (len(outbound.Ips) * len(*pce.Workloads[each.Href].Interfaces))
			wkldAcl[index].Rules.Outbound[ruleindex].ProtocolTxt = ProtocolNumToText(outbound.ProtocolNum)
			if outbound.Port == "*" && outbound.ProtocolNum == "*" {
				continue
			}
			protoPort[ProtocolNumToText(outbound.ProtocolNum)+outbound.Port] = ProtoPort{Port: outbound.Port, ProtocolNum: outbound.ProtocolNum, ProtocolTxt: ProtocolNumToText(outbound.ProtocolNum)}

		}
	}
	switchAclData.BaseSwitch = wkldAcl
	switchAclData.ProtoPort = protoPort
	return switchAclData, err
}

// TranslateSwitchPolicy - Takes a PCE created policy and translates that to a specific format that the users specifies
func TranslateSwitchPolicy() {

	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
	}

	workingDevices := make(map[string]string)
	var dataHref string

	for _, nd := range pce.NetworkDeviceSlice {

		if nd.Config.Name == networkDeviceName && (nd.Config.Model == "Workload IP Cidrs (JSON)" || nd.Config.Model == "Workload IP Ranges (JSON)") {
			workingDevices[nd.Config.Name] = strings.TrimPrefix(nd.Href, fmt.Sprintf("/orgs/%d/", pce.Org))

			if days == 0 || nd.EnforcementInstructionsDataHref == "" || nd.EnforcementInstructionsDataTimestamp.Before(time.Now().AddDate(0, 0, -days)) {
				dataHref = BuildAndWaitForACLData(&nd)
			} else {
				dataHref = nd.EnforcementInstructionsDataHref
			}
			switchACL, err := GetNetworkDeviceACLData(dataHref)
			if err != nil {
				utils.LogError(err.Error())
			}
			tmpl, err := template.New(templateFile).Funcs(funcMap).ParseFiles(templateFile)
			if err != nil {
				utils.LogError(err.Error())
			}
			writer := os.Stdout
			if outFile != "" {
				writer, err = os.Create(outFile)
				if err != nil {
					utils.LogError(err.Error())
				}
			}

			err = tmpl.Execute(writer, switchACL)
			if err != nil {
				utils.LogError(err.Error())
			}

		} else {
			continue
		}

	}
}
