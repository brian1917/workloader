package subnet

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var csvFile, labelFile, outputFileName string
var inclUmwl, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

func init() {
	SubnetCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	SubnetCmd.Flags().BoolVar(&inclUmwl, "incl-umwl", false, "include unmanaged workloads.")
	SubnetCmd.Flags().StringVar(&labelFile, "label-file", "", "csv file with labels to filter query. the file should have 4 headers: role, app, env, and loc. The four columns in each row is an \"AND\" operation. Each row is an \"OR\" operation.")
	SubnetCmd.Flags().SortFlags = false

}

// SubnetCmd runs the workload identifier
var SubnetCmd = &cobra.Command{
	Use:   "subnet [csv file with subnet inputs]",
	Short: "Assign labels based on a workload's network.",
	Long: `
Assign labels based on a workload's network.
	
If a workload has more than one interface, the first interface with a default gateway that is matched to a network is used.

The input csv requires a "network" header as well as a header for each label key that should be updated. Order of columns does not matter. See below for an example input file. 

+----------------+------+-----+
|    network     | env  | loc |
+----------------+------+-----+
| 10.0.0.0/8     | prod | bos |
| 192.168.0.0/16 | dev  | nyc |
+----------------+------+-----+

If a workload matches multiple subnets, the first subnet on the input CSV is used.

If no label-file is used all workloads are processed. The first row of a label-file should be label keys. The workload query uses an AND operator for entries on the same row and an OR operator for the separate rows. An example label file is below:
+------+-----+-----+-----+----+
| role | app | env | loc | bu |
+------+-----+-----+-----+----+
| web  | erp |     |     |    |
|      |     |     | bos | it |
|      | crm |     |     |    |
+------+-----+-----+-----+----+
This example queries all idle workloads that are
- web (role) AND erp (app) 
- OR bos(loc) AND it (bu)
- OR CRM (app)

Recommended to run without --update-pce first to log of what will change in a csv file. To disable the prompt for updates, use --no-prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogErrorf("getting pce - %s", err)
		}

		// Get CSV file
		if len(args) != 1 {
			utils.LogErrorf("1 argument required for the csv file. see help menu for details.")
		}
		csvFile = args[0]

		// Get Viper configuration
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		subnetParser()
	},
}

type userProvidedNetwork struct {
	providedNetwork string
	labels          map[string]string
	network         net.IPNet
	csvLine         int
}

func subnetParser() {

	userNetworks := []userProvidedNetwork{}
	labelKeySlice := []string{}

	// Parse the input CSV
	inputData, err := utils.ParseCSV(csvFile)
	if err != nil {
		utils.LogErrorf("parsing input csv - %s", err)
	}
	labelColumns := make(map[string]int)
	networkIndex := 0
	networkMatch := false
	// Iterate through each row
	for rowIndex, rowData := range inputData {
		// Process headers
		if rowIndex == 0 {
			for colIndex, colData := range rowData {
				if colData == "network" {
					networkIndex = colIndex
					networkMatch = true
					continue
				}
				labelColumns[colData] = colIndex
				labelKeySlice = append(labelKeySlice, colData)
			}
			if !networkMatch {
				utils.LogError("input file must contain network header")
			}
			continue
		}

		// Process other rows
		labels := make(map[string]string)
		for colIndex, colData := range rowData {
			if colData == "network" {
				continue
			}
			labels[inputData[0][colIndex]] = rowData[colIndex]
		}
		network := userProvidedNetwork{providedNetwork: rowData[networkIndex], labels: labels, csvLine: rowIndex + 1}
		_, net, err := net.ParseCIDR(network.providedNetwork)
		if err != nil {
			utils.LogErrorf("csv line %d - %s is invalid cidr - %s", rowIndex+1, network.providedNetwork, err)
		}
		network.network = *net
		userNetworks = append(userNetworks, network)
	}

	// GetAllWorkloads
	qp := make(map[string]string)

	// Managed or unmanaged
	if !inclUmwl {
		qp["managed"] = "true"
	}
	// Process a label file if one is provided
	if labelFile != "" {
		labelCsvData, err := utils.ParseCSV(labelFile)
		if err != nil {
			utils.LogErrorf("parsing labelFile - %s", err)
		}

		labelQuery, err := pce.WorkloadQueryLabelParameter(labelCsvData)
		if err != nil {
			utils.LogErrorf("getting label parameter query - %s", err)
		}
		if len(labelQuery) > 10000 {
			utils.LogErrorf("the query is too large. the total character count is %d and the limit for this command is 10,000", len(labelQuery))
		}
		qp["labels"] = labelQuery
	}
	if len(qp) == 0 {
		qp = nil
	}
	api, err := pce.GetWklds(qp)
	utils.LogAPIRespV2("GetWklds", api)
	if err != nil {
		utils.LogErrorf("GetWklds - %s", err)
	}

	// Create a slice to store our results
	csvData := [][]string{{"hostname", "href", "ip", "network", "csv_line_from_input"}}
	csvData[0] = append(csvData[0], labelKeySlice...)

	// Iterate through workloads

workloads:
	for _, w := range pce.WorkloadsSlice {
		// Iterate through the interfaces
		for _, nic := range illumioapi.PtrToVal(w.Interfaces) {
			if len(illumioapi.PtrToVal(w.Interfaces)) > 1 && nic.DefaultGatewayAddress == "" {
				continue
			}
			// Check networks
			for _, userNetwork := range userNetworks {
				if userNetwork.network.Contains(net.ParseIP(nic.Address)) {
					csvRow := []string{illumioapi.PtrToVal(w.Hostname), w.Href, nic.Address, userNetwork.providedNetwork, strconv.Itoa(userNetwork.csvLine)}
					for _, col := range csvData[0] {
						if col == "hostname" || col == "href" || col == "ip" || col == "network" || col == "csv_line_from_input" {
							continue
						}
						csvRow = append(csvRow, userNetwork.labels[col])
					}
					csvData = append(csvData, csvRow)
					continue workloads
				}
			}
		}
	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-subnet-wkld-import-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		wkldImport := wkldimport.Input{
			PCE:                     pce,
			ImportFile:              outputFileName,
			RemoveValue:             "<subnet_remove_value>",
			Umwl:                    false,
			AllowEnforcementChanges: false,
			UpdateWorkloads:         true,
			UpdatePCE:               updatePCE,
			NoPrompt:                noPrompt,
			MaxUpdate:               -1,
			MaxCreate:               0,
		}
		wkldimport.ImportWkldsFromCSV(wkldImport)
	}

}
