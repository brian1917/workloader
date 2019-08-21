package importer

import (
	"fmt"
	"net"
	"strings"

	"github.com/brian1917/illumioapi"
)

// validateIP takes a string and will return a valid CIDR notation IP address if the provided string is valid.
// if a single IP address is provided, it will return a /32. (e.g., 192.168.200.24 will return 192.168.200.24/32)
func validateIP(ip string) (string, error) {
	// If the input includes a CIDR, make sure it's valid by parsing and then return it
	if strings.Contains(ip, "/") {
		ipAddress, ipNet, err := net.ParseCIDR(ip)
		if err != nil {
			return "", err
		}
		cidr, _ := ipNet.Mask.Size()
		return fmt.Sprintf("%s/%d", ipAddress.String(), cidr), nil
	}

	// We only get here if it does not include CIDR notation.
	// Validate IP address by parsing it with /32
	ipAddress, ipNet, err := net.ParseCIDR(ip + "/32")
	if err != nil {
		return "", err
	}
	cidr, _ := ipNet.Mask.Size()
	return fmt.Sprintf("%s/%d", ipAddress.String(), cidr), nil
}

// userInputConvert takes an ip address in the format of eth0:192.168.20.21 and returns an illumio interface struct
func userInputConvert(ip string) (illumioapi.Interface, error) {
	x := strings.Split(ip, ":")
	if len(x) != 2 {
		return illumioapi.Interface{}, fmt.Errorf("%s is not a valid format (e.g., eth0:192.168.4.2)", ip)
	}

	// If the input includes a CIDR, make sure it's valid by parsing and then return it
	if strings.Contains(x[1], "/") {
		ipAddress, ipNet, err := net.ParseCIDR(x[1])
		if err != nil {
			return illumioapi.Interface{}, err
		}
		cidr, _ := ipNet.Mask.Size()
		return illumioapi.Interface{Address: ipAddress.String(), CidrBlock: &cidr, Name: x[0]}, nil
	}

	// We only get here if it does not include CIDR notation.
	// Validate IP address by parsing it with /32
	ipAddress := net.ParseIP(x[1])
	if ipAddress == nil {
		return illumioapi.Interface{}, fmt.Errorf("%s is invalid ip address", ip)
	}
	return illumioapi.Interface{Address: ipAddress.String(), CidrBlock: nil, Name: x[0]}, nil

}
