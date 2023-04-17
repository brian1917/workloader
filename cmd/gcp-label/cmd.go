package gcplabel

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var labelMapping, outputFileName, gcpOptions string

func init() {
	GcpLabelCmd.Flags().StringVarP(&labelMapping, "mapping", "m", "", "mappings of GCP labels to illumio labels. the format is a comma-separated list of gcp-label:illumio-label. For example, \"application:app,type:role\" maps the GCP labels of application to the Illumio app label and the GCP type label to the Illumio role label.")
	GcpLabelCmd.Flags().StringVarP(&gcpOptions, "options", "o", "", "GCP CLI can be extended using this option.  Anything added after -o inside quotes will be passed as is(e.g \"--filter status=RUNNNING\"")
	GcpLabelCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	GcpLabelCmd.MarkFlagRequired("mapping")
	GcpLabelCmd.Flags().SortFlags = false
}

// TrafficCmd runs the workload identifier
var GcpLabelCmd = &cobra.Command{
	Use:   "gcp-label",
	Short: "Import labels for GCP VMs.",
	Long: `
Import labels for AWS VMs.

The command relies on the GCP CLI(gcloud) being installed and authenticated. See here for installing the GCP CLI: https://cloud.google.com/sdk/docs/install-sdk.

To test the GCP CLI is authenticated, run gcloud compute instances list --output=json and ensure JSON output is displayed.

A file will be produced that is passed into the wkld-import command. 

It is recommend to run without --update-pce first to the csv produced and what impacts of the wkld-import command.
`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting pce - %s", err.Error()))
		}

		updatePCE := viper.Get("update_pce").(bool)
		noPrompt := viper.Get("no_prompt").(bool)

		GCPLabels(labelMapping, &pce, updatePCE, noPrompt)
	},
}

func GCPLabels(labelMapping string, pce *illumioapi.PCE, updatePCE, noPrompt bool) {

	// Create the lookup map where the illumio label is the key and the AWS key is the value
	illumioGcpMap := make(map[string]string)

	// Iterate through the user provider mappings
	x := strings.Replace(labelMapping, ", ", ",", -1)
	for _, lm := range strings.Split(x, ",") {
		s := strings.Split(lm, ":")
		if len(s) != 2 {
			utils.LogError(fmt.Sprintf("%s is an invalid mapping", lm))
		}
		illumioGcpMap[s[1]] = s[0]
	}

	// Set up the csv headers
	csvData := [][]string{{"instanceid", "hostname"}}
	for illumioLabel := range illumioGcpMap {
		csvData[0] = append(csvData[0], illumioLabel)
	}

	//Include GCP options if user enters any
	cmd := exec.Command("gcloud", "compute", "instances", "list", "--format=json")
	if gcpOptions != "" {
		cmd.Args = append(cmd.Args, strings.Split(gcpOptions, " ")...)
	}

	// Build the VM list command with a pipe
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		utils.LogError(fmt.Sprintf("pipe error - %s", err.Error()))
	}

	// Run the command
	if err := cmd.Start(); err != nil {
		utils.LogError(fmt.Sprintf("run error - %s", err.Error()))
	}

	// Read the stout
	bytes, err := io.ReadAll(pipe)
	if err != nil {
		log.Fatal(err)
	}

	// Unmarshall the JSON
	var gcpInstances []GcpCLIResponse
	json.Unmarshal(bytes, &gcpInstances)

	var gcpInstanceCount int
	// Iterate through the AWS VMs
	for _, instance := range gcpInstances {

		gcpInstanceCount++
		//Create map for all instances tags(key/values)
		tagMap := make(map[string]string)
		for key, value := range instance.Labels {
			tagMap[key] = value
		}
		// Start the new csv row
		csvRow := []string{instance.Id}
		for _, header := range csvData[0] {
			// Process instanceid
			if header == "instanceid" {
				continue
			}
			//process hostname by finding Name TAG
			if header == "hostname" {
				if tagMap["Name"] == "" {
					csvRow = append(csvRow, instance.Name)
				} else {
					csvRow = append(csvRow, tagMap["Name"])
				}
			} else {
				csvRow = append(csvRow, tagMap[illumioGcpMap[header]])
			}
		}
		csvData = append(csvData, csvRow)
	}

	// Create the output file and call wkld-import
	if gcpInstanceCount > 0 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-gcp-label-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d gcp vms with label data exported", len(csvData)-1), true)

		utils.LogInfo("passing output into wkld-import...", true)

		wkldimport.ImportWkldsFromCSV(wkldimport.Input{
			PCE:             *pce,
			ImportFile:      outputFileName,
			RemoveValue:     "gcp-label-delete",
			Umwl:            false,
			UpdateWorkloads: true,
			UpdatePCE:       updatePCE,
			NoPrompt:        noPrompt,
		})

	} else {
		utils.LogInfo("no GCP vms found", true)
	}

	utils.LogEndCommand("gcp-label")

}
