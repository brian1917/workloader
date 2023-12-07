package awslabel

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var labelMapping, outputFileName, awsOptions string

func init() {
	AwsLabelCmd.Flags().StringVarP(&labelMapping, "mapping", "m", "", "mappings of AWS tags to illumio labels. the format is a comma-separated list of aws-tag:illumio-label. For example, \"application:app,type:role\" maps the AWS tag of application to the Illumio app label and the Azure type tag to the Illumio role label.")
	AwsLabelCmd.Flags().StringVarP(&awsOptions, "options", "o", "", "AWS CLI can be extended using this option.  Anything added after -o inside quotes will be passed as is(e.g \"--region us-west-1\"")
	AwsLabelCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	AwsLabelCmd.MarkFlagRequired("mapping")
	AwsLabelCmd.Flags().SortFlags = false
}

// TrafficCmd runs the workload identifier
var AwsLabelCmd = &cobra.Command{
	Use:   "aws-label",
	Short: "Import labels for AWS VMs.",
	Long: `
Import labels for AWS VMs.

The command relies on the AWS CLI being installed and authenticated. See here for installing the AWS CLI: https://aws.amazon.com/cli/.

To test the AWS CLI is authenticated, run aws ec2 describe-instances and ensure JSON output is displayed.

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

		AwsLabels(labelMapping, &pce, updatePCE, noPrompt)
	},
}

func AwsLabels(labelMapping string, pce *illumioapi.PCE, updatePCE, noPrompt bool) {

	// Create the lookup map where the illumio label is the key and the AWS key is the value
	illumioAwsMap := make(map[string]string)

	// Iterate through the user provider mappings
	x := strings.Replace(labelMapping, ", ", ",", -1)
	for _, lm := range strings.Split(x, ",") {
		s := strings.Split(lm, ":")
		if len(s) != 2 {
			utils.LogError(fmt.Sprintf("%s is an invalid mapping", lm))
		}
		illumioAwsMap[s[1]] = s[0]
	}

	// Set up the csv headers
	csvData := [][]string{{"instanceid", "hostname"}}
	for illumioLabel := range illumioAwsMap {
		csvData[0] = append(csvData[0], illumioLabel)
	}

	//Include AWS options if user enters any
	cmd := exec.Command("aws", "ec2", "describe-instances", "--no-cli-pager")
	if awsOptions != "" {
		cmd.Args = append(cmd.Args, strings.Split(awsOptions, " ")...)
	}

	utils.LogInfof(true, "running command: %s", cmd.String())
	outputBytes, _ := cmd.CombinedOutput()
	utils.LogDebug(fmt.Sprintf("stdout or stderror: %s", string(outputBytes)))
	utils.LogInfof(false, "exit code: %d", cmd.ProcessState.ExitCode())

	// Unmarshall the JSON
	var awsReservations AwsCLIResponse
	if err := json.Unmarshal(outputBytes, &awsReservations); err != nil {
		utils.LogErrorf("unmarshaling output - %s", err)
	}
	//json.Unmarshal([]byte(outbuf.String()), &awsReservations)

	var awsInstanceCount int
	// Iterate through the AWS VMs
	for _, reservation := range awsReservations.Reservations {
		for _, instance := range reservation.Instance {

			awsInstanceCount++
			//Create map for all instances tags(key/values)
			tagMap := make(map[string]string)
			for _, tag := range instance.Tags {
				tagMap[*tag.Key] = *tag.Value
			}
			// Start the new csv row
			csvRow := []string{*instance.InstanceId}
			for _, header := range csvData[0] {
				// Process instanceid
				if header == "instanceid" {
					continue
				}
				//process hostname by finding Name TAG
				if header == "hostname" {
					if tagMap["Name"] == "" {
						csvRow = append(csvRow, *instance.InstanceId)
					} else {
						csvRow = append(csvRow, tagMap["Name"])
					}
				} else {
					csvRow = append(csvRow, tagMap[illumioAwsMap[header]])
				}
			}
			csvData = append(csvData, csvRow)
		}

	}

	// Create the output file and call wkld-import
	if awsInstanceCount > 0 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-aws-label-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d aws vms with label data exported", len(csvData)-1), true)

		utils.LogInfo("passing output into wkld-import...", true)

		wkldimport.ImportWkldsFromCSV(wkldimport.Input{
			PCE:             *pce,
			ImportFile:      outputFileName,
			RemoveValue:     "aws-label-delete",
			Umwl:            false,
			UpdateWorkloads: true,
			UpdatePCE:       updatePCE,
			NoPrompt:        noPrompt,
			MaxUpdate:       -1,
			MaxCreate:       -1,
		})

	} else {
		utils.LogInfo("no aws vms found", true)
	}

}
