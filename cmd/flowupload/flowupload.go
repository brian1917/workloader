package flowupload

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Global variables
var pce illumioapi.PCE
var err error
var csvFile string
var debug, noHeader bool

func init() {
	FlowUpload.Flags().BoolVarP(&noHeader, "no-header", "n", false, "Indicates there is no header line in input CSV file.")
}

// FlowUpload runs the upload command
var FlowUpload = &cobra.Command{
	Use:   "flowupload [csv file with flows]",
	Short: "Upload flows from CSV file to the PCE.",
	Long: `
Upload flows from CSV file to the PCE
	
The CSV requires 4 columns: src_ip, dst_ip, port, protocol. The protocol should be in numeric format (TCP=6 and UDP=17).

The default assumes there is a header line and will skip the first entry. If there no header, set the --no-header (-n) flag.

There is no limit for maximum flows in the CSV. API calls to PCE will be sent in 1,000 entry chunks.

Example input:
+----------------+-----------------+-------+-----------+
|     src_ip     |      dst_ip     |  port |  protocol |
+----------------+-----------------+-------+-----------+
| 192.168.200.21 |  192.168.200.22 |  8080 |         6 |
| 192.168.200.22 |  192.168.200.23 |  8080 |        17 |
+----------------+-----------------+-------+-----------+`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetPCE(false)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for flowupload command - %s", err)
		}

		// Get csv file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		uploadFlows()
	},
}

func uploadFlows() {
	// Log start
	utils.Log(0, "started flowupload command")

	// Upload flows
	f, err := pce.UploadTraffic(csvFile, !noHeader)
	for _, a := range f.APIResps {
		utils.LogAPIResp("UploadTraffic", a)
	}

	// Log error
	if err != nil {
		utils.Log(1, err.Error())
	}

	// Log response
	utils.Log(0, fmt.Sprintf("%d flows in CSV file.", f.TotalFlowsInCSV))
	i := 1
	for _, flowResp := range f.FlowResps {
		fmt.Printf("API Call %d of %d...\r\n", i, len(f.APIResps))
		utils.Log(0, fmt.Sprintf("%d flows received", flowResp.NumFlowsReceived))
		utils.Log(0, fmt.Sprintf("%d flows failed", flowResp.NumFlowsFailed))
		fmt.Printf("%d flows received\r\n", flowResp.NumFlowsReceived)
		fmt.Printf("%d flows failed\r\n", flowResp.NumFlowsFailed)
		if i < len(f.APIResps) {
			fmt.Println("-------------------------")
		}

		if flowResp.NumFlowsFailed > 0 {
			var failedFlow []string
			for _, ff := range flowResp.FailedFlows {
				failedFlow = append(failedFlow, *ff)
			}
			utils.Log(0, fmt.Sprintf("failed flows: %s", strings.Join(failedFlow, ",")))
			fmt.Printf("Failed flows: %s\r\n", strings.Join(failedFlow, ","))
		}
		i++
	}

}
