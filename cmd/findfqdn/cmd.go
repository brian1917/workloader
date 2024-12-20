package findfqdn

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var lookupFile, outputFileName, umwlFile string
var anyIP bool

func init() {
	FindFQDNCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	FindFQDNCmd.Flags().BoolVar(&anyIP, "any-ip", false, "look up all ip addresses. default is just rfc1918.")
	FindFQDNCmd.Flags().StringVar(&umwlFile, "umwl-file", "", "create a new file in wkld-import format.")
}

// TrafficCmd runs the workload identifier
var FindFQDNCmd = &cobra.Command{
	Use:   "find-fqdn",
	Short: "Perform reverse name lookup on list of IPs.",
	Long: `
Perform reverse name lookup on list of IPs.

Use the export of the "Connection with Unknown IPs" traffic report as input.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting pce - %s", err.Error()))
		}
		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for file with IPs to perform Reverse lookup on. See usage help.")
			os.Exit(0)
		}

		lookupFile = args[0]

		// Get the workloads
		pce.Load(illumioapi.LoadInput{Workloads: true}, utils.UseMulti())

		updatePCE := viper.Get("update_pce").(bool)
		noPrompt := viper.Get("no_prompt").(bool)

		FindFQDN(&pce, updatePCE, noPrompt)
	},
}

// FindFQDN - Performs looking through CSV and taking all IPs in the IP Address field and performing reverse lookup on that IP.
// Results will place the names found back into the FQDN field in the CSV and export the file either to specified named file or generic time/date file
func FindFQDN(pce *illumioapi.PCE, updatePCE, noPrompt bool) {

	// Start the output CSV data
	umwlCSV := [][]string{}

	// Parse the input CSV
	csvData, err := utils.ParseCSV(lookupFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create the headers
	headers := make(map[string]*int)

	ipMap := map[string]bool{}
	// Iterate through the CSV
	for rowIndex, row := range csvData {

		// If it's the first row, process the headers
		if rowIndex == 0 {
			for i, l := range row {
				x := i
				headers[l] = &x
			}
			umwlCSV = append(umwlCSV, []string{"hostname", "ip"})
			continue
		}

		// Get the lookup IP address
		var lookupIP string
		if valIP, ok := headers[HeaderIPAddr]; ok {
			lookupIP = row[*valIP]
		} else {
			utils.LogWarning(fmt.Sprintf("the ip address field is left blank so no lookup was performed line %d", rowIndex), false)
			continue
		}

		// Do RFC 1918 check
		if !utils.IsRFC1918(lookupIP) && !anyIP {
			continue
		}

		fqdn, err := net.LookupAddr(lookupIP)
		if err != nil {
			utils.LogWarningf(false, "error performing reverse lookup for IP %s: %v", lookupIP, err)
		}

		// Change the fqdn entry in the CSV unless ip address was skipped (RFC1918)
		if val, ok := headers[HeaderFQDN]; ok && (row[*val] == "" && len(fqdn) != 0) {
			row[*val] = strings.Join(fqdn, ";")
			if ok := ipMap[lookupIP]; ok && fqdn[0] != "" {
				umwlCSV = append(umwlCSV, []string{fqdn[0], lookupIP})
			}
		} else if row[*val] != "" && fqdn != nil {
			umwlCSV = append(umwlCSV, []string{row[*val], lookupIP})
		}
		ipMap[lookupIP] = true

	}

	//Output the CSV now with any FQDNs found.
	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = utils.FileName("")
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d rows exported.", len(csvData)-1), true)

		//If you use the umwl-option it will cause a new file in the wkld-import format to be created."
		if len(umwlCSV) > 1 && umwlFile != "" {
			utils.WriteOutput(umwlCSV, umwlCSV, umwlFile)
			utils.LogInfo(fmt.Sprintf("%d rows in umwl file exported.", len(umwlCSV)-1), true)
		}
	}
}
