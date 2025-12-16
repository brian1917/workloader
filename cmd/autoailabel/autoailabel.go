package autoailabel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// AI endpoint flag
var aiEndpoint string

func init() {
	// Initialize AI endpoint flag for Cobra CLI command
	AutoAILabelCmd.Flags().StringVar(&aiEndpoint, "ai-endpoint", "", "AI API endpoint URL")
}

// Cobra command for auto AI labeling
var AutoAILabelCmd = &cobra.Command{
	Use:   "auto-ai-label",
	Short: "List workloads, run traffic queries, and optionally label workloads via AI",
	Run: func(cmd *cobra.Command, args []string) {
		runAutoAILabel()
	},
}

// FlowSummary represents aggregated traffic flows
type FlowSummary struct {
	NumConnections int
	Port           int
	Proto          int
}

// WorkloadDetails represents workload info including open services
type WorkloadDetails struct {
	Href     string `json:"href"`
	Hostname string `json:"hostname"`
	Services struct {
		OpenServicePorts []OpenServicePort `json:"open_service_ports"`
	} `json:"services"`
}

// OpenServicePort represents a workload's open service port
type OpenServicePort struct {
	Port           int    `json:"port"`
	Protocol       int    `json:"protocol"`
	ProcessName    string `json:"process_name"`
	Package        string `json:"package"`
	WinServiceName string `json:"win_service_name"`
}

// AIPayload defines the request sent to AI endpoint
type AIPayload struct {
	Workload  WorkloadRef `json:"workload"`
	ExtraInfo AIExtraInfo `json:"extraInfo"`
}

// WorkloadRef wraps workload href for AI payload
type WorkloadRef struct {
	Href string `json:"href"`
}

// AIExtraInfo wraps service/process info for AI payload
type AIExtraInfo struct {
	ProcessName string `json:"process_name,omitempty"`
	Package     string `json:"package,omitempty"`
	WinService  string `json:"win_service_name,omitempty"`
	Port        int    `json:"port,omitempty"`
	Protocol    int    `json:"protocol,omitempty"`
}

// runAutoAILabel executes the main workflow
func runAutoAILabel() {
	pce, err := utils.GetTargetPCEV2(true)
	if err != nil {
		log.Fatalf("Failed to load PCE: %v", err)
	}

	workloads := getWorkloads(pce)

	for _, w := range workloads {
		fmt.Printf("\nRunning traffic query for destination workload: %s\n", w.Href)

		queryHref := submitTrafficQueryForDestination(pce, w.Href)
		flows := downloadTrafficFlowsJSON(pce, queryHref)

		details, err := getWorkloadDetails(pce, w.Href)
		if err != nil {
			log.Fatalf("Failed to fetch workload details: %v", err)
		}

		topService := findTopMatchingService(flows, details.Services.OpenServicePorts)
		if topService == nil {
			fmt.Println("No matching open service found for traffic flows")
			continue
		}

		fmt.Printf("Top matching service: connections=%d port=%d proto=%d process=%s package=%s win_service_name=%s\n",
			flows[0].NumConnections,
			topService.Port,
			topService.Protocol,
			topService.ProcessName,
			topService.Package,
			topService.WinServiceName,
		)

		// Send to AI endpoint if configured
		if aiEndpoint != "" {
			label, err := sendToAIEndpoint(aiEndpoint, w.Href, topService)
			if err != nil {
				log.Printf("AI labeling failed: %v", err)
				continue
			}
			handleAILabel(pce, w.Href, label)
		}
	}

	log.Println("auto-ai-label completed")
}

// getWorkloads fetches online/managed workloads without app labels
func getWorkloads(pce illumioapi.PCE) []illumioapi.Workload {
	params := map[string]string{
		"managed":           "true",
		"online":            "true",
		"enforcement_modes": `["idle","visibility_only"]`,
		"labels":            fmt.Sprintf(`[["/orgs/%d/labels?key=app&exists=false"]]`, pce.Org),
	}

	_, err := pce.GetWklds(params)
	if err != nil {
		log.Fatalf("Error fetching workloads: %v", err)
	}

	wlds := pce.WorkloadsSlice
	fmt.Printf("Found %d filtered workloads:\n", len(wlds))
	for _, w := range wlds {
		hostname := "<no hostname>"
		if w.Hostname != nil {
			hostname = *w.Hostname
		}
		fmt.Printf(" - %s (%s)\n", hostname, w.Href)
	}
	return wlds
}

