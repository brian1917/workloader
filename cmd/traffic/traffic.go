package traffic

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var inclHrefDstFile, exclHrefDstFile, inclHrefSrcFile, exclHrefSrcFile, inclServiceCSV, exclServiceCSV, inclProcessCSV, exclProcessCSV, start, end, outputFileName string
var exclAllowed, exclPotentiallyBlocked, exclBlocked, exclUnknown, nonUni, exclWorkloadsFromIPListQuery, draftPolicy bool
var maxResults int
var pce illumioapi.PCE
var err error

func init() {

	TrafficCmd.Flags().StringVarP(&inclHrefDstFile, "incl-dst-file", "a", "", "file with hrefs on separate lines to be used in as a provider include. Each line is treated as OR logic. On same line, combine hrefs of same object type with a semi-colon separator for an AND logic. Headers optional.")
	TrafficCmd.Flags().StringVarP(&exclHrefDstFile, "excl-dst-file", "b", "", "file with hrefs on separate lines to be used in as a provider exclude. Can be a csv with hrefs in first column. Headers optional")
	TrafficCmd.Flags().StringVarP(&inclHrefSrcFile, "incl-src-file", "c", "", "file with hrefs on separate lines to be used in as a consumer include. Each line is treated as OR logic. On same line, combine hrefs of same object type with a semi-colon separator for an AND logic. Headers optional.")
	TrafficCmd.Flags().StringVarP(&exclHrefSrcFile, "excl-src-file", "d", "", "file with hrefs on separate lines to be used in as a consumer exclude. Can be a csv with hrefs in first column. Headers optional")
	TrafficCmd.Flags().StringVarP(&inclServiceCSV, "incl-svc-file", "i", "", "file location of csv with port/protocols to include. Port number in column 1 and IANA numeric protocol in Col 2. Headers optional.")
	TrafficCmd.Flags().StringVarP(&exclServiceCSV, "excl-svc-file", "j", "", "file location of csv with port/protocols to exclude. Port number in column 1 and IANA numeric protocol in Col 2. Headers optional.")
	TrafficCmd.Flags().StringVarP(&inclProcessCSV, "incl-proc-file", "k", "", "file location of csv with single column of processes to include. No headers.")
	TrafficCmd.Flags().StringVarP(&exclProcessCSV, "excl-proc-file", "n", "", "file location of csv with single column of processes to exclude. No headers.")
	TrafficCmd.Flags().StringVarP(&start, "start", "s", time.Now().AddDate(0, 0, -88).In(time.UTC).Format("2006-01-02"), "start date in the format or yyyy-mm-dd or yyyy-mm-ddTHH:mm:ss. if no time is provided, 00:00:00 is used. all times in GMT.")
	TrafficCmd.Flags().StringVarP(&end, "end", "e", time.Now().Add(time.Hour*24).Format("2006-01-02"), "end date in the format of yyyy-mm-dd or yyyy-mm-dd or yyyy-mm-ddTHH:mm:ss. if no time is provided, 23:59:59 is used. all times in GMT.")
	TrafficCmd.Flags().BoolVar(&exclWorkloadsFromIPListQuery, "excl-wkld-from-iplist-query", true, "exclude workload traffic when ip list is provided either in consumer or provider part of the traffic query. default of true matches UI")
	TrafficCmd.Flags().BoolVar(&exclAllowed, "excl-allowed", false, "excludes allowed traffic flows.")
	TrafficCmd.Flags().BoolVar(&exclPotentiallyBlocked, "excl-potentially-blocked", false, "excludes potentially blocked traffic flows.")
	TrafficCmd.Flags().BoolVar(&exclBlocked, "excl-blocked", false, "excludes blocked traffic flows.")
	TrafficCmd.Flags().BoolVar(&exclUnknown, "excl-unknown", false, "excludes unkown policy decision traffic flows.")
	TrafficCmd.Flags().BoolVar(&nonUni, "incl-non-unicast", false, "includes non-unicast (broadcast and multicast) flows in the output. Default is unicast only.")
	TrafficCmd.Flags().IntVarP(&maxResults, "max-results", "m", 100000, "max results in explorer. Maximum value is 200000.")
	TrafficCmd.Flags().BoolVar(&draftPolicy, "draft", false, "include draft policy decision in results (added time to queries).")
	TrafficCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename. If iterating through labels, the labels will be appended to the provided name before the provided file extension. To name the files for the labels, use just an extension (--output-file .csv).")

	TrafficCmd.Flags().SortFlags = false
}

