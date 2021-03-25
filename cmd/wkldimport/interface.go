package wkldimport

import (
	"fmt"
	"net"
	"strings"

	"github.com/brian1917/illumioapi"
)

// userInputConvert takes an ip address in the format of eth0:192.168.20.21 and returns an illumio interface struct
func userInputConvert(ip string) (illumioapi.Interface, error) {
	var ifaceName, ifaceAddress string

	x := strings.Split(ip, ":")

	if len(x) == 1 {
		ifaceName = "umwl"
		ifaceAddress = x[0]
	} else if len(x) == 2 {
		ifaceName = x[0]
		ifaceAddress = x[1]
	} else {
		return illumioapi.Interface{}, fmt.Errorf("%s is not a valid format", ip)
	}

	// If the input includes a CIDR, make sure it's valid by parsing and then return it
	if strings.Contains(ifaceAddress, "/") {
		ipAddress, ipNet, err := net.ParseCIDR(ifaceAddress)
		if err != nil {
			return illumioapi.Interface{}, err
		}
		cidr, _ := ipNet.Mask.Size()
		return illumioapi.Interface{Address: ipAddress.String(), CidrBlock: &cidr, Name: ifaceName}, nil
	}

	// We only get here if it does not include CIDR notation.
	// Validate IP address by parsing it with /32
	ipAddress := net.ParseIP(ifaceAddress)
	if ipAddress == nil {
		return illumioapi.Interface{}, fmt.Errorf("%s is an invalid ip address", ip)
	}
	return illumioapi.Interface{Address: ipAddress.String(), CidrBlock: nil, Name: ifaceName}, nil
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
