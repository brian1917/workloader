package traffic

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var csvFile, inclHrefDstFile, exclHrefDstFile, inclHrefSrcFile, exclHrefSrcFile, inclServiceCSV, exclServiceCSV, start, end, outputFileName string
var lookupTO, maxResults int
var privOnly, exclAllowed, exclPotentiallyBlocked, exclBlocked, exclWLs bool
var pce illumioapi.PCE
var err error

func init() {
	TrafficCmd.Flags().BoolVarP(&privOnly, "excl-public-ips", "p", false, "exclude public IP addresses and limit suggested workloads to the RFC 1918 address space")
	TrafficCmd.Flags().BoolVarP(&exclWLs, "excl-wklds", "w", false, "exclude IP addresses already assigned to workloads to suggest or verify labels")
	TrafficCmd.Flags().StringVarP(&inclHrefDstFile, "incl-dst-file", "a", "", "file with hrefs on separate lines to be used in as a provider include. Each line is treated as OR logic. On same line, combine hrefs of same object type for an AND logic. Headers optional")
	TrafficCmd.Flags().StringVarP(&exclHrefDstFile, "excl-dst-file", "b", "", "file with hrefs on separate lines to be used in as a provider exclude. Can be a csv with hrefs in first column. Headers optional")
	TrafficCmd.Flags().StringVarP(&inclHrefSrcFile, "incl-src-file", "c", "", "file with hrefs on separate lines to be used in as a consumer include. Each line is treated as OR logic. On same line, combine hrefs of same object type for an AND logic. Headers optional")
	TrafficCmd.Flags().StringVarP(&exclHrefSrcFile, "excl-src-file", "d", "", "file with hrefs on separate lines to be used in as a consumer exclude. Can be a csv with hrefs in first column. Headers optional")
	TrafficCmd.Flags().StringVarP(&inclServiceCSV, "incl-svc-file", "i", "", "file location of csv with port/protocols to exclude. Port number in column 1 and IANA numeric protocol in Col 2. Headers optional.")
	TrafficCmd.Flags().StringVarP(&exclServiceCSV, "excl-svc-file", "j", "", "file location of csv with port/protocols to exclude. Port number in column 1 and IANA numeric protocol in Col 2. Headers optional.")
	TrafficCmd.Flags().StringVarP(&start, "start", "s", time.Now().AddDate(0, 0, -88).In(time.UTC).Format("2006-01-02"), "start date in the format of yyyy-mm-dd.")
	TrafficCmd.Flags().StringVarP(&end, "end", "e", time.Now().Add(time.Hour*24).Format("2006-01-02"), "end date in the format of yyyy-mm-dd.")
	TrafficCmd.Flags().IntVarP(&maxResults, "max-results", "m", 100000, "max results in explorer. Maximum value is 100000")
	TrafficCmd.Flags().BoolVar(&exclAllowed, "excl-allowed", false, "excludes allowed traffic flows.")
	TrafficCmd.Flags().BoolVar(&exclPotentiallyBlocked, "excl-potentially-blocked", false, "excludes potentially blocked traffic flows.")
	TrafficCmd.Flags().BoolVar(&exclBlocked, "excl-blocked", false, "excludes blocked traffic flows.")
	TrafficCmd.Flags().IntVarP(&lookupTO, "time", "t", 1000, "timeout to lookup hostname in ms. 0 will skip hostname lookups.")
	TrafficCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	TrafficCmd.Flags().SortFlags = false

}

