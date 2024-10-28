package nen

import (
	"fmt"
	"hash/crc64"
	"html/template"
	"net"
	"os"
	"path/filepath"
	"sort"
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
var days, timeout int

func init() {
	NENACLCmd.Flags().IntVarP(&days, "days", "d", 7, "How old can the switch ACL be before rebuilding( use =0 to rebuild)?")
	NENACLCmd.Flags().IntVarP(&timeout, "timeout", "t", 10, "How many minutes to wait for ACL to be built before timeout and exit?")
	NENACLCmd.Flags().StringVarP(&outFile, "output-file", "o", "", "Enter the output file for the template processing.")
	NENACLCmd.Flags().StringVarP(&networkDeviceName, "name", "n", "", "Name of the NEN switch device you want to create an ACL file for")
}

// NenCmd builds a file with ACL information
var NENACLCmd = &cobra.Command{
	Use:   "nen-acl",
	Short: "Create NEN ACL file.  Requires a golang template file.",
	Long: `
Create output file for different types of enforcement network equipment using devices native syntax.  Requires a template files using Golang templating language

Requires the name of the Switch configured on the NEN using --name or -n <switch name>.  By default looks for an ACL file already created within the last 7 days.  
Can select a smaller time frame or use -d or --days 0 to re-create a new ACL. Will build the ACL based on the template file included as an argument.  
Additionally, you can send the output to stdout or to a file using --output-file or -o <filename>.

Golang template has been extended to include some basic functions like:
{{ add <value A> <value B>}} so you can create incrementing values.  
included in the ip value.  The mask can be traditional notation of 255.255.255.0 = /24 or you can invert 0.255.255.255 by setting inv = true.  {{mask "10.0.0.0/8" true}}.  
{{mask <ip with mask> boolean}} To calulate the mask of the IP address.  setting boolean to true inverses the mask.
{{ipclean string}} To remove the prefix from the IP address. 
{{splitrange string boolean}} To split a range of IPs and return the first or second value. 

The Nen Switch configuration and policy data structure is what the template uses to create the output.  


type SwitchACLData struct {        //array of ACLs for switch configured on NEN 
	BaseSwitch BaseSwitchData []struct {
	Name          string   //name of the workload on the interface
	IntfName      string   //name of the interface
	Href          string   //href of the workload
	Ips           []string //array of all workload IP address
	SetsRuleCount int      //number of rules that PCE sends natively using sets
	RuleCount     int      //number of rules if not supporting sets.
	Rules         struct {
		Outbound []struct {
			Action      string   //permit or deny
			Port        string   //permit or deny
			Port        string 	 //udp or tcp port number
			ProtocolNum string   //udp or tcp protocol as a number
			ProtocolTxt string	 //udp or tcp protocol as text
			Ips         []string //array of all the dst IPs
			OutHash     uint64   //name of the unique hash of the Ips array as a string
		} 
		Inbound []struct {
			Action      string   //permit or deny
			Port        string 	 //udp or tcp port number
			ProtocolNum string   //udp or tcp protocol number
			ProtocolTxt string	 //udp or tcp protocol as txt
			Ips         []string //array of all the src IPs
			InHash      uint64   //name of the unique hash of the Ips array as a string
		} 
	} 			
}
`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the golang template file. See usage help.")
			os.Exit(0)
		}
		templateFile = args[0]

		// Get the services
		pce.Load(illumioapi.LoadInput{Workloads: true, NetworkEnforcementNode: true}, utils.UseMulti())

		//Make sure you have a switch name to build the ACL for.
		if networkDeviceName == "" {
			utils.LogError("You must enter the name of the switch you want to create an ACL file for.")
		}
		//Use Golang templates to transform NEN JSON object to any output.
		TranslateSwitchPolicy()

	},
}

// ProtocolNumToText - way to take the TCPIP protocol number to a the well know protocol name.
func ProtocolPortNumToText(numericProtocl, port string) (string, string) {

	switch numericProtocl {
	case "6":
		return "tcp", port
	case "17":
		return "udp", port
	case "1":
		if port == "*" {
			return "icmp", ""
		} else {
			return "icmp", port
		}
	default:
		return "any", ""
	}
}

// BuildAndWaitForACLData - function that will use the HREF of the Network device to see if the ACL has been built looking
// at to see if EnforcementInstructionsGenerationInProgress is true which means we need to wait 3 seconds and check again.
// Return the HREF of the ACL object to get.
func BuildAndWaitForACLData(nd *illumioapi.NetworkDevice) string {

	nd.RequestNetworkDeviceACL(&pce)
	count := 0
	for {
		var tmpnd illumioapi.NetworkDevice
		api, err := pce.GetNetworkDevice(nd.Href, &tmpnd)
		utils.LogAPIRespV2("BuildAndWaitForACLData", api)
		if err != nil {
			utils.LogError(err.Error())

		}
		time.Sleep(3 * time.Second)
		// If the NEN is finished building the ACL Data return the Href to it.
		if !tmpnd.EnforcementInstructionsGenerationInProgress {
			return tmpnd.EnforcementInstructionsDataHref
		}
		//Currently wait var.timeout * 60 seconds before you time out.
		count++
		if count >= (timeout * 60) {
			utils.LogError("Timed out....Waited 10 minutes for JSON ACL to be calculated")
		}
	}

}

