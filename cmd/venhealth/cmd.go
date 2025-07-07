package venhealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pce illumioapi.PCE
var err error
var start, end, customEventList, outputFileName string
var yesterday, lastWeek, lastMonth, includeEventList bool
var maxResults int
var yesterdayStart, yesterdayEnd, lastWeekStart, lastWeekEnd, lastMonthStart, lastMonthEnd string

type uniqueEntry struct {
	Hostname string
	Events   map[string]int
}

var venHealthEvents []string = []string{
	"agent.clone_detected",
	"agent.deactivate",
	"agent.missing_heartbeats_after_upgrade",
	"agent.service_not_available",
	"agent.suspend",
	"agent.tampering",
	"agent.upgrade_time_out",
	"system_task.agent_missed_heartbeats_check",
	"system_task.agent_offline_check",
	"workload.offline_after_ven_goodbye",
}

func init() {

	_, offset := time.Now().Zone()

	yesterdayStart = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.FixedZone(fmt.Sprintf("UTC%d", offset), offset)).AddDate(0, 0, -1).Format(time.RFC3339)
	yesterdayEnd = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 999999999, time.FixedZone(fmt.Sprintf("UTC%d", offset), offset)).AddDate(0, 0, -1).Format(time.RFC3339)

	lastWeekStart = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.FixedZone(fmt.Sprintf("UTC%d", offset), offset)).AddDate(0, 0, -7).Format(time.RFC3339)
	lastWeekEnd = time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 23, 59, 59, 999999999, time.FixedZone(fmt.Sprintf("UTC%d", offset), offset)).AddDate(0, 0, -1).Format(time.RFC3339)

	lastMonthStart = time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.FixedZone(fmt.Sprintf("UTC%d", offset), offset)).AddDate(0, -1, 0).Format(time.RFC3339)
	lastMonthEnd = time.Date(time.Now().Year(), time.Now().Month(), 1, 23, 59, 59, 999999999, time.FixedZone(fmt.Sprintf("UTC%d", offset), offset)).AddDate(0, 0, -1).Format(time.RFC3339)

	VenHealthCmd.Flags().BoolVar(&yesterday, "yesterday", false, fmt.Sprintf("time range set to yesterday (%s to %s)", yesterdayStart, yesterdayEnd))
	VenHealthCmd.Flags().BoolVar(&lastWeek, "last-week", false, fmt.Sprintf("time range set to last week (%s to %s)", lastWeekStart, lastWeekEnd))
	VenHealthCmd.Flags().BoolVar(&lastMonth, "last-month", false, fmt.Sprintf("time range set to last month (%s to %s)", lastMonthStart, lastMonthEnd))
	VenHealthCmd.Flags().StringVar(&start, "start", "", "custom start date in RFC 3339 format")
	VenHealthCmd.Flags().StringVar(&end, "end", "", "custom end date in RFC 3339 format.")
	VenHealthCmd.Flags().IntVar(&maxResults, "max-results", 10000, "maximum results. max is 10,000.")
	VenHealthCmd.Flags().BoolVar(&includeEventList, "include-event-list", false, "include output of full event list with th summarized report.")
	VenHealthCmd.Flags().StringVar(&customEventList, "custom-event-list", "", fmt.Sprintf("text file with events on separate lines to override the default %d events", len(venHealthEvents)))
	VenHealthCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	VenHealthCmd.Flags().SortFlags = false
}

// WorkloadToIPLCmd runs the upload command
var VenHealthCmd = &cobra.Command{
	Use:   "ven-health",
	Short: "Create a CSV report of VEN health events for specific time period.",
	Long: `
Create a CSV report of VEN health events for specific time period

The monitored events are listed below:` + "\r\n\r\n" + strings.Join(venHealthEvents, "\r\n"),

	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Disable stdout
		viper.Set("output_format", "csv")
		if err := viper.WriteConfig(); err != nil {
			utils.LogError(err.Error())
		}

		// If the customEventList is provided, use that
		if customEventList != "" {
			venHealthEvents = []string{}
			data, err := utils.ParseCSV(customEventList)
			if err != nil {
				utils.LogError(err.Error())
			}
			for _, d := range data {
				venHealthEvents = append(venHealthEvents, d[0])
			}
		}

		eventMonitor(venHealthEvents)
	},
}

