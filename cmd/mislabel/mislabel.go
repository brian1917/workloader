package mislabel

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var envFlag, appFlag, locFlag, exclWkldFile, exclPortFile string
var pce illumioapi.PCE
var err error

func init() {
	MisLabelCmd.Flags().StringVarP(&appFlag, "app", "a", "", "App label.")
	MisLabelCmd.Flags().StringVarP(&envFlag, "env", "r", "", "Role label.")
	MisLabelCmd.Flags().StringVarP(&locFlag, "loc", "l", "", "Location label.")
	MisLabelCmd.Flags().StringVarP(&exclWkldFile, "wExclude", "w", "", "File location of hostnames to exclude as orphans.")
	MisLabelCmd.Flags().StringVarP(&exclPortFile, "pExclude", "p", "", "File location of ports to exclude in traffic query.")
	MisLabelCmd.Flags().SortFlags = false
}

// MisLabelCmd Finds workloads that have no communications within an App-Group....
var MisLabelCmd = &cobra.Command{
	Use:   "mislabel",
	Short: "Display workloads that have no intra App-Group communications to identify potentially mislabled workloads.",
	Long: `
	Display workloads that have no intra App-Group communications to identify potentially mislabled workloads.
	
	The explorer query will ignore traffic on UDP ports 5355 (DNSCache) and 137, 138, 139 (NETBIOS). To customize this list, use the --pExclude (-p) flag to pass in a CSV with no headers and two columns. First column is port number and second column is protocol number (TCP is 6 and UDP is 17). CSV should include above ports to exclude them.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetPCE()
		if err != nil {
			utils.Log(1, fmt.Sprintf("error getting pce - %s", err))
		}

		misLabel()
	},
}

func getExclHosts(filename string) map[string]bool {
	// Open CSV File
	csvFile, _ := os.Open(filename)
	reader := csv.NewReader(bufio.NewReader(csvFile))

	exclHosts := make(map[string]bool)

	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("Reading CSV File - %s", err))
		}
		exclHosts[line[0]] = true
	}

	return exclHosts
}

func getExclPorts(filename string) [][2]int {
	// Open CSV File
	csvFile, _ := os.Open(filename)
	reader := csv.NewReader(bufio.NewReader(csvFile))

	exclPorts := [][2]int{}

	n := 0
	for {
		n++
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("Reading CSV File - %s", err))
		}

		port, err := strconv.Atoi(line[0])
		if err != nil {
			utils.Log(1, fmt.Sprintf("Non-integer port value on line %d - %s", n, err))
		}
		protocol, err := strconv.Atoi(line[1])
		if err != nil {
			utils.Log(1, fmt.Sprintf("Non-integer protocol value on line %d - %s", n, err))
		}

		exclPorts = append(exclPorts, [2]int{port, protocol})
	}

	return exclPorts
}

//misLabel determines if workloads in an app-group only communicate outside the app-group.
func misLabel() {

	// Log start
	utils.Log(0, "started mislabel command")
	debug := true
	ignoreloc := false

	// Get the labelMap
	labelmap, apiResp, err := illumioapi.GetLabelMapH(pce)
	if debug == true {
		utils.Log(2, fmt.Sprintf("get href label map returned %d entries", len(labelmap)))
		utils.LogAPIResp("getLabelMap", apiResp)
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting labelmap - %s", err))
	}

	// Get all workloads
	wklds, _, err := illumioapi.GetAllWorkloads(pce)
	if err != nil {
		utils.Log(1, fmt.Sprintf("error getting all workloads - %s", err))
	}

	// Get ports we should ignore
	exclPorts := [][2]int{[2]int{5355, 17}, [2]int{137, 17}, [2]int{138, 17}, [2]int{139, 17}}
	if exclPortFile != "" {
		exclPorts = getExclPorts(exclPortFile)
	}

	// Build the traffic query struct
	tq := illumioapi.TrafficQuery{
		StartTime:        time.Date(2013, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:          time.Now(),
		PolicyStatuses:   []string{"allowed", "potentially_blocked", "blocked"},
		PortRangeExclude: exclPorts,
		MaxFLows:         100000}

	// Get traffic
	traffic, apiResp, err := illumioapi.GetTrafficAnalysis(pce, tq)
	if err != nil {
		utils.Log(1, fmt.Sprintf("error making traffic api call - %s", err))
	}
	if debug {
		utils.LogAPIResp("GetTrafficAnalysis", apiResp)
	}

	// nonOrphans will hold workloads that are not orphans
	nonOrphpans := make(map[string]bool)

	// Iterate through each traffic entry
	for _, ta := range traffic {

		// If the source or destination is not a workload, we are done with this traffic entry
		if ta.Src.Workload == nil || ta.Dst.Workload == nil {
			continue
		}

		// If source and destination are the same, we are done with this traffic entry
		if ta.Src.Workload.Href == ta.Dst.Workload.Href {
			continue
		}

		// Get the App Groups
		srcAppGroup := ta.Src.Workload.GetAppGroupL(labelmap)
		dstAppGroup := ta.Dst.Workload.GetAppGroupL(labelmap)
		if ignoreloc {
			srcAppGroup = ta.Src.Workload.GetAppGroup(labelmap)
			dstAppGroup = ta.Dst.Workload.GetAppGroup(labelmap)
		}

		// If the app groups are the same, mark the source and destination as true and stop processing
		if srcAppGroup == dstAppGroup && srcAppGroup != "NO APP GROUP" {
			nonOrphpans[ta.Src.Workload.Href] = true
			nonOrphpans[ta.Dst.Workload.Href] = true
			continue
		}
	}

	// Get the excluded workload list
	exclWklds := make(map[string]bool)
	if exclWkldFile != "" {
		exclWklds = getExclHosts(exclWkldFile)
	}

	// Iterate through each workload. If it's an orphan and not in our exclude list, add it to the slice.
	var orphanWklds []illumioapi.Workload
	for _, w := range wklds {
		if !nonOrphpans[w.Href] && !exclWklds[w.Hostname] {
			orphanWklds = append(orphanWklds, w)
		}
	}

	// Print each orphan
	for _, w := range orphanWklds {
		fmt.Println(w.Hostname)
	}
}
