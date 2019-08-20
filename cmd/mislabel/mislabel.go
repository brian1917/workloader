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

var appFlag, exclWkldFile, exclPortFile, outFormat string
var debug, ignoreLoc bool
var pce illumioapi.PCE
var err error

func init() {
	MisLabelCmd.Flags().StringVarP(&appFlag, "app", "a", "", "App label to limit Explorer query.")
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
	
The default Explorer query will look at all data. Explorer API has a max of 100,000 records. If you're query will exceed this, use the app flag to work through application labels. The app flag will get all traffic where that app is the source or destination (in 2 separate queries).
	
The explorer query will ignore traffic on UDP ports 5355 (DNSCache) and 137, 138, 139 (NETBIOS). To customize this list, use the --pExclude (-p) flag to pass in a CSV with no headers and two columns. First column is port number and second column is protocol number (TCP is 6 and UDP is 17). CSV should include above ports to exclude them.
	
The --update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetPCE()
		if err != nil {
			utils.Log(1, fmt.Sprintf("error getting pce - %s", err))
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

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

	// If app flag is set, adjust tq struct
	if appFlag != "" {
		l, a, err := pce.GetLabelbyKeyValue("app", appFlag)
		if debug {
			utils.LogAPIResp("GetLabelbyKeyValue", a)
		}
		if err != nil {
			utils.Log(1, err.Error())
		}
		tq.SourcesInclude = []string{l.Href}
	}

	// Get traffic
	traffic, apiResp, err := pce.GetTrafficAnalysis(tq)
	if debug {
		utils.LogAPIResp("GetTrafficAnalysis", apiResp)
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("error making traffic api call - %s", err))
	}

	// If app flag is set, edit tq stuct, run again and append.
	// We will have duplicate entries here, but it won't matter with logic.
	if appFlag != "" {
		tq.DestinationsInclude = tq.SourcesInclude
		tq.SourcesInclude = []string{}
		traffic2, apiResp, err := pce.GetTrafficAnalysis(tq)
		if debug {
			utils.LogAPIResp("GetTrafficAnalysis", apiResp)
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("error making traffic api call - %s", err))
		}
		traffic = append(traffic, traffic2...)
	}

	// nonOrphans will hold workloads that are not orphans
	nonOrphpans := make(map[string]bool)

	// Get the Href label map
	labelmap, apiResp, err := pce.GetLabelMapH()
	if debug {
		utils.Log(2, fmt.Sprintf("get href label map returned %d entries", len(labelmap)))
		utils.LogAPIResp("getLabelMap", apiResp)
	}
	if err != nil {
		utils.Log(1, fmt.Sprintf("getting labelmap - %s", err))
	}

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
		if ignoreLoc {
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

	// Get all workloads.
	wklds, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloads", a)
	}
	if err != nil {
		utils.Log(1, err.Error())
	}

	// Build a map of app groups and their count
	appGroupCount := make(map[string]int)
	for _, w := range wklds {
		appGrp := w.GetAppGroupL(labelmap)
		if ignoreLoc {
			appGrp = w.GetAppGroup(labelmap)
		}
		appGroupCount[appGrp] = appGroupCount[appGrp] + 1
	}

	// Iterate through each workload. If it's an orphan and not in our exclude list and not the only workload in the app group, add it to the slice.
	// We need to iterate two separate times because we need the complete list processed to get our count above
	orphanWklds := []illumioapi.Workload{}
	for _, w := range wklds {
		appGrp := w.GetAppGroupL(labelmap)
		if ignoreLoc {
			appGrp = w.GetAppGroup(labelmap)
		}
		if !nonOrphpans[w.Href] && !exclWklds[w.Hostname] && appGroupCount[appGrp] > 1 {
			orphanWklds = append(orphanWklds, w)
		}
	}

	// Create CSV output - start data slice with headers
	data := [][]string{[]string{"hostname", "role", "app", "env", "loc"}}
	for _, w := range orphanWklds {
		data = append(data, []string{w.Hostname, w.GetRole(labelmap).Value, w.GetApp(labelmap).Value, w.GetEnv(labelmap).Value, w.GetLoc(labelmap).Value})
	}

	if len(data) > 1 {
		fmt.Printf("\r\n%d potentially mislabeled workloads detected.\r\n", len(data)-1)
		utils.WriteOutput(data, fmt.Sprintf("workloader-mislabel-%s-.csv", time.Now().Format("20060102_150405")))
		utils.Log(0, fmt.Sprintf("mislabel complete - %d workloads identified", len(data)-1))
	} else {
		// Log if we don't find any
		fmt.Println("\r\n0 potentially mislabeled workloads detected.")
		utils.Log(0, "mislabel complete - 0 workloads identified")
	}
}
