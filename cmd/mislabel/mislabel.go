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
	"github.com/spf13/viper"
)

var appFlag, exclWkldFile, exclPortFile, exclAppFile, outFormat, outputFileName string
var debug, ignoreLoc, inclUnmanagedAppGroups bool
var pce illumioapi.PCE
var err error

func init() {
	MisLabelCmd.Flags().StringVarP(&appFlag, "app", "a", "", "App label to limit Explorer query.")
	MisLabelCmd.Flags().BoolVarP(&inclUnmanagedAppGroups, "inclUn", "u", false, "Include app groups that are all unmanaged workloads.")
	MisLabelCmd.Flags().StringVarP(&exclWkldFile, "wExclude", "w", "", "File location of hostnames to exclude as orphans.")
	MisLabelCmd.Flags().StringVarP(&exclAppFile, "aExclude", "x", "", "File location of app labels to exclude as orphans.")
	MisLabelCmd.Flags().StringVarP(&exclPortFile, "pExclude", "p", "", "File location of ports to exclude in traffic query.")
	MisLabelCmd.Flags().BoolVar(&ignoreLoc, "ignore-location", false, "Do not use location in comparing app groups.")
	MisLabelCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	MisLabelCmd.Flags().SortFlags = false
}

// MisLabelCmd Finds workloads that have no communications within an App-Group....
var MisLabelCmd = &cobra.Command{
	Use:   "mislabel",
	Short: "Display workloads that have no intra App-Group communications to identify potentially mislabled workloads.",
	Long: `
Display workloads that have no intra App-Group communications to identify potentially mislabled workloads.

Workloads will not be identified as orphans if they are part of an app group with only unmanaged workloads.
	
The default Explorer query will look at all data. Explorer API has a max of 100,000 records. If you're query will exceed this, use the app flag to work through application labels. The app flag will get all traffic where that app is the source or destination.
	
The explorer query will ignore traffic on UDP ports 5355 (DNSCache) and 137, 138, 139 (NETBIOS). To customize this list, use the --pExclude (-p) flag to pass in a CSV with no headers and two columns. First column is port number and second column is protocol number (TCP is 6 and UDP is 17). If using the CSV option, UDP 5355, 137, 138, and 139 are not exlucded by default; you must add them to the list.
	
The --update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting pce - %s", err))
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		misLabel()
	},
}

func getExclHostsOrApps(filename string) map[string]bool {
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
			utils.LogError(fmt.Sprintf("Reading CSV File - %s", err))
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
			utils.LogError(fmt.Sprintf("Reading CSV File - %s", err))
		}

		port, err := strconv.Atoi(line[0])
		if err != nil {
			utils.LogError(fmt.Sprintf("Non-integer port value on line %d - %s", n, err))
		}
		protocol, err := strconv.Atoi(line[1])
		if err != nil {
			utils.LogError(fmt.Sprintf("Non-integer protocol value on line %d - %s", n, err))
		}

		exclPorts = append(exclPorts, [2]int{port, protocol})
	}

	return exclPorts
}

//misLabel determines if workloads in an app-group only communicate outside the app-group.
func misLabel() {

	// Log start
	utils.LogStartCommand("mislabel")

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
		PortProtoExclude: exclPorts,
		MaxFLows:         100000}

	// If app flag is set, adjust tq struct
	if appFlag != "" {
		l, a, err := pce.GetLabelbyKeyValue("app", appFlag)
		if debug {
			utils.LogAPIResp("GetLabelbyKeyValue", a)
		}
		if err != nil {
			utils.LogError(err.Error())
		}
		tq.SourcesInclude = [][]string{[]string{l.Href}}
	}

	// Get traffic
	traffic, apiResp, err := pce.GetTrafficAnalysis(tq)
	if debug {
		utils.LogAPIResp("GetTrafficAnalysis", apiResp)
	}
	if err != nil {
		utils.LogError(fmt.Sprintf("error making traffic api call - %s", err))
	}

	// If app flag is set, edit tq stuct, run again and append.
	// We will have duplicate entries here, but it won't matter with logic.
	if appFlag != "" {
		tq.DestinationsInclude = tq.SourcesInclude
		tq.SourcesInclude = [][]string{}
		traffic2, apiResp, err := pce.GetTrafficAnalysis(tq)
		if debug {
			utils.LogAPIResp("GetTrafficAnalysis", apiResp)
		}
		if err != nil {
			utils.LogError(fmt.Sprintf("error making traffic api call - %s", err))
		}
		traffic = append(traffic, traffic2...)
	}

	// nonOrphans will hold workloads that are not orphans
	nonOrphpans := make(map[string]bool)

	// Create a map for each workload to know if it has traffic reported
	wkldTrafficMap := make(map[string]int)

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

		// Iterate the workload traffic counter
		if ta.Src.Workload != nil {
			wkldTrafficMap[ta.Src.Workload.Href] = wkldTrafficMap[ta.Src.Workload.Href] + 1
		}
		if ta.Dst.Workload != nil {
			wkldTrafficMap[ta.Dst.Workload.Href] = wkldTrafficMap[ta.Dst.Workload.Href] + 1
		}

		// Get the App Groups
		srcAppGroup := ta.Src.Workload.GetAppGroupL(pce.Labels)
		dstAppGroup := ta.Dst.Workload.GetAppGroupL(pce.Labels)
		if ignoreLoc {
			srcAppGroup = ta.Src.Workload.GetAppGroup(pce.Labels)
			dstAppGroup = ta.Dst.Workload.GetAppGroup(pce.Labels)
		}

		// If the app groups are the same, mark the source and destination as true and stop processing
		if srcAppGroup == dstAppGroup && srcAppGroup != "NO APP GROUP" {
			nonOrphpans[ta.Src.Workload.Href] = true
			nonOrphpans[ta.Dst.Workload.Href] = true
			continue
		}
	}

	// Get the excluded workload list and app list
	exclWklds := make(map[string]bool)
	if exclWkldFile != "" {
		exclWklds = getExclHostsOrApps(exclWkldFile)
	}
	exclApps := make(map[string]bool)
	if exclAppFile != "" {
		exclApps = getExclHostsOrApps(exclAppFile)
	}

	// Get all workloads.
	wklds, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloads", a)
	}
	if err != nil {
		utils.LogError(err.Error())
	}

	// Build a map of app groups and their count
	appGroupCount := make(map[string]int)
	for _, w := range wklds {
		appGrp := w.GetAppGroupL(pce.Labels)
		if ignoreLoc {
			appGrp = w.GetAppGroup(pce.Labels)
		}
		appGroupCount[appGrp] = appGroupCount[appGrp] + 1
	}

	// Get managed workload counter
	managedWkldsInAppGroup := appGroupManagedCounter(wklds)

	// Iterate through each workload. If it's an orphan, not in our exclude list, not the only workload in the app group, and not in an AppGroup with only unmanaged workloads, add it to the slice.
	// We need to iterate two separate times because we need the complete list processed to get our count above
	orphanWklds := []illumioapi.Workload{}
	for _, w := range wklds {
		appGrp := w.GetAppGroupL(pce.Labels)
		if ignoreLoc {
			appGrp = w.GetAppGroup(pce.Labels)
		}
		// if the workload is not in non-orphans, not in exclude list,
		if !nonOrphpans[w.Href] && !exclWklds[w.Hostname] && !exclApps[w.GetApp(pce.Labels).Value] && appGroupCount[appGrp] > 1 && (managedWkldsInAppGroup[appGrp] > 0 || inclUnmanagedAppGroups) && wkldTrafficMap[w.Href] > 0 {
			orphanWklds = append(orphanWklds, w)
		}
	}

	// Create CSV output - start data slice with headers
	data := [][]string{[]string{"hostname", "role", "app", "env", "loc"}}
	for _, w := range orphanWklds {
		data = append(data, []string{w.Hostname, w.GetRole(pce.Labels).Value, w.GetApp(pce.Labels).Value, w.GetEnv(pce.Labels).Value, w.GetLoc(pce.Labels).Value})
	}

	if len(data) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-mislabel-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(data, data, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d potentially mislabeled workloads detected.", len(data)-1), true)
	} else {
		// Log if we don't find any
		utils.LogInfo(fmt.Sprintln("no potentially mislabeled workloads detected."), true)
	}

	utils.LogEndCommand("mislabel")
}

func appGroupManagedCounter(allWklds []illumioapi.Workload) map[string]int {
	// Initialize the map
	managedWkldCounter := make(map[string]int)

	// Iterate through each workload
	for _, w := range allWklds {
		// Get app group
		appGroup := w.GetAppGroupL(pce.Labels)
		if ignoreLoc {
			appGroup = w.GetAppGroup(pce.Labels)
		}

		// If this is an unmanaged workload, do nothing more
		if w.GetMode() == "unmanaged" {
			continue
		}

		// If this is not an unmanaged workload, increment the map
		managedWkldCounter[appGroup] = managedWkldCounter[appGroup] + 1
	}

	return managedWkldCounter

}
