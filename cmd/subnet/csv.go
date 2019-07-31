package subnet

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
)

// A subnet is extracted from the CSV and has an assoicated location and environment
type subnet struct {
	network net.IPNet
	loc     string
	env     string
}

// used to parse subnet to environment and location labels
func locParser(csvFile string, netCol, envCol, locCol int) []subnet {
	var results []subnet

	// Open CSV File
	file, err := os.Open(csvFile)
	if err != nil {
		utils.Logger.Fatalf("Error opening CSV - %s", err)
	}
	defer file.Close()
	reader := csv.NewReader(bufio.NewReader(file))

	// Start the counter
	i := 0

	// Iterate through CSV entries
	for {

		// Increment the counter
		i++

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			utils.Logger.Fatalf("Error - reading CSV file - %s", err)
		}

		// Skipe the header row
		if i == 1 {
			continue
		}

		//make sure location label not empty
		if line[locCol] == "" {
			utils.Logger.Fatal("Error - Label field cannot be empty")
		}

		//Place subnet into net.IPNet data structure as part of subnetLabel struct
		_, network, err := net.ParseCIDR(line[netCol])
		if err != nil {
			utils.Logger.Fatal("Error - The Subnet field cannot be parsed.  The format is 10.10.10.0/24")
		}

		//Set struct values
		results = append(results, subnet{network: *network, env: line[envCol], loc: line[locCol]})
	}
	return results
}

func csvWriter(pce illumioapi.PCE, matches []match) {
	// Get all the labels again so we have a map

	labelMap, _, err := pce.GetLabelMapH()
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting label map - %s", err))
	}

	// Get time stamp for output files
	timestamp := time.Now().Format("20060102_150405")

	// Always create the default file
	outputFile, err := os.Create("subnet-output-" + timestamp + ".csv")
	if err != nil {
		utils.Logger.Fatalf("ERROR - Creating file - %s\n", err)
	}
	defer outputFile.Close()

	fmt.Fprintf(outputFile, "hostname,ip_address,original_loc,original_env,new_loc,new_env\r\n")

	for _, m := range matches {
		fmt.Fprintf(outputFile, "%s,%s,%s,%s,%s,%s\r\n", m.workload.Hostname, m.workload.Interfaces[0].Address, m.oldLoc, m.oldEnv, m.workload.GetLoc(labelMap).Value, m.workload.GetEnv(labelMap).Value)
	}

}