// submitTrafficQueryForDestination submits async traffic query for a destination workload
func submitTrafficQueryForDestination(pce illumioapi.PCE, workloadHref string) string {
	now := time.Now().UTC()
	payload := map[string]interface{}{
		"sources": map[string]interface{}{
			"include": [][]interface{}{{}},
			"exclude": []interface{}{},
		},
		"destinations": map[string]interface{}{
			"include": [][]interface{}{{map[string]interface{}{"workload": map[string]string{"href": workloadHref}}}},
			"exclude": []interface{}{
				map[string]string{"transmission": "broadcast"},
				map[string]string{"transmission": "multicast"},
			},
		},
		"services": map[string]interface{}{
			"include": []interface{}{},
			"exclude": []interface{}{},
		},
		"sources_destinations_query_op":        "and",
		"start_date":                           now.Add(-7 * 24 * time.Hour).Format(time.RFC3339),
		"end_date":                             now.Format(time.RFC3339),
		"policy_decisions":                     []string{},
		"boundary_decisions":                   []string{},
		"query_name":                           fmt.Sprintf("DEST_WKLD_%s", workloadHref),
		"exclude_workloads_from_ip_list_query": false,
		"max_results":                          100000,
	}

	var resp struct{ Href string }
	_, err := pce.Post("traffic_flows/async_queries", payload, &resp)
	if err != nil {
		log.Fatalf("Traffic query failed for %s: %v", workloadHref, err)
	}
	if resp.Href == "" {
		log.Fatalf("Traffic query returned no href for %s", workloadHref)
	}

	fmt.Printf("Async query submitted. Href: %s\n", resp.Href)

	// Poll until query completes or times out
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-timeout:
			log.Fatalf("Traffic query timed out for %s", workloadHref)
		case <-ticker.C:
			var status struct {
				Status     string  `json:"status"`
				FlowsCount float64 `json:"flows_count"`
			}
			_, err := pce.GetHref(resp.Href, &status)
			if err != nil {
				log.Fatalf("Failed to poll async query: %v", err)
			}
			if status.Status == "completed" {
				fmt.Printf("Query completed for %s: %.0f flows\n", workloadHref, status.FlowsCount)
				return resp.Href
			}
			if status.Status == "failed" {
				log.Fatalf("Async query failed for %s", workloadHref)
			}
		}
	}
}

// downloadTrafficFlowsJSON downloads async query results as FlowSummary
func downloadTrafficFlowsJSON(pce illumioapi.PCE, queryHref string) []FlowSummary {
	if queryHref == "" {
		log.Fatalf("downloadTrafficFlowsJSON: empty queryHref")
	}

	// Normalize endpoint
	endpoint := strings.TrimPrefix(queryHref, "/")
	if strings.HasPrefix(endpoint, fmt.Sprintf("orgs/%d/", pce.Org)) {
		endpoint = endpoint[len(fmt.Sprintf("orgs/%d/", pce.Org)):]
	}
	endpoint += "/download"

	fmt.Printf("Downloading JSON flows from: %s\n", endpoint)

	var flows interface{}
	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	_, err := pce.GetCollectionHeaders(endpoint, false, nil, headers, &flows)
	if err != nil {
		log.Fatalf("Error downloading flows as JSON: %v", err)
	}

	rawFlows, ok := flows.([]interface{})
	if !ok {
		log.Fatalf("Expected flows to be []interface{}, got %T", flows)
	}

	// Convert raw flows to FlowSummary
	summaries := make([]FlowSummary, 0, len(rawFlows))
	for _, f := range rawFlows {
		flowMap, ok := f.(map[string]interface{})
		if !ok {
			continue
		}

		numConn, _ := flowMap["num_connections"].(float64)
		serviceMap, ok := flowMap["service"].(map[string]interface{})
		if !ok {
			continue
		}
		port, _ := serviceMap["port"].(float64)
		proto, _ := serviceMap["proto"].(float64)

		summaries = append(summaries, FlowSummary{
			NumConnections: int(numConn),
			Port:           int(port),
			Proto:          int(proto),
		})
	}

	// Sort by descending connection count
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].NumConnections > summaries[j].NumConnections
	})

	return summaries
}

// getWorkloadDetails fetches detailed info about a workload
func getWorkloadDetails(pce illumioapi.PCE, workloadHref string) (*WorkloadDetails, error) {
	var details WorkloadDetails
	_, err := pce.GetHref(workloadHref, &details)
	if err != nil {
		return nil, err
	}
	return &details, nil
}

// findTopMatchingService finds the first service that matches a traffic flow
func findTopMatchingService(flows []FlowSummary, services []OpenServicePort) *OpenServicePort {
	for _, flow := range flows {
		for _, svc := range services {
			if svc.Port == flow.Port && svc.Protocol == flow.Proto {
				return &svc
			}
		}
	}
	return nil
}

