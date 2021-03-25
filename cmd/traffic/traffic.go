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
	"github.com/spf13/viper"
)

var csvFile, app, consExcl, outputFileName string
var lookupTO int
var privOnly, exclWLs, debug bool
var pce illumioapi.PCE
var err error

func init() {
	TrafficCmd.Flags().IntVarP(&lookupTO, "time", "t", 1000, "timeout to lookup hostname in ms. 0 will skip hostname lookups.")
	TrafficCmd.Flags().StringVarP(&app, "app", "a", "", "app name to limit Explorer results to flows with that app as a provider or consumer. default is all apps")
	TrafficCmd.Flags().StringVarP(&consExcl, "exclConsumer", "e", "", "label to exclude as a consumer role")
	TrafficCmd.Flags().BoolVarP(&privOnly, "exclPubIPs", "p", false, "exclude public IP addresses and limit suggested workloads to the RFC 1918 address space")
	TrafficCmd.Flags().BoolVarP(&exclWLs, "exclWklds", "w", false, "exclude IP addresses already assigned to workloads to suggest or verify labels")
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

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

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

	// Get all workloads and create workload map
	allIPWLs := make(map[string]illumioapi.Workload)
	wls, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloads", a)
	}
	if err != nil {
		utils.LogError(fmt.Sprintf("getting all workloads - %s", err))
	}
	for _, wl := range wls {
		for _, iface := range wl.Interfaces {
			// We are going to use the workloads name field. If hostname is populated and not an IP address, we put that value in workload name to use the hostname
			if net.ParseIP(wl.Hostname) == nil && len(wl.Hostname) > 0 {
				wl.Name = wl.Hostname
			}
			allIPWLs[iface.Address] = wl
		}
	}

	// Create the default query struct
	tq := illumioapi.TrafficQuery{
		StartTime:      time.Date(2013, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:        time.Date(2020, 12, 30, 0, 0, 0, 0, time.UTC),
		PolicyStatuses: []string{"allowed", "potentially_blocked", "blocked"},
		MaxFLows:       100000}

	// Get the label if we are going to do a consumer exclude
	var exclLabel illumioapi.Label
	if len(consExcl) > 0 {
		exclLabel, _, err = pce.GetLabelbyKeyValue("role", consExcl)
		if err != nil {
			utils.LogError(fmt.Sprintf("getting label HREF - %s", err))
		}
		if exclLabel.Href == "" {
			utils.LogError(fmt.Sprintf("%s does not exist as an role label.", consExcl))
		}
		tq.SourcesExclude = []string{exclLabel.Href}
	}

	// If an app is provided, adjust query to include it
	if app != "" {
		label, a, err := pce.GetLabelbyKeyValue("app", app)
		if debug {
			utils.LogAPIResp("GetLabelbyKeyValue", a)
		}
		if err != nil {
			utils.LogError(fmt.Sprintf("getting label HREF - %s", err))
		}
		if label.Href == "" {
			utils.LogError(fmt.Sprintf("%s does not exist as an app label.", app))
		}
		tq.SourcesInclude = [][]string{[]string{label.Href}}
	}

	// Run traffic query
	traffic, a, err := pce.GetTrafficAnalysis(tq)
	if debug {
		utils.LogAPIResp("GetTrafficAnalysis", a)
	}
	if err != nil {
		utils.LogError(fmt.Sprintf("making explorer API call - %s", err))
	}

	// If app is provided, switch to the destination include, clear the sources include, run query again, append to previous result
	if app != "" {
		tq.DestinationsInclude = tq.SourcesInclude
		tq.SourcesInclude = [][]string{}
		traffic2, a, err := pce.GetTrafficAnalysis(tq)
		if debug {
			utils.LogAPIResp("GetTrafficAnalysis", a)
		}
		if err != nil {
			utils.LogError(fmt.Sprintf("making second explorer API call - %s", err))
		}
		traffic = append(traffic, traffic2...)
	}

	// Get matches for provider ports (including non-match existing workloads), consumer ports, and processes
	portProv, _ := findPorts(traffic, coreServices, true)
	portCons, _ := findPorts(traffic, coreServices, false)
	process := findProcesses(traffic, coreServices)

	// Make one slice from port port results (prov and cons), processes, and nonmatches
	results := append(append(portProv, portCons...), process...)

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