// GetHref - This function gets the URL sent as the variable.
func GetHref(href string, data interface{}, dataType string) {
	// Get the NetworkDevice aka switch data from PCE.
	api, err := pce.GetHref(href, &data)
	utils.LogAPIRespV2(dataType, api)
	if err != nil {
		utils.LogError(err.Error())
	}
}

// GetNetworkEndpointData - Gets the switch port info so you can discover interface name and other endpoint configuration
func GetNetworkEndpointData(href string) map[string]IntfConfig {

	var switchConf BaseSwitchConfig
	intConf := make(map[string]IntfConfig)
	href = href + "/network_endpoints"
	GetHref(href, &switchConf, "GetNetworkDeviceACLData")
	for _, networkEndpoint := range switchConf {
		if len(networkEndpoint.Workloads) != 0 {
			for _, workload := range networkEndpoint.Workloads {
				intConf[workload.Href] = networkEndpoint.Config
			}
		}
	}

	return intConf
}

// GetNetworkDeviceACLData - This function gets the data structure of the ACL and adds more data to it so using
// golang templates can create any type of ACL syntax for any type of device.
func GetNetworkDeviceACLData(dataHref string, switchConf map[string]IntfConfig) (switchAclData SwitchACLData) {

	var wkldAcl BaseSwitchData

	GetHref(dataHref, &wkldAcl, "GetNetworkDeviceACLData")

	//initialize the maps and hash variables.
	switchAclData.HashList = map[uint64][]string{}
	protoPort := map[string]ProtoPort{}
	//newhash := maphash.Hash{}
	table := crc64.MakeTable(crc64.ISO)

	//Cycle through all the workloads configured on the switch and get some additional metadata to help with
	//writing different policy definitions for different devices (PAN, Forinet, Juniper, Switches....)
	for index, each := range wkldAcl {
		var ips []string

		//add the interface name on the switch to the ACL data
		wkldAcl[index].IntfName = switchConf[each.Href].Name

		//Cycle through each switch port's workload and get all the IPs.
		for _, intf := range *pce.Workloads[each.Href].Interfaces {
			ips = append(ips, intf.Address)
		}

		//get the ips of the UMWL on the switch port and populate the object
		wkldAcl[index].Ips = ips
		//count number of if you only use the sets not individual IPs in a single rule.
		wkldAcl[index].SetsRuleCount = len(each.Rules.Inbound) + len(each.Rules.Outbound)
		//Build a set of different services across the entire switch.

		for ruleindex, inbound := range each.Rules.Inbound {

			proto, port := ProtocolPortNumToText(inbound.ProtocolNum, inbound.Port)
			//count all rules for inbound by finding all dst ips * number of interfaces on the device
			wkldAcl[index].RuleCount = wkldAcl[index].RuleCount + (len(inbound.Ips) * len(*pce.Workloads[each.Href].Interfaces))
			wkldAcl[index].Rules.Inbound[ruleindex].ProtocolTxt = proto
			wkldAcl[index].Rules.Inbound[ruleindex].Port = port

			//Sort array and reset the hash to keep Seed the same but zero out the hash itself
			sort.Strings(inbound.Ips)

			// Calculate the CRC-64 hash
			hash := crc64.Checksum([]byte(fmt.Sprint(inbound.Ips)), table)

			//assign hash to inbound Ips and add the hash to the data struct with the inbound Ips
			wkldAcl[index].Rules.Inbound[ruleindex].InHash = hash

			switchAclData.HashList[hash] = inbound.Ips

			if inbound.Port == "*" && inbound.ProtocolNum == "*" {
				continue
			}
			//populate a set of Services found in all the rules for that switch.  Add that to the data available to Golang template

			protoPort[proto+port] = ProtoPort{Port: inbound.Port, ProtocolNum: inbound.ProtocolNum, ProtocolTxt: proto}

		}

		for ruleindex, outbound := range each.Rules.Outbound {
			proto, port := ProtocolPortNumToText(outbound.ProtocolNum, outbound.Port)
			//count all rules for outbound by finding all src ips * number of interfaces on the device
			wkldAcl[index].RuleCount = wkldAcl[index].RuleCount + (len(outbound.Ips) * len(*pce.Workloads[each.Href].Interfaces))
			wkldAcl[index].Rules.Outbound[ruleindex].ProtocolTxt = proto
			wkldAcl[index].Rules.Outbound[ruleindex].Port = port

			//Sort array and reset the hash to keep Seed the same but zero out the hash itself
			sort.Strings(outbound.Ips)
			hash := crc64.Checksum([]byte(fmt.Sprint(outbound.Ips)), table)

			//assign hash to inbound Ips and add the hash to the data struct with the inbound Ips
			wkldAcl[index].Rules.Outbound[ruleindex].OutHash = hash
			switchAclData.HashList[hash] = outbound.Ips

			if outbound.Port == "*" && outbound.ProtocolNum == "*" {
				continue
			}
			//populate a set of Services found in all the rules for that switch.  Add that to the data available to Golang template
			protoPort[proto+port] = ProtoPort{Port: port, ProtocolNum: outbound.ProtocolNum, ProtocolTxt: proto}

		}
	}
	switchAclData.BaseSwitch = wkldAcl
	switchAclData.ProtoPort = protoPort
	return switchAclData
}