// TrafficCmd summarizes flows
var TrafficCmd = &cobra.Command{
	Use:   "traffic",
	Short: "Export traffic data.",
	Long: `
Export traffic data.

See the flags for filtering options.

Use the following commands to get necessary HREFs for include/exlude files: label-export, ipl-export, wkld-export.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set output to CSV only
		viper.Set("output_format", "csv")

		explorerExport()
	},
}

func explorerExport() {

	// Create the default query struct
	tq := illumioapi.TrafficQuery{ExcludeWorkloadsFromIPListQuery: exclWorkloadsFromIPListQuery}

	// Check max results for valid value
	if maxResults < 1 || maxResults > 200000 {
		utils.LogError("max-results must be between 1 and 200000")
	}
	tq.MaxFLows = maxResults

	// Get Labels and workloads
	apiResps, err := pce.Load(illumioapi.LoadInput{Labels: true, Workloads: true}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

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
	if !exclUnknown {
		tq.PolicyStatuses = append(tq.PolicyStatuses, "unknown")
	}
	if !exclAllowed && !exclPotentiallyBlocked && !exclBlocked && !exclUnknown {
		tq.PolicyStatuses = []string{}
	}

	// Get the start date
	timeFormat := "2006-01-02 MST"
	if strings.Contains(start, ":") {
		timeFormat = "2006-01-02T15:04:05 MST"
	}
	tq.StartTime, err = time.Parse(timeFormat, fmt.Sprintf("%s UTC", start))
	if err != nil {
		utils.LogErrorf("error parsing start time: %s", err)
	}
	tq.StartTime = tq.StartTime.In(time.UTC)

	// Get the end date
	if strings.Contains(end, ":") {
		tq.EndTime, err = time.Parse("2006-01-02T15:04:05 MST", fmt.Sprintf("%s UTC", end))
	} else {
		tq.EndTime, err = time.Parse("2006-01-02 15:04:05 MST", fmt.Sprintf("%s 23:59:59 UTC", end))
	}
	if err != nil {
		utils.LogErrorf("error parsing end time: %s", err)
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

	// Get the processes
	if inclProcessCSV != "" {
		tq.ProcessInclude, err = utils.GetProcesses(inclProcessCSV)
		if err != nil {
			utils.LogError(err.Error())
		}
	}
	if exclProcessCSV != "" {
		tq.ProcessExclude, err = utils.GetProcesses(exclProcessCSV)
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

	// Exclude broadcast and multicast, unless flag set to include non-unicast flows
	if !nonUni {
		tq.TransmissionExcludes = []string{"broadcast", "multicast"}
	}

	// Set some variables for traffic analysis

	traffic, a, err := pce.GetTrafficAnalysisCsv(tq, draftPolicy)
	utils.LogInfo("making explorer query", false)
	utils.LogInfo(a.ReqBody, false)
	utils.LogAPIRespV2("GetTrafficAnalysis", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	outFileName := fmt.Sprintf("workloader-explorer-%s.csv", time.Now().Format("20060102_150405"))
	if outputFileName != "" {
		outFileName = outputFileName
	}

	utils.WriteOutput(traffic, nil, outFileName)
	utils.LogInfo(fmt.Sprintf("%d traffic records exported", len(traffic)-1), true)

}
