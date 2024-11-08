package wkldimport

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldexport"
	"github.com/brian1917/workloader/utils"
)

// userInputConvert takes an ip address in the format of eth0:192.168.20.21 and returns an illumio interface struct
func userInputConvert(ip string) (illumioapi.Interface, error) {

	x := strings.Split(ip, ":")

	// If the length is 1, we know it's IPv4 with no interface name
	if len(x) == 1 {
		nic, err := ipCheck(ip)
		nic.Name = "umwl"
		return nic, err
	}

	// If the length is 2, we know it's an IPv4 address with an interface name
	if len(x) == 2 {
		nic, err := ipCheck(x[1])
		nic.Name = x[0]
		return nic, err
	}

	// If the length is greater than 2, it's an IPv6 address. Check with no name. If err is nil, return it
	nic, err := ipCheck(ip)
	if err == nil {
		nic.Name = "umwl"
		return nic, err
	}

	// Check with name
	nic, err = ipCheck(strings.Join(x[1:], ":"))
	if err == nil {
		nic.Name = x[0]
		return nic, err
	}

	return illumioapi.Interface{}, fmt.Errorf("%s is an invalid ip format", ip)
}

func ipCheck(ip string) (illumioapi.Interface, error) {
	if strings.Contains(ip, "/") {
		ipAddress, ipNet, err := net.ParseCIDR(ip)
		if err == nil {
			cidr, _ := ipNet.Mask.Size()
			return illumioapi.Interface{Address: ipAddress.String(), CidrBlock: &cidr}, nil
		}
	}

	ipAddress := net.ParseIP(ip)
	if ipAddress != nil {
		return illumioapi.Interface{Address: ipAddress.String(), CidrBlock: nil}, nil
	}

	return illumioapi.Interface{}, fmt.Errorf("invalid IP address")
}

// publicIPIsValid validates the ip string is either a valid CIDR or IP address
func publicIPIsValid(ip string) bool {

	if ip == "" {
		return true
	}

	if strings.Contains(ip, "/") {
		_, _, err := net.ParseCIDR(ip)
		return err == nil
	}

	i := net.ParseIP(ip)
	return i != nil

}

func (w *importWkld) interfaces(input Input) {

	// Validate the workload can be updated
	if !(w.wkld.GetMode() == "unmanaged" && ((w.wkld.Agent != nil && w.wkld.Agent.Type != nil && *w.wkld.Agent.Type != "NetworkEnforcementNode") || (w.wkld.VEN != nil && w.wkld.VEN.Name != nil && strings.Contains(*w.wkld.VEN.Name, "Illumio Network Enforcement Node")))) {
		return
	}

	// If IP field is there and  IP address is provided, check it out
	if index, ok := input.Headers[wkldexport.HeaderInterfaces]; ok && len(w.csvLine[index]) > 0 {
		// Build out the netInterfaces slice provided by the user
		netInterfaces := []illumioapi.Interface{}
		nics := strings.Split(strings.Replace(w.csvLine[index], " ", "", -1), ";")
		for _, n := range nics {
			ipInterface, err := userInputConvert(n)
			if err != nil {
				utils.LogWarning(fmt.Sprintf("csv line %d - %s - skipping processing interfaces - ", w.csvLineNum, err.Error()), true)
				return
			}
			netInterfaces = append(netInterfaces, ipInterface)
		}

		// If instructed by flag, make sure we keep all PCE interfaces
		if input.KeepAllPCEInterfaces {
			// Build a map of the interfaces provided by the user with the address as the key
			interfaceMap := make(map[string]illumioapi.Interface)
			for _, i := range netInterfaces {
				interfaceMap[i.Address] = i
			}
			// For each interface on the PCE, check if the address is in the map
			for _, i := range illumioapi.PtrToVal(w.wkld.Interfaces) {
				// If it's not in them map, append it to the user provdided netInterfaces so we keep it
				if _, ok := interfaceMap[i.Address]; !ok {
					netInterfaces = append(netInterfaces, i)
				}
			}
		}

		// Build some maps
		userMap := make(map[string]bool)
		wkldIntMap := make(map[string]bool)
		for _, w := range illumioapi.PtrToVal(w.wkld.Interfaces) {
			cidrText := "nil"
			if w.CidrBlock != nil {
				cidrText = strconv.Itoa(*w.CidrBlock)
			}
			wkldIntMap[w.Address+cidrText+w.Name] = true
		}
		for _, u := range netInterfaces {
			cidrText := "nil"
			if u.CidrBlock != nil {
				cidrText = strconv.Itoa(*u.CidrBlock)
			}
			userMap[u.Address+cidrText+u.Name] = true
		}

		updateInterfaces := false
		// Are all workload interfaces in spreadsheet?
		for _, iFace := range illumioapi.PtrToVal(w.wkld.Interfaces) {
			cidrText := "nil"
			if iFace.CidrBlock != nil && *iFace.CidrBlock != 0 {
				cidrText = strconv.Itoa(*iFace.CidrBlock)
			}
			if !userMap[iFace.Address+cidrText+iFace.Name] {
				updateInterfaces = true
				if w.wkld.Href != "" && input.UpdateWorkloads {
					w.change = true
					utils.LogInfo(fmt.Sprintf("csv line %d - %s - interface not in csv and will be removed - ip: %s, cidr: %s, name: %s", w.csvLineNum, w.compareString, iFace.Address, cidrText, iFace.Name), false)
				}
			}
		}

		// Are all user interfaces on workload?
		for _, u := range netInterfaces {
			cidrText := "nil"
			if u.CidrBlock != nil && *u.CidrBlock != 0 {
				cidrText = strconv.Itoa(*u.CidrBlock)
			}
			if !wkldIntMap[u.Address+cidrText+u.Name] {
				updateInterfaces = true
				if w.wkld.Href != "" && input.UpdateWorkloads {
					w.change = true
					utils.LogInfo(fmt.Sprintf("csv line %d - %s - interface not in pce and will be added - ip: %s, cidr: %s, name: %s", w.csvLineNum, w.compareString, u.Address, cidrText, u.Name), false)
				}
			}
		}

		if updateInterfaces {
			w.wkld.Interfaces = &netInterfaces
		}
	}
}