// TrafficCmd runs the workload identifier
var TrafficCmd = &cobra.Command{
	Use:   "traffic [csv file with input services]",
	Short: "Find and label unmanaged workloads and label existing workloads based on Explorer traffic and an input CSV.",
	Long: `
Find and label unmanaged workloads and label existing workloads based on Explorer traffic and an input CSV.

The --update-pce and --no-prompt flags are ignored for this command. Use workloader import to upload to PCE after review.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get CSV File
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		workloadIdentifier()
	},
}

type result struct {
	csname      string
	ipAddress   string
	fqdn        string
	hostname    string
	app         string
	env         string
	loc         string
	role        string
	reason      string
	eApp        string
	eEnv        string
	eLoc        string
	eRole       string
	wlHref      string
	matchStatus int // 0 = Existing Workload Match; 1 = UMWL Match; 2 = Existing Workload No Match
}

// Workload Labels
func (m *result) existingLabels(workloads map[string]illumioapi.Workload, labels map[string]illumioapi.Label) {
	if workloads[m.ipAddress].Labels != nil {
		for _, l := range *workloads[m.ipAddress].Labels {
			switch {
			case labels[l.Href].Key == "app":
				{
					m.eApp = labels[l.Href].Value
				}
			case labels[l.Href].Key == "role":
				{
					m.eRole = labels[l.Href].Value
				}
			case labels[l.Href].Key == "env":
				{
					m.eEnv = labels[l.Href].Value
				}
			case labels[l.Href].Key == "loc":
				{
					m.eLoc = labels[l.Href].Value
				}
			}
		}
	}
}

// RFC 1918 Check
func rfc1918(ipAddr string) bool {
	check := false
	rfc1918 := []string{"192.168.0.0/16", "172.16.0.0/12", "10.0.0.0/8"}
	// Iterate through the three RFC 1918 ranges
	for _, cidr := range rfc1918 {
		// Get the ipv4Net
		_, ipv4Net, _ := net.ParseCIDR(cidr)
		// Check if it is in the range
		check = ipv4Net.Contains(net.ParseIP(ipAddr))
		// If we get a true, append to the slice and stop checking the other ranges
		if check {
			break
		}
	}
	return check
}

// Hostname Lookup
func hostname(ipAddr string, t int) string {
	var hostname string
	ctx, cancel := context.WithTimeout(context.TODO(), time.Duration(t)*time.Millisecond)
	defer cancel()
	var r net.Resolver
	names, _ := r.LookupAddr(ctx, ipAddr)
	if len(names) > 2 {
		hostname = fmt.Sprintf("%s; %s; and %d more", names[0], names[1], len(names)-2)
	} else {
		hostname = strings.Join(names, ";")
	}
	return hostname
}

func workloadIdentifier() {

	utils.LogStartCommand("traffic")
	// Parse the iunput CSVs
	coreServices := parseCoreServices(csvFile)

	// Get Labels and workloads
	apiResps, err := pce.Load(illumioapi.LoadInput{Labels: true, Workloads: true})
	utils.LogMultiAPIResp(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get all workloads and create workload map
	allIPWLs := make(map[string]illumioapi.Workload)
	for _, wl := range pce.WorkloadsSlice {
		for _, iface := range wl.Interfaces {
			// We are going to use the workloads name field. If hostname is populated and not an IP address, we put that value in workload name to use the hostname
			if net.ParseIP(wl.Hostname) == nil && len(wl.Hostname) > 0 {
				wl.Name = wl.Hostname
			}
			allIPWLs[iface.Address] = wl
		}
	}

	// Create the default query struct
	tq := illumioapi.TrafficQuery{ExcludeWorkloadsFromIPListQuery: true}

	// Check max results for valid value
	if maxResults < 1 || maxResults > 100000 {
		utils.LogError("max-results must be between 1 and 100000")
	}
	tq.MaxFLows = maxResults

	// Build policy status slice
	if !exclAllowed {
		tq.PolicyStatuses = append(tq.PolicyStatuses, "allowed")
	}
	if !exclPotentiallyBlocked {
		tq.PolicyStatuses = append(tq.PolicyStatuses, "potentially_blocked")
	}
	if !exclBlocked {
		tq.PolicyStatuses = append(tq.PolicyStatuses, "blocked")
	}
	if !exclAllowed && !exclPotentiallyBlocked && !exclBlocked {
		tq.PolicyStatuses = []string{}
	}

	// Get the start date
	tq.StartTime, err = time.Parse("2006-01-02 MST", fmt.Sprintf("%s %s", start, "UTC"))
	if err != nil {
		utils.LogError(err.Error())
	}
	tq.StartTime = tq.StartTime.In(time.UTC)

	// Get the end date
	tq.EndTime, err = time.Parse("2006-01-02 15:04:05 MST", fmt.Sprintf("%s 23:59:59 %s", end, "UTC"))
	if err != nil {
		utils.LogError(err.Error())
	}
	tq.EndTime = tq.EndTime.In(time.UTC)

	// Get the services
	if exclServiceCSV != "" {
		tq.PortProtoExclude, err = utils.GetServicePortsCSV(exclServiceCSV)
		if err != nil {
			utils.LogError(err.Error())
		}
	}
	if inclServiceCSV != "" {
		tq.PortProtoInclude, err = utils.GetServicePortsCSV(inclServiceCSV)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	// Get the Include Source
	if inclHrefSrcFile != "" {
		// Parse the file
		d, err := utils.ParseCSV(inclHrefSrcFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		// For each entry in the file, add an include - OR operator
		// Semi-colons are used to differentiate hrefs in the same include - AND operator.
		for _, entry := range d {
			tq.SourcesInclude = append(tq.SourcesInclude, strings.Split(strings.ReplaceAll(entry[0], "; ", ";"), ";"))
		}
	} else {
		tq.SourcesInclude = append(tq.SourcesInclude, make([]string, 0))
	}

	// Get the Include Destination
	if inclHrefDstFile != "" {
		// Parse the file
		d, err := utils.ParseCSV(inclHrefDstFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		// For each entry in the file, add an include - OR operator
		// Semi-colons are used to differentiate hrefs in the same include - AND operator.
		for _, entry := range d {
			tq.DestinationsInclude = append(tq.DestinationsInclude, strings.Split(strings.ReplaceAll(entry[0], "; ", ";"), ";"))
		}
	} else {
		tq.DestinationsInclude = append(tq.DestinationsInclude, make([]string, 0))
	}

	// Get the Exclude Sources
	if exclHrefSrcFile != "" {
		// Parse the file
		d, err := utils.ParseCSV(exclHrefSrcFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		// For each entry in the file, add an exclude - OR operator
		for _, entry := range d {
			tq.SourcesExclude = append(tq.SourcesExclude, entry[0])
		}
	}

	// Get the Exclude Destinations
	if exclHrefDstFile != "" {
		// Parse the file
		d, err := utils.ParseCSV(exclHrefDstFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		// For each entry in the file, add an exclude - OR operator
		for _, entry := range d {
			tq.DestinationsExclude = append(tq.DestinationsExclude, entry[0])
		}
	}

	// Focus on unicast
	tq.TransmissionExcludes = []string{"broadcast", "multicast"}

	// Run traffic query
	traffic, a, err := pce.GetTrafficAnalysis(tq)
	utils.LogAPIResp("GetTrafficAnalysis", a)
	if err != nil {
		utils.LogError(fmt.Sprintf("making explorer API call - %s", err))
	}
	utils.LogInfo(fmt.Sprintf("explorer query returned %d records", len(traffic)), true)

	// Get matches for provider ports (including non-match existing workloads), consumer ports, and processes
	ignoreSameSubnetCS := []coreService{}
	includeSameSubnetCS := []coreService{}
	for _, cs := range coreServices {
		if cs.ignoreSameSubnet {
			ignoreSameSubnetCS = append(ignoreSameSubnetCS, cs)
		} else {
			includeSameSubnetCS = append(includeSameSubnetCS, cs)
		}
	}

	ignoreSameSubnetTraffic := []illumioapi.TrafficAnalysis{}
	includeSameSubnetTraffic := []illumioapi.TrafficAnalysis{}
	for _, t := range traffic {
		var network, ipCheck string
		// Skip any traffic that is between same workload
		if t.Src.IP == t.Dst.IP {
			continue
		}
		// All traffic goes into all bucker
		includeSameSubnetTraffic = append(includeSameSubnetTraffic, t)

		// Look at destination first. If the Destination does not have a workload, look at the source
		if t.Dst.Workload != nil && t.Dst.Workload.Href != "" {
			dstWkld := pce.Workloads[t.Dst.Workload.Href]
			if dstWkld.GetMode() != "unmanaged" && dstWkld.GetNetworkWithDefaultGateway() != "NA" {
				network = dstWkld.GetNetworkWithDefaultGateway()
				ipCheck = t.Src.IP
			}
		} else if t.Src.Workload != nil && t.Src.Workload.Href != "" {
			srcWkld := pce.Workloads[t.Src.Workload.Href]
			if srcWkld.GetMode() != "unmanaged" && srcWkld.GetNetworkWithDefaultGateway() != "NA" {
				network = srcWkld.GetNetworkWithDefaultGateway()
				ipCheck = t.Dst.IP
			}
		}

		// Parse the network
		_, ipNet, err := net.ParseCIDR(network)

		// If we can't parse the network, put it in the ignore to be safe
		if err != nil {
			ignoreSameSubnetTraffic = append(ignoreSameSubnetTraffic, t)
			continue
		}
		// Check the IP. If it is NOT in the same subnet, add it to the ignore
		ip := net.ParseIP(ipCheck)
		if !ipNet.Contains(ip) {
			ignoreSameSubnetTraffic = append(ignoreSameSubnetTraffic, t)
		}

	}
	utils.LogInfo(fmt.Sprintf("explorer query returned %d records after removing flows with same src and dst", len(includeSameSubnetTraffic)), true)
	utils.LogInfo(fmt.Sprintf("explorer query returned %d records for core services ignoring same subnet traffic", len(ignoreSameSubnetTraffic)), true)
	portProv1, _ := findPorts(ignoreSameSubnetTraffic, ignoreSameSubnetCS, true)
	portCons1, _ := findPorts(ignoreSameSubnetTraffic, ignoreSameSubnetCS, false)
	portProv2, _ := findPorts(includeSameSubnetTraffic, includeSameSubnetCS, true)
	portCons2, _ := findPorts(includeSameSubnetTraffic, includeSameSubnetCS, false)
	process := findProcesses(includeSameSubnetTraffic, coreServices)

	// Get matches for

	// Make one slice from port port results (prov and cons), processes, and nonmatches
	results := append(portProv1, portProv2...)
	results = append(append(results, portCons1...), portCons2...)
	results = append(results, process...)

	// Create the final matches array
	finalMatches := []result{}

	// Create a map to keep track of when we write a match.
	ipAddr := make(map[string]int)

	// For each coreservice, cycle through the results.
	i := 0
	for _, cs := range coreServices {
		i++
		for _, r := range results {
			// Only process results that haven't been matched
			if _, ok := ipAddr[r.ipAddress]; !ok {
				// If the result isn't mathched yet, we will process it if:
				// 1) It's matched on the current core-service OR
				// 2) It's a non-match, existing workload, and we are done cyclying through core services (last entry)
				if r.csname == cs.name || (r.matchStatus == 2 && allIPWLs[r.ipAddress].Href != "" && i == len(coreServices)) {
					// Set hostnames and HREF for existing workloads
					r.hostname = allIPWLs[r.ipAddress].Name
					r.wlHref = allIPWLs[r.ipAddress].Href
					// Set hostname for non-existing workloads
					if _, ok := allIPWLs[r.ipAddress]; !ok {
						r.matchStatus = 1 // UMWL status code
						// Default hostname is FQDN or IP. Lookup used to override IP if FQDN is blank.
						r.hostname = r.fqdn
						if r.hostname == "" {
							r.hostname = r.ipAddress
						}
						if lookupTO > 0 && r.fqdn == "" {
							h := hostname(r.ipAddress, lookupTO)
							if h != "" {
								r.hostname = h
							}
						}
					}
					// Populate existing label information
					r.existingLabels(allIPWLs, pce.Labels)

					// Append results to a new array if RFC 1918 and that's all we want OR we don't care about RFC 1918.
					if rfc1918(r.ipAddress) && privOnly || !privOnly {
						finalMatches = append(finalMatches, r)
						ipAddr[r.ipAddress] = 1
					}
				}
			}
		}
	}

	// If we have results, send to writing CSV
	if len(results) > 0 {
		csvWriter(finalMatches, exclWLs, outputFileName)
	}

	utils.LogEndCommand("traffic")
}
