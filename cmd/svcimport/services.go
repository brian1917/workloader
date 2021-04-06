package svcimport

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/cmd/svcexport"
	"github.com/brian1917/workloader/utils"
)

func processServices(input Input, data []string, csvLine int) (winSvc illumioapi.WindowsService, svcPort illumioapi.ServicePort) {
	var err error

	// If the port column is there and not blank, process it.
	if col, ok := input.Headers[svcexport.HeaderPort]; ok && data[col] != "" {
		// The port is the first entry after splitting on the "-" and removing spaces
		winSvc.Port, err = strconv.Atoi(strings.Split(strings.Replace(data[col], " ", "", -1), "-")[0])
		if err != nil {
			utils.LogError(fmt.Sprintf("CSV line %d - invalid %s", csvLine, svcexport.HeaderPort))
		}
		// Make the service port the same as the WinSvc
		svcPort.Port = winSvc.Port

		// The to port is the second entry after splitting on the "-" and removing spaces
		if strings.Contains(data[input.Headers[svcexport.HeaderPort]], "-") {
			winSvc.ToPort, err = strconv.Atoi(strings.Split(strings.Replace(data[col], " ", "", -1), "-")[1])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - invalid %s", csvLine, svcexport.HeaderPort))
			}
			// Make the service port the same as the WinSvc
			svcPort.ToPort = winSvc.ToPort
		}
	}

	// Process the protocol column
	if col, ok := input.Headers[svcexport.HeaderProto]; !ok && winSvc.Port != 0 {
		utils.LogError(fmt.Sprintf("CSV line %d - protocol is required when port is provided", csvLine))
	} else if ok && data[col] != "" {
		proto := 0
		if strings.ToLower(data[col]) == "tcp" {
			proto = 6
		} else if strings.ToLower(data[col]) == "udp" {
			proto = 17
		} else {
			proto, err = strconv.Atoi(data[col])
			if err != nil {
				utils.LogError(fmt.Sprintf("CSV line %d - invalid %s", csvLine, svcexport.HeaderProto))
			}
		}
		winSvc.Protocol = proto
		svcPort.Protocol = proto
	}

	// Process the ICMP Code column
	if col, ok := input.Headers[svcexport.HeaderICMPCode]; ok && data[col] != "" {
		winSvc.IcmpCode, err = strconv.Atoi(data[col])
		if err != nil {
			utils.LogError(fmt.Sprintf("CSV line %d - invalid ICMP code", csvLine))
		}
		svcPort.IcmpCode = winSvc.IcmpCode
	}

	// Process the ICMP Type column
	if col, ok := input.Headers[svcexport.HeaderICMPType]; ok && data[col] != "" {
		winSvc.IcmpType, err = strconv.Atoi(data[col])
		if err != nil {
			utils.LogError(fmt.Sprintf("CSV line %d - invalid ICMP type", csvLine))
		}
		svcPort.IcmpType = winSvc.IcmpType
	}

	// Process the Process Name
	if col, ok := input.Headers[svcexport.HeaderProcess]; ok {
		winSvc.ProcessName = data[col]
	}

	// Process the service name
	if col, ok := input.Headers[svcexport.HeaderService]; ok {
		winSvc.ServiceName = data[col]
	}

	return winSvc, svcPort

}