func eventMonitor(targetEvents []string) {

	// validate a time has been provided
	if (!yesterday && !lastWeek && !lastMonth) && (start == "" || end == "") {
		utils.LogError("a time period must be provided using either --yesterday, --last-week, or --last-month flags or the customer --start and --end flags.")
	}

	// Create the output slice
	allEvents := []illumioapi.Event{}

	// Create the query parameter map
	qp := make(map[string]string)

	// Max results
	if maxResults < 1 || maxResults > 10000 {
		utils.LogError("max results must be between 1 and 10,000")
	}
	qp["max_results"] = "10000"

	// Process Time
	if yesterday {
		utils.LogInfo("yesterday flag used", false)
		qp["timestamp[gte]"] = yesterdayStart
		qp["timestamp[lte]"] = yesterdayEnd
	} else if lastWeek {
		utils.LogInfo("last-week flag used", false)
		qp["timestamp[gte]"] = lastWeekStart
		qp["timestamp[lte]"] = lastWeekEnd
	} else if lastMonth {
		utils.LogInfo("last-month flag used", false)
		qp["timestamp[gte]"] = lastMonthStart
		qp["timestamp[lte]"] = lastMonthEnd
	} else {
		utils.LogInfo("custom start and end used", false)
		qp["timestamp[gte]"] = start
		qp["timestamp[lte]"] = end
	}

	// Log the start and end times
	utils.LogInfo(fmt.Sprintf("start: %s", qp["timestamp[gte]"]), true)
	utils.LogInfo(fmt.Sprintf("end: %s", qp["timestamp[lte]"]), true)

	// Create the summary map and unique VENs map
	summaryMap := make(map[string]string)
	uniqueVENs := make(map[string]uniqueEntry)

	// Iterate the target events
	for i, event := range targetEvents {

		// Make the API request
		qp["event_type"] = event
		events, a, err := pce.GetEvents(qp)
		utils.LogAPIRespV2("GetEvents", a)
		if err != nil {
			utils.LogErrorf("error getting events for %s - %s", event, err.Error())
		}

		// Append to the allEvents
		allEvents = append(allEvents, events...)

		// Iterate through the events
		for _, e := range events {

			// Get the hostname
			hostname := ""
			// Look at the VEN hostname first
			if e.EventCreatedBy.VEN.Hostname != nil && *e.EventCreatedBy.VEN.Hostname != "" {
				hostname = *e.EventCreatedBy.VEN.Hostname
				// If VEN doesn't have it, look at agent
			} else if e.EventCreatedBy.Agent != nil && e.EventCreatedBy.Agent.Hostname != "" {
				hostname = e.EventCreatedBy.Agent.Hostname
			} else { // If agent doesn't have it, look at resource changes
				for _, r := range illumioapi.PtrToVal(e.ResourceChanges) {
					if r.Resource.Workload.Hostname != nil && *r.Resource.Workload.Hostname != "" {
						hostname = *r.Resource.Workload.Hostname
						break
					}
				}
			}

			// Process the event and add to the uniqueVENs map
			if e.EventCreatedBy.VEN != nil {
				if val, exists := uniqueVENs[e.EventCreatedBy.VEN.Href]; exists {
					val.Events[e.EventType] = val.Events[e.EventType] + 1
				} else {
					uniqueVENs[e.EventCreatedBy.VEN.Href] = uniqueEntry{
						Hostname: hostname,
						Events:   map[string]int{e.EventType: 1},
					}
				}
			}

			// Add to the summary map
			summaryMap[event] = fmt.Sprintf("%d events over %d agents", len(events), len(uniqueVENs))
		}

		// Log the events
		utils.LogInfo(fmt.Sprintf("%d of %d - %s - %d events", i+1, len(targetEvents), event, len(events)), true)
	}

	// Output the CSV
	if len(uniqueVENs) > 0 {
		csvOut := [][]string{{"start:", qp["timestamp[gte]"], ""}, {"end:", qp["timestamp[lte]"], ""}, {"", "", ""}, {"summary", "", ""}}
		for event, summary := range summaryMap {
			csvOut = append(csvOut, []string{fmt.Sprintf("%s:", event), summary, ""})
		}
		csvOut = append(csvOut, []string{"", "", ""})
		csvOut = append(csvOut, []string{"ven details", "", ""})
		csvOut = append(csvOut, []string{"ven_href", "hostname", "events"})

		for href, uniqueVen := range uniqueVENs {
			events := []string{}
			for eventType, count := range uniqueVen.Events {
				events = append(events, fmt.Sprintf("%s (%d)", eventType, count))
			}
			csvOut = append(csvOut, []string{href, uniqueVen.Hostname, strings.Join(events, "; ")})

		}

		if outputFileName == "" {
			outputFileName = "workloader-ven-health-summary-report-" + time.Now().Format("20060102_150405") + ".csv"
		}
		utils.WriteOutput(csvOut, csvOut, outputFileName)
	}

	if includeEventList && len(allEvents) > 0 {
		csvOut := [][]string{{"event_type", "timestamp", "created_by_href", "created_by_details"}}
		for _, e := range allEvents {
			csvOut = append(csvOut, []string{e.EventType, time.Time.String(e.Timestamp), e.EventCreatedBy.Href, e.EventCreatedBy.Name})
		}
		if outputFileName == "" {
			outputFileName = "workloader-ven-health-event-list-" + time.Now().Format("20060102_150405") + ".csv"
		} else {
			outputFileName = "full-event-list-" + outputFileName
		}
		utils.WriteOutput(csvOut, csvOut, outputFileName)
	}

}