// sendToAIEndpoint sends service/workload info to AI and returns the recommended label
func sendToAIEndpoint(apiURL, workloadHref string, svc *OpenServicePort) (string, error) {
	payload := AIPayload{
		Workload: WorkloadRef{Href: workloadHref},
		ExtraInfo: AIExtraInfo{
			ProcessName: svc.ProcessName,
			Package:     svc.Package,
			WinService:  svc.WinServiceName,
			Port:        svc.Port,
			Protocol:    svc.Protocol,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal AI payload: %w", err)
	}

	log.Println("AI payload preview:", string(body))
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 300 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("POST AI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("AI API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var respJSON map[string]interface{}
	if err := json.Unmarshal(respBody, &respJSON); err != nil {
		return "", fmt.Errorf("invalid AI response JSON: %w", err)
	}

	label, _ := respJSON["label"].(string)
	return label, nil
}

// normalizeLabel removes non-alphanumeric characters
func normalizeLabel(label string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	return re.ReplaceAllString(label, "")
}

// labelExists checks if a label with the given value already exists
func labelExists(pce illumioapi.PCE, labelValue string) (bool, *illumioapi.Label) {
	var existing []illumioapi.Label
	_, err := pce.GetCollection("labels", false, map[string]string{"value": labelValue}, &existing)
	if err != nil {
		log.Printf("Error fetching labels: %v", err)
		return false, nil
	}
	for _, lbl := range existing {
		if lbl.Value == labelValue {
			return true, &lbl
		}
	}
	return false, nil
}

// findSimilarLabel performs a fuzzy search for similar labels
func findSimilarLabel(pce illumioapi.PCE, labelValue string) *illumioapi.Label {
	var all []illumioapi.Label
	_, err := pce.GetCollection("labels", false, nil, &all)
	if err != nil {
		log.Printf("Error fetching labels: %v", err)
		return nil
	}

	lower := strings.ToLower(labelValue)
	for _, lbl := range all {
		if strings.Contains(strings.ToLower(lbl.Value), lower) {
			return &lbl
		}
	}
	return nil
}

// createLabel creates a new label with "AI-" prefix
func createLabel(pce illumioapi.PCE, labelValue string) (*illumioapi.Label, error) {
	payload := map[string]interface{}{
		"key":   "app",
		"value": "AI-" + labelValue,
	}
	var newLabel illumioapi.Label
	_, err := pce.Post("labels", payload, &newLabel)
	if err != nil {
		return nil, fmt.Errorf("failed to create label: %w", err)
	}
	return &newLabel, nil
}

// applyLabelToWorkload adds a label to a workload and updates it
func applyLabelToWorkload(pce illumioapi.PCE, workloadHref, newLabelHref string) error {
	var wl illumioapi.Workload
	_, err := pce.GetHref(workloadHref, &wl)
	if err != nil {
		return fmt.Errorf("failed to fetch workload: %w", err)
	}

	if wl.Labels == nil {
		wl.Labels = &[]illumioapi.Label{}
	}

	// Add label if not already present
	exists := false
	for _, lbl := range *wl.Labels {
		if lbl.Href == newLabelHref {
			exists = true
			break
		}
	}
	if !exists {
		*wl.Labels = append(*wl.Labels, illumioapi.Label{Href: newLabelHref})
	}

	// Update workload
	_, err = pce.UpdateWkld(wl)
	if err != nil {
		return fmt.Errorf("failed to update workload: %w", err)
	}
	return nil
}

// handleAILabel manages the AI labeling workflow: normalize → check → fuzzy → create → apply
func handleAILabel(pce illumioapi.PCE, workloadHref, rawLabel string) {
	normalized := normalizeLabel(rawLabel)
	if normalized == "" {
		log.Println("AI returned empty label after normalization")
		return
	}

	var labelToApply *illumioapi.Label

	// Check for exact match
	if exists, lbl := labelExists(pce, normalized); exists {
		labelToApply = lbl
		log.Printf("Applying existing label: %s", lbl.Value)
	} else if similar := findSimilarLabel(pce, normalized); similar != nil {
		labelToApply = similar
		log.Printf("Applying similar label: %s", similar.Value)
	} else {
		newLbl, err := createLabel(pce, normalized)
		if err != nil {
			log.Printf("Failed to create label: %v", err)
			return
		}
		labelToApply = newLbl
		log.Printf("Created new label: %s", newLbl.Value)
	}

	if labelToApply != nil {
		if err := applyLabelToWorkload(pce, workloadHref, labelToApply.Href); err != nil {
			log.Printf("Failed to apply label to workload: %v", err)
		} else {
			log.Printf("Label applied successfully to workload %s: %s", workloadHref, labelToApply.Value)
		}
	}
}
