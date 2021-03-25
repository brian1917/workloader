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
func GetServicePortsCSV(filename string) ([][2]int, error) {
	// Open CSV File
	csvFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer csvFile.Close()

	reader := csv.NewReader(ClearBOM(bufio.NewReader(csvFile)))

	svcList := [][2]int{}

	// Start the CSV line counter
	n := 0

	// Iterate over lines
	for {

		// Increase the counter
		n++

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading CSV File for port/protocol list - %s", err)
		}

		// Check the first line and skip if it's not integers
		if n == 1 {
			_, err = strconv.Atoi(line[0])
			if err != nil {
				continue
			}
			_, err = strconv.Atoi(line[1])
			if err != nil {
				continue
			}
		}

		// Convert the port
		port, err := strconv.Atoi(line[0])
		if err != nil {
			return nil, fmt.Errorf("non-integer port value on line %d - %s", n, err)
		}

		// Convert the protocol
		protocol, err := strconv.Atoi(line[1])
		if err != nil {
			return nil, fmt.Errorf("non-integer protocol value on line %d - %s", n, err)
		}

		// Append to the list
		svcList = append(svcList, [2]int{port, protocol})
	}

	return svcList, nil
}
