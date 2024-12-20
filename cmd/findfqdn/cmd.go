package findfqdn

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var lookupFile, outputFileName, umwlFile string
var anyIP bool

func init() {
	FindFQDNCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	FindFQDNCmd.Flags().BoolVar(&anyIP, "anyip", false, "By default only perform lookup on RFC1918 address.  Select this option if you want to perform lookup on any ip")
	FindFQDNCmd.Flags().StringVar(&umwlFile, "umwl-file", "", "this option will create a new file in wkld-import format.")
}

// TrafficCmd runs the workload identifier
var FindFQDNCmd = &cobra.Command{
	Use:   "find-fqdn",
	Short: "Perform reverse name lookup on list of IPs.",
	Long: `
Take the Connection with Unknown IPs traffic export data and tries performs reverse lookup the IP to fill in the FQDN.

Use --output-file to output to specific file otherwise output will be workloader-findfile-<date and time>.csv
`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		// pce, err := utils.GetTargetPCEV2(false)
		// if err != nil {
		// 	utils.LogError(fmt.Sprintf("error getting pce - %s", err.Error()))
		// }
		// Set the CSV file
		utils.LogInfo("Reading CSV and performing reverse lookup.", true)

		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for file with IPs to perform Reverse lookup on. See usage help.")
			os.Exit(0)
		}

		lookupFile = args[0]

		// Get the workloads
		//pce.Load(illumioapi.LoadInput{Workloads: true}, utils.UseMulti())

		updatePCE := viper.Get("update_pce").(bool)
		noPrompt := viper.Get("no_prompt").(bool)

		FindFQDN( /*&pce,*/ updatePCE, noPrompt)
	},
}

// RFC1918 - Check if IP is in the range
func RFC1918(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Define the private IP ranges
	privateIPBlocks := []*net.IPNet{
		{
			IP:   net.IPv4(10, 0, 0, 0),
			Mask: net.CIDRMask(8, 32),
		},
		{
			IP:   net.IPv4(172, 16, 0, 0),
			Mask: net.CIDRMask(12, 32),
		},
		{
			IP:   net.IPv4(192, 168, 0, 0),
			Mask: net.CIDRMask(16, 32),
		},
	}

	// Check if the IP is within any of the private IP ranges
	for _, privateIPBlock := range privateIPBlocks {
		if privateIPBlock.Contains(ip) {
			return true
		}
	}

	return false

}

// lookupIP - performs reverse lookup on the IP address provided using local machines DNS server
func lookupIP(ip string) []string {

	if (!anyIP && RFC1918(ip)) || anyIP {

		// Perform a reverse lookup for the IP address
		names, err := net.LookupAddr(ip)
		if err != nil {
			utils.LogWarning(fmt.Sprintf("Perform reverse lookup for IP %s: %v", ip, err), false)
			return []string{"Not Found"}
		}

		return names
	}
	return nil
}

// FindFQDN - Performs looking through CSV and taking all IPs in the IP Address field and performing reverse lookup on that IP.
// Results will place the names found back into the FQDN field in the CSV and export the file either to specified named file or generic time/date file
func FindFQDN( /*pce *illumioapi.PCE,*/ updatePCE, noPrompt bool) {

	// Parse the CSV
	csvData, err := utils.ParseCSV(lookupFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create the headers
	headers := make(map[string]*int)

	umwlCSV := [][]string{}
	//make sure there we dont perform lookup on the same IP again.
	ipMap := map[string]bool{}
	// Iterate through the CSV
	for i, line := range csvData {
		//csvLine := i + 1

		// If it's the first row, process the headers
		if i == 0 {
			for i, l := range line {
				x := i
				headers[l] = &x
			}
			umwlCSV = append(umwlCSV, []string{"hostname", "ip"})
			continue
		}

		//Lookup IP make sure to check if there is a IP address header.  Also save IP to add to UMWL file.
		valIP, ok := headers[HeaderIPAddr]
		if !ok {
			utils.LogWarning(fmt.Sprintf("There is no IP Address column so no lookup was performed line %d", i), false)
			break
		}

		if ok := ipMap[line[*valIP]]; ok {
			utils.LogWarning(fmt.Sprintf("Duplicate IP in the list.  Skipping Reverse lookup for this ip %s", line[*valIP]), false)
			continue
		}

		fqdn := lookupIP(line[*valIP])
		//change the FQDN entry in the CSV unless ip address was skipped (RFC1918)
		if val, ok := headers[HeaderFQDN]; ok && (line[*val] == "" && fqdn != nil) {
			line[*val] = strings.Join(fqdn, ",")
			if fqdn[0] != "Not Found" {
				tmpRow := []string{fqdn[0], line[*valIP]}
				umwlCSV = append(umwlCSV, tmpRow)
			}
		} else if line[*val] != "" && fqdn != nil {
			umwlCSV = append(umwlCSV, []string{line[*val], line[*valIP]})
		}
		ipMap[line[*valIP]] = true
	}

	//Output the CSV now with any FQDNs found.
	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-findfqdn-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d rows exported.", len(csvData)-1), true)

		//If you use the umwl-option it will cause a new file in the wkld-import format to be created."
		if len(umwlCSV) > 1 && umwlFile != "" {
			utils.WriteOutput(umwlCSV, umwlCSV, umwlFile)
			utils.LogInfo(fmt.Sprintf("%d rows in umwl file exported.", len(umwlCSV)-1), true)
		}
	}
}
