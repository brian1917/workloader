package nen

import (
	"fmt"
	"os"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var input wkldimport.Input
var switchupdate, jsontype, createUMWL bool
var nenname, devicename, addr string

func init() {
	NENSWITCHCmd.Flags().BoolVarP(&jsontype, "ipcidr", "i", false, "Default switch JSON configuration is IPRange. Use this option to enable IPRange.")
	NENSWITCHCmd.Flags().BoolVarP(&switchupdate, "switch-update", "s", false, "If the switch exists but you want to update the interfaces select this option.")
	NENSWITCHCmd.Flags().BoolVarP(&createUMWL, "umwl", "u", false, "Import the Switch workloads as UWMLs.  You will need to also add \"--update-pce\" to complete the import")
	NENSWITCHCmd.Flags().StringVarP(&devicename, "name", "n", "", "Enter the output file for the template processing.")
	NENSWITCHCmd.Flags().StringVarP(&addr, "addr", "a", "", "Name of the NEN switch device you want to create an ACL file for")
	NENSWITCHCmd.Flags().StringVarP(&nenname, "nen-name", "", "", "Name of the NEN that you want to build a Switch on.  If left blank you must have only one NEN attached to PCE")

}

// NenCmd builds a file with ACL information
var NENSWITCHCmd = &cobra.Command{
	Use:   "nen-switch",
	Short: "Create NEN switch with workloads attached to interfaces.  Requires interface to workload file.",
	Long: `
Provides the user ability to add a NEN switch to the PCE for pulling JSON policy.

The switch requires UMWLs attached to the configured interfaces to be able to build JSON policy.  The 'nen-acl' command is used
to pull the JSON policy. The requirements for the switch is a name, an unique IP address, and type of IP address output (IP range or IP Cidr).
The command can build the UMWLs for the switch if the 'umwl' flag is set.  The 'update-pce' flag must also be set to update the 
PCE to create the switch or the UMWLs. The 'nen-name' flag is required if there is more than one NEN configured on the PCE.
the switch.csv file is required to have the following columns: interface, href, hostname, ip address, labels.  The 'interface' column is required.
The 'href' column is optional.  If the 'href' column is not provided the 'hostname' or 'name' column is used to find the workload.  If the 'hostname' column is not

T

interface, href, hostname(or name), ip address, labels, 

`,
	Run: func(cmd *cobra.Command, args []string) {

		var err error
		input.PCE, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		input.ImportFile = args[0]

		ImportXLS(input.ImportData)

		//Check to see if you want to create UMWL using the XLS provided.
		if createUMWL {
			// Get the services
			// Get the debug value from viper
			input.UpdatePCE = viper.Get("update_pce").(bool)
			input.NoPrompt = viper.Get("no_prompt").(bool)
			input.Umwl = true
			input.MaxCreate = -1
			input.MaxUpdate = -1
			input.MatchString = wkldexport.HeaderHostname
			wkldimport.ImportWkldsFromCSV(input)
		}
		// Load the PCE with workloads
		apiResps, err := input.PCE.Load(illumioapi.LoadInput{Workloads: true, NetworkEnforcementNode: true}, utils.UseMulti())
		utils.LogMultiAPIRespV2(apiResps)
		if err != nil {
			utils.LogError(err.Error())
		}

		//Use Golang templates to transform NEN JSON object to any output.

		newSwitch := CreateSwitch()

		AttachInterfaces(newSwitch)

	},
}

// GetXLSWklds -
func ImportXLS(data [][]string) {
	// Parse the CSV File
	var err error
	if len(data) == 0 {
		data, err = utils.ParseCSV(input.ImportFile)
		if err != nil {
			utils.LogError(err.Error())
		}
	}
	input.ProcessHeaders(data[0])

	//Check to see that you have switchinterface as column in the XLS as well as href.
	found := false
	hrefpresent := false
	for key := range input.Headers {
		if key == "switchinterface" {
			found = true
			continue
		}
		if key == wkldexport.HeaderHref {
			hrefpresent = true
			continue
		}
	}
	if !found {
		utils.LogError("Error:  Input csv must have a header with the text \"switchinterface\"")
	}

	//If Href not present add it to the end of the Headers Map with a value 1 greater than last column value
	if !hrefpresent {
		input.Headers[wkldexport.HeaderHref] = len(input.Headers)
	}

	//Add the href column to the XLS data
	for index, row := range data {
		if index == 0 {
			data[index] = append(row, "href")
		} else {
			data[index] = append(row, "")
		}
	}

	input.ImportData = data

}

// CreaetSwitch -
func CreateSwitch() illumioapi.NetworkDevice {

	if _, ok := input.PCE.NetworkEnforcementNode[nenname].NetworkDevice[devicename]; ok {
		if !switchupdate {
			utils.LogError("Error:  There is already a switch with that name already attached to the NEN.  Please provide another new unique name.")
		}
		utils.LogWarning("Error:  There is already a switch with that name already attached to the NEN.  Please provide another new unique name.", true)
		return input.PCE.NetworkEnforcementNode[nenname].NetworkDevice[devicename]
	}

	//Check to make sure you dont have 2 or more NENs and havent specified the nenname as an option
	allnens := ""
	if len(input.PCE.NetworkEnforcementNodeSlice) > 1 && nenname == "" {
		for i, nen := range input.PCE.NetworkEnforcementNodeSlice {
			if i > 0 {
				allnens = allnens + ","
			}
			allnens = allnens + nen.Hostname
		}
		utils.LogError("Error:  You  have more than one NEN configured on the PCE.  You must specify the NEN \"--nenname\" ")
	}

	//Make sure you have at least 1 NEN.
	if len(input.PCE.NetworkEnforcementNodeSlice) == 0 {
		utils.LogError("Error:  No NEN configured on the PCE.  You must first register a NEN to be able to build a switch configuration")
	}

	//Make sure the nenname is a valid name for the NEN.
	if input.PCE.NetworkEnforcementNode[nenname].Hostname != nenname {
		utils.LogError(fmt.Sprintf("Error: The specified NEN name %s was not found.  Check the NEN and try again", nenname))
	}

	var tmpNetworkDeviceRequest illumioapi.NetworkDeviceRequest
	if jsontype {
		tmpNetworkDeviceRequest = illumioapi.NetworkDeviceRequest{
			Name:         devicename,
			Manufacturer: "Generic",
			DeviceType:   "switch",
			IPAddress:    addr,
			Model:        "Workload IP Ranges (JSON)",
		}
	} else {
		tmpNetworkDeviceRequest = illumioapi.NetworkDeviceRequest{
			Name:         devicename,
			Manufacturer: "Generic",
			DeviceType:   "switch",
			IPAddress:    addr,
			Model:        "Workload IP Cidrs (JSON)",
		}
	}

	tmp := input.PCE.NetworkEnforcementNode[nenname]
	var tmpnd illumioapi.NetworkDevice
	api, err := tmp.AddNetworkDevice(&input.PCE, tmpNetworkDeviceRequest, &tmpnd)
	if err != nil {
		utils.LogError(err.Error() + api.RespBody)
	}

	return tmpnd

}

func AttachInterfaces(newSwitch illumioapi.NetworkDevice) {

	for i, row := range input.ImportData {
		if i == 0 {
			continue
		}

		tmpEndpoint := illumioapi.NetworkEndpointRequest{
			Config: struct {
				EndpointType      string `json:"endpoint_type"`
				Name              string `json:"name"`
				TrafficFlowID     string `json:"traffic_flow_id,omitempty"`
				WorkloadDiscovery bool   `json:"workload_discovery,omitempty"`
			}{
				Name:         row[input.Headers["switchinterface"]],
				EndpointType: "switch_port"},
			Workloads: []struct {
				Href string `json:"href,omitempty"`
			}{},
		}

		//If there is a value in href column add it to the NetworkEndpoint structure. Otherwise find hostname or name.
		tmpstr := input.PCE.Workloads[row[input.Headers[wkldexport.HeaderName]]].Href
		if tmpstr == "" {
			tmpstr = input.PCE.Workloads[row[input.Headers[wkldexport.HeaderHostname]]].Href
		}

		if row[input.Headers[wkldexport.HeaderHref]] != "" {
			tmpstr = row[input.Headers[wkldexport.HeaderHref]]
		}

		haswkld := true
		if tmpstr != "" {
			tmpEndpoint.Workloads = append(tmpEndpoint.Workloads, struct {
				Href string `json:"href,omitempty"`
			}{Href: tmpstr})
		} else {
			haswkld = false
		}

		//Check to see if the switchinterface value is already attached to the switch and you want to update your switch
		if _, ok := input.PCE.NetworkEnforcementNode[newSwitch.NetworkEnforcementNode.Href].NetworkDevice[newSwitch.Href].NetworkEndpoint[row[input.Headers["switchinterface"]]]; ok && switchupdate {
			tmpEndpoint.Href = input.PCE.NetworkEnforcementNode[newSwitch.NetworkEnforcementNode.Href].NetworkDevice[newSwitch.Href].NetworkEndpoint[row[input.Headers["switchinterface"]]].Href
			api, err := input.PCE.UpdateNetworkEndpoint(&tmpEndpoint)
			if err != nil {
				utils.LogWarning(err.Error()+" "+tmpEndpoint.Config.Name+" "+api.RespBody, true)
			}
			//Check to see if you dont have a workload from the csv.  If not skip updating or adding.
		} else if haswkld {
			//if there is a workload check to see if its already attached to the switch.  If not add it.
			if _, ok := input.PCE.NetworkEnforcementNode[newSwitch.NetworkEnforcementNode.Href].NetworkDevice[newSwitch.Href].NetworkEndpoint[tmpEndpoint.Workloads[0].Href]; !ok && tmpEndpoint.Href == "" && switchupdate {
				api, err := newSwitch.AddNetworkEndpoint(&input.PCE, &tmpEndpoint)
				if err != nil {
					utils.LogWarning(err.Error()+" "+tmpEndpoint.Config.Name+" "+api.RespBody, true)
				}
				//If there is a switch interface with the workload but the interfaces is new use the original switch interfaces and update.
			} else {
				tmpEndpoint.Href = input.PCE.NetworkEnforcementNode[newSwitch.NetworkEnforcementNode.Href].NetworkDevice[newSwitch.Href].NetworkEndpoint[tmpEndpoint.Workloads[0].Href].Href
				api, err := input.PCE.UpdateNetworkEndpoint(&tmpEndpoint)
				if err != nil {
					utils.LogWarning(err.Error()+" "+tmpEndpoint.Config.Name+" "+api.RespBody, true)
				}
			}
			//If there is no workload but there is an interface name just create the interface.
		} else {
			api, err := newSwitch.AddNetworkEndpoint(&input.PCE, &tmpEndpoint)
			if err != nil {
				utils.LogWarning(err.Error()+" "+tmpEndpoint.Config.Name+" "+api.RespBody, true)
			}
		}

	}

}
