package utils

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/brian1917/illumioapi"
)

// GetServicePortsPCE returns PortProto list and PortRangeProto for use in a traffic query from a service object in the PCE
func GetServicePortsPCE(pce illumioapi.PCE, serviceName string) ([][2]int, [][3]int) {

	// Create our return
	portProtoExcl := [][2]int{}
	portRangeProtoExcl := [][3]int{}

	// Get all services
	svcs, _, err := pce.GetAllServices("draft")
	if err != nil {
		LogError(err.Error())
	}

	// Find our service of interest
	for _, s := range svcs {
		if s.Name == serviceName {
			for _, sp := range s.ServicePorts {
				if sp.ToPort != 0 {
					portRangeProtoExcl = append(portRangeProtoExcl, [3]int{sp.Port, sp.ToPort, sp.Protocol})
				} else {
					portProtoExcl = append(portProtoExcl, [2]int{sp.Port, sp.Protocol})
				}
			}
			return portProtoExcl, portRangeProtoExcl
		}
	}
	return nil, nil
}

// GetServicePortsCSV returns port proto list from a CSV
func GetServicePortsCSV(filename string) [][2]int {
	// Open CSV File
	csvFile, _ := os.Open(filename)
	reader := csv.NewReader(bufio.NewReader(csvFile))

	exclPorts := [][2]int{}

	n := 0
	for {
		n++
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			LogError(fmt.Sprintf("Reading CSV File for port/protocol list - %s", err))
		}

		port, err := strconv.Atoi(line[0])
		if err != nil {
			LogError(fmt.Sprintf("Non-integer port value on line %d - %s", n, err))
		}
		protocol, err := strconv.Atoi(line[1])
		if err != nil {
			LogError(fmt.Sprintf("Non-integer protocol value on line %d - %s", n, err))
		}

		exclPorts = append(exclPorts, [2]int{port, protocol})
	}

	return exclPorts
}
