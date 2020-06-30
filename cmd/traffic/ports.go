package traffic

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi"
)

// Contains checks if an integer is in a slice
func containsInt(intSlice []int, searchInt int) bool {
	for _, value := range intSlice {
		if value == searchInt {
			return true
		}
	}
	return false
}

func findPorts(traffic []illumioapi.TrafficAnalysis, coreServices []coreService, provider bool) ([]result, []result) {
	// Create a slice to hold the matches and non-matches
	var matches []result
	var nonmatches []result

	var ft []illumioapi.TrafficAnalysis

	// Create the filter traffic slice by removing traffic that is talking to each other
	for _, entry := range traffic {
		if entry.Src.IP != entry.Dst.IP {
			ft = append(ft, entry)
		}
	}

	// Get the traffic flow count for each machine on a port
	ipPortCount := make(map[string]int)

	for _, entry := range traffic {
		ip := entry.Dst.IP
		if !provider {
			ip = entry.Src.IP
		}
		ipPortCount[ip+"-"+strconv.Itoa(entry.ExpSrv.Port)] = ipPortCount[ip+"-"+strconv.Itoa(entry.ExpSrv.Port)] + entry.NumConnections

	}

	// Create a map for looking up FQDNs later
	fqdnMap := make(map[string]string)
	for _, entry := range traffic {
		if entry.Src.FQDN != "" {
			fqdnMap[entry.Src.IP] = entry.Src.FQDN
		}
		if entry.Dst.FQDN != "" {
			fqdnMap[entry.Dst.IP] = entry.Dst.FQDN
		}
	}

	// For each traffic flow not going to a workload, see if it already exists in the ipAddrPorts map. If no, add it.
	ipPorts := make(map[string][]int)
	for _, flow := range ft {
		// Set the IP as destination or source
		ip := flow.Dst.IP
		if !provider {
			ip = flow.Src.IP
		}
		if ports, ok := ipPorts[ip]; ok {
			if !containsInt(ports, flow.ExpSrv.Port) {
				ipPorts[ip] = append(ports, flow.ExpSrv.Port)
			}
		} else {
			ipPorts[ip] = []int{flow.ExpSrv.Port}
		}
	}

	// Iterate through each machine seen in explorer
	for ipAddr, ports := range ipPorts {
		// Reset the flow counter
		flowCounter := 0
		// Cycle through core services to look for matches
		for _, cs := range coreServices {
			// Reset the portMatches slice
			portMatches := []string{}
			// Only run when the the provider flag is the same for core service and passed into function
			if provider == cs.provider {
				// Required Ports
				reqPortMatches := 0
				for _, csReqPort := range cs.requiredPorts {
					if containsInt(ports, csReqPort) {
						reqPortMatches++
						flowCounter = flowCounter + ipPortCount[ipAddr+"-"+strconv.Itoa(csReqPort)]
						portMatches = append(portMatches, strconv.Itoa(csReqPort))
					}
				}
				// Optional Ports
				optPortMatches := 0
				for _, csOptPort := range cs.optionalPorts {
					if containsInt(ports, csOptPort) {
						optPortMatches++
						flowCounter = flowCounter + ipPortCount[ipAddr+"-"+strconv.Itoa(csOptPort)]
						portMatches = append(portMatches, strconv.Itoa(csOptPort))
					}
				}
				// Optional Port Ranges
				optPortRangeMatches := 0
				for _, csOptPortRange := range cs.optionalPortRanges {
					for _, port := range ports {
						if csOptPortRange[0] <= port && csOptPortRange[1] >= port {
							optPortRangeMatches++
							portMatches = append(portMatches, fmt.Sprintf("%s-%s", strconv.Itoa(csOptPortRange[0]), strconv.Itoa(csOptPortRange[1])))
							break // Only want to count one match in each range (e.g., range: 40000-50000 and ports 40001 and 40002 are used, we only want to count that as one match.)
						}
					}
				}
				// Check if it should count
				if (len(cs.requiredPorts) == reqPortMatches && len(cs.requiredPorts) > 0 && cs.numOptionalPorts <= (optPortMatches+optPortRangeMatches) && cs.numFlows <= flowCounter) ||
					(len(cs.requiredPorts) == 0 && cs.numOptionalPorts <= (optPortMatches+optPortRangeMatches) && cs.numFlows <= flowCounter) {

					t := "provider"
					if !provider {
						t = "consumer"
					}
					s := "port"
					if len(portMatches) > 1 {
						s = "ports"
					}
					reason := fmt.Sprintf("%s is the %s on traffic over %s %s. Required and optional non-ranges flow count is %d. ", ipAddr, t, s, strings.Join(portMatches, " "), flowCounter)

					matches = append(matches, result{csname: cs.name, ipAddress: ipAddr, fqdn: fqdnMap[ipAddr], app: cs.app, env: cs.env, loc: cs.loc, role: cs.role, reason: reason})
				} else if provider {
					// Convert slice of int to slice of string
					var portStr []string
					for _, p := range ports {
						if p == 0 {
							portStr = append(portStr, "ICMP")
						} else {
							portStr = append(portStr, strconv.Itoa(p))
						}
					}
					reason := fmt.Sprintf("Traffic observed on ports %s", strings.Join(portStr, ";"))
					nonmatches = append(nonmatches, result{ipAddress: ipAddr, reason: reason, matchStatus: 2})
				}
			}
		}
	}

	return matches, nonmatches
}
