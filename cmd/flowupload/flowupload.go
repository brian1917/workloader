package flowupload

import (
	"fmt"
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
var debug bool

func init() {
	FlowUpload.Flags().StringVar(&csvFile, "in", "", "Input csv file. The first row (headers) will be skipped.")
	FlowUpload.MarkFlagRequired("in")

	FlowUpload.Flags().SortFlags = false

}

// FlowUpload runs the upload command
var FlowUpload = &cobra.Command{
	Use:   "flowupload",
	Short: "Upload flows from CSV file to the PCE.",
	Long: `
Upload flows from CSV file to the PCE
	
The CSV requires 4 columns WITHOUT a header row: src, dst, port, protocol. The protocol should be in numeric format (TCP=6 and UDP=17).

Example input:

+----------------+----------------+------+----+
| 192.168.200.21 | 192.168.200.22 | 8080 | 6  |
+----------------+----------------+------+----+
| 192.168.200.22 | 192.168.200.23 | 8080 | 17 |
+----------------+----------------+------+----+`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetPCE()
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for flowupload command - %s", err)
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		uploadFlows()
	},
}

func uploadFlows() {
	// Log start
	utils.Log(0, "started flowupload command")

	// Upload flows
	f, a, err := pce.UploadTraffic(csvFile)
	if debug {
		utils.LogAPIResp("UploadTraffic", a)
	}
	// Log error
	if err != nil {
		utils.Log(1, err.Error())
	}

	// Log response
	utils.Log(0, fmt.Sprintf("%d flows received", f.NumFlowsReceived))
	utils.Log(0, fmt.Sprintf("%d flows failed", f.NumFlowsFailed))
	fmt.Printf("%d flows received\r\n", f.NumFlowsReceived)
	fmt.Printf("%d flows failed\r\n", f.NumFlowsFailed)
	if f.NumFlowsFailed > 0 {
		var failedFlow []string
		for _, ff := range f.FailedFlows {
			failedFlow = append(failedFlow, *ff)
		}
		utils.Log(0, fmt.Sprintf("failed flows: %s", strings.Join(failedFlow, ",")))
		fmt.Printf("Failed flows: %s\r\n", strings.Join(failedFlow, ","))
	}
}