// GetMask - Function gets a string that has a prefix included in the ip string ("172.16.0.0/16").  It creates the long for of that IP and prefix.
// 172.16.0.0 255.255.0.0.   By passing true as the second variable the mask is inverted which looks like 172.16.0.0 0.0.255.255.  This is sometimes
// needed for routers and switches.
func GetMask(ip string, inv bool) string {
	return (GetIPMask(ip, inv, true))
}

// GetIP - Function gets a string of the IP address without the prefix.
func GetIP(ip string) string {
	return (GetIPMask(ip, true, false))
}

// GetMask - Function gets a string that has a prefix included in the ip string ("172.16.0.0/16").  It creates the long for of that IP and prefix.
// 172.16.0.0 255.255.0.0.   By passing true as the second variable the mask is inverted which looks like 172.16.0.0 0.0.255.255.  This is sometimes
// needed for routers and switches.
func GetIPMask(ip string, inv bool, mask bool) string {

	ipv4, ipv4Net, err := net.ParseCIDR(ip)
	if err != nil {
		utils.LogError("Trying to parse the IPaddress for prefix.  Make sure you are using IPCidr vs IPRange")
	}
	ipmask := ipv4Net.Mask
	if inv {
		for i, b := range ipv4Net.Mask {
			ipmask[i] = ^b
		}
	}
	if mask {
		return net.IP(ipmask).String()
	} else {
		return net.IP(ipv4).String()
	}

}

func SetFuncMap() template.FuncMap {

	//this adds the "add" and "mask" function to the golang template functions.  Used for creating increasing names....
	funcMap := template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"mask": func(ip string, inv bool) string {
			return (GetMask(ip, inv))
		},
		"ipclean": func(ip string) string {
			return (GetIP(ip))
		},
		"replace": func(s, old, new string) string {
			return strings.ReplaceAll(s, old, new)
		},
		"hasdash": func(s string) bool {
			return strings.Contains(s, "-")
		},
		"splitrange": func(tmp string, to bool) string {
			if to {
				return strings.Split(tmp, "-")[1]
			} else {
				return strings.Split(tmp, "-")[0]
			}

		},
	}
	return funcMap
}

// TranslateSwitchPolicy - Takes a PCE created policy and translates that to a specific format that the users specifies
func TranslateSwitchPolicy() {

	workingDevices := make(map[string]string)
	var dataHref string

	match := false
	//Go through all the NEN switches and find only those that are using JSON IP CIDRs or JSON IP Ranges.  All others ignored
	for _, nen := range pce.NetworkEnforcementNodeSlice {
		for _, nd := range nen.NetworkDeviceSlice {

			if nd.Config.Name == networkDeviceName && (strings.Contains(nd.Config.Model, "Workload IP Cidrs (JSON)") || strings.Contains(nd.Config.Model, "Workload IP Ranges (JSON)")) {
				//remove "/orgs/" in the href of the nen.
				workingDevices[nd.Config.Name] = strings.TrimPrefix(nd.Href, fmt.Sprintf("/orgs/%d/", pce.Org))

				//check to see if switch ACL was build in the configured number of days...if not build new ACL.  If set to 0 then build no matter the number of days.
				now := time.Now()
				if days == 0 || nd.EnforcementInstructionsDataHref == "" || nd.EnforcementInstructionsDataTimestamp.Before(now.AddDate(0, 0, -days)) {
					dataHref = BuildAndWaitForACLData(&nd)
				} else {
					dataHref = nd.EnforcementInstructionsDataHref
				}

				switchConf := GetNetworkEndpointData(nd.Href)
				switchACL := GetNetworkDeviceACLData(dataHref, switchConf)
				if err != nil {
					utils.LogError(err.Error())
				}

				//Found I needed just the base filename for part of the Template parsing command
				//Parse template file and load template functions from SetFuncMap() to build the output
				tmpl, err := template.New(filepath.Base(templateFile)).Funcs(SetFuncMap()).ParseFiles(templateFile)
				if err != nil {
					utils.LogError(err.Error())
				}

				//If not output file then send to screen
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
				match = true
				break

			} else {
				continue
			}
		}
	}
	if !match {
		utils.LogError("No match found for switch names \"" + networkDeviceName + "\".  Check names of switches in the PCE")
	}
}
