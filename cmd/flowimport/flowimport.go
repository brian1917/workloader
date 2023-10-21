package flowimport

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Global variables
var pce illumioapi.PCE
var err error
var csvFile string
var noHeader bool

// FlowImportCmd runs the upload command
var FlowImportCmd = &cobra.Command{
	Use:   "flow-import [csv file with flows]",
	Short: "Upload flows from CSV file to the PCE.",
	Long: `
Upload flows from CSV file to the PCE.
	
The input CSV requires 4 columns: source, destination, port, and protocol.
Headers must be included, but values do not matter.
The CSV can have more than 4 columns, but first four must be as shown in example.

The source and destination can be an IP address or a hostname. If it's a hostname, the first interface on the workload will be used.
The protocol can be either any IANA protcol numeric value, tcp, or udp.

An intermediate CSV will be created and saved that translates hostnames to IP addresses, tcp to 6, and udp to 17.

There is no limit for maximum flows in the CSV. API calls to PCE will be sent in 1,000 entry chunks.

Example input:
+----------------+-----------------+-------+--------+
|      src       |       dst       |  port |  proto |
+----------------+-----------------+-------+--------+
| 192.168.200.21 |  asset-mgt-web1 |   443 |  tcp   |
| asset-mgt-web1 |  asset-mgt-db1  |  3306 |  tcp   |
| asset-mgt-web2 |  ntp-1          |   123 |  17    |
+----------------+-----------------+-------+--------+

The update-pce and --no-prompt flags are ignored for this command.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(false)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for flowupload command - %s", err)
		}

		// Get csv file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		uploadFlows()
	},
}

func uploadFlows() {

	// Get all workloads in a map by hostname
	_, a, err := pce.GetWklds(nil)
	utils.LogAPIResp("GetWkldHostMap", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Set the header for the new csv file
	newCSVData := [][]string{{"src", "dst", "port", "protocol"}}

	// Open CSV File
	file, err := os.Open(csvFile)
	if err != nil {
		utils.LogError(err.Error())
	}
	defer file.Close()
	reader := csv.NewReader(utils.ClearBOM(bufio.NewReader(file)))

	// Iterate through CSV entries
	i := 0
	for {

		// Increment the counter
		i++

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.LogError(err.Error())
		}

		// Skip the header row if needed
		if i == 1 && !noHeader {
			continue
		}

		// Process Source
		src := line[0]
		if net.ParseIP(line[0]) == nil {
			if _, ok := pce.Workloads[line[0]]; !ok {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid IP or valid hostname", i, line[0]))
			}

			sWkld := pce.Workloads[line[0]]
			if sWkld.GetIPWithDefaultGW() == "NA" {
				src = pce.Workloads[line[0]].Interfaces[0].Address
			} else {
				src = sWkld.GetIPWithDefaultGW()
			}

			if net.ParseIP(src) == nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s does not have a valid IP address on the first interface", i, line[0]))
			}
		}

		// Process Destination
		dst := line[1]
		if net.ParseIP(line[1]) == nil {
			if _, ok := pce.Workloads[line[1]]; !ok {
				utils.LogError(fmt.Sprintf("CSV line %d - %s is not valid IP or valid hostname", i, line[1]))
			}
			dWkld := pce.Workloads[line[1]]
			if dWkld.GetIPWithDefaultGW() == "NA" {
				dst = pce.Workloads[line[1]].Interfaces[0].Address
			} else {
				dst = dWkld.GetIPWithDefaultGW()
			}
			if net.ParseIP(dst) == nil {
				utils.LogError(fmt.Sprintf("CSV line %d - %s does not have a valid IP address on the first interface", i, line[1]))
			}
		}

		// Process protocols
		proto := strings.ToLower(line[3])
		if proto == "tcp" {
			proto = "6"
		}
		if proto == "udp" {
			proto = "17"
		}

		// Add to CSV array
		newCSVData = append(newCSVData, []string{src, dst, line[2], proto})
	}

	// Write the new CSV File
	newCSVFileName := "workloader-processed-flow-import-input-" + time.Now().Format("20060102_150405") + ".csv"

	// Create CSV
	outFile, err := os.Create(newCSVFileName)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Write CSV data
	writer := csv.NewWriter(outFile)
	writer.WriteAll(newCSVData)
	if err := writer.Error(); err != nil {
		utils.LogError(err.Error())
	}

	// Upload flows
	f, err := pce.UploadTraffic(newCSVFileName, !noHeader)
	for _, a := range f.APIResps {
		utils.LogAPIResp("UploadTraffic", a)
	}

	// Log error
	if err != nil {
		utils.LogError(err.Error())
	}

	// Log response
	utils.LogInfo(fmt.Sprintf("%d flows in CSV file.", f.TotalFlowsInCSV), false)
	i = 1
	for _, flowResp := range f.FlowResps {
		fmt.Printf("API Call %d of %d...\r\n", i, len(f.APIResps))
		utils.LogInfo(fmt.Sprintf("%d flows received", flowResp.NumFlowsReceived), true)
		utils.LogInfo(fmt.Sprintf("%d flows failed", flowResp.NumFlowsFailed), true)
		if i < len(f.APIResps) {
			fmt.Println("-------------------------")
		}

		if flowResp.NumFlowsFailed > 0 {
			var failedFlow []string
			for _, ff := range flowResp.FailedFlows {
				failedFlow = append(failedFlow, *ff)
			}
			utils.LogInfo(fmt.Sprintf("failed flows: %s", strings.Join(failedFlow, ",")), true)
		}
		i++
	}

}
