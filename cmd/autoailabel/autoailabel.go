package autoailabel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
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

// Compare mode + sampling
var compareExisting bool
var sampleSize int

func init() {
	AutoAILabelCmd.Flags().StringVar(&aiEndpoint, "ai-endpoint", "", "AI API endpoint URL")
	AutoAILabelCmd.Flags().BoolVar(&compareExisting, "compare-existing", false,
		"Compare existing app labels vs AI recommendation (no changes applied)")
	AutoAILabelCmd.Flags().IntVar(&sampleSize, "sample-size", 0,
		"Random sample size for compare mode (0 = all)")
}

// Cobra command for auto AI labeling
var AutoAILabelCmd = &cobra.Command{
	Use:   "auto-ai-label",
	Short: "Automatically label workloads via AI recommendations based on workload metadata.",
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

// levenshteinDistance computes the Levenshtein edit distance between two strings.
// This implementation is optimized to use O(min(len(a),len(b))) space.
func levenshteinDistance(a, b string) int {
	// Make sure a is the shorter string to minimize memory use
	if len(a) > len(b) {
		a, b = b, a
	}

	la := len(a)
	lb := len(b)

	// If one string is empty, distance is the other's length
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	// Convert to rune slices if you expect non-ASCII.
	// Since you're normalizing to [a-zA-Z0-9], byte-wise is fine.
	prev := make([]int, la+1)
	cur := make([]int, la+1)

	for i := 0; i <= la; i++ {
		prev[i] = i
	}

	for j := 1; j <= lb; j++ {
		cur[0] = j
		bj := b[j-1]
		for i := 1; i <= la; i++ {
			cost := 0
			if a[i-1] != bj {
				cost = 1
			}

			// deletion: prev[i] + 1
			// insertion: cur[i-1] + 1
			// substitution: prev[i-1] + cost
			del := prev[i] + 1
			ins := cur[i-1] + 1
			sub := prev[i-1] + cost

			cur[i] = del
			if ins < cur[i] {
				cur[i] = ins
			}
			if sub < cur[i] {
				cur[i] = sub
			}
		}
		prev, cur = cur, prev
	}

	return prev[la]
}

// levenshteinSimilarity returns a value in [0,1], where 1 means exact match.
func levenshteinSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen == 0 {
		return 1.0
	}
	dist := levenshteinDistance(a, b)
	return 1.0 - (float64(dist) / float64(maxLen))
}

// normalizeForCompare keeps your original normalization but lowercases for comparisons.
func normalizeForCompare(s string) string {
	return strings.ToLower(normalizeLabel(s))
}

// labelsMatchCompareMode uses exact/contains/levenshtein similarity to decide match.
// No prefix stripping is performed.
func labelsMatchCompareMode(existing, ai string, threshold float64) (bool, string, float64) {
	e := normalizeForCompare(existing)
	a := normalizeForCompare(ai)

	if e == "" || a == "" {
		return false, "empty", 0
	}

	if e == a {
		return true, "exact", 1.0
	}

	// Contains match catches "AI-IllumioCore" vs "Illumio Core" after normalization
	if strings.Contains(e, a) || strings.Contains(a, e) {
		return true, "contains", 1.0
	}

	sim := levenshteinSimilarity(e, a)
	if sim >= threshold {
		return true, "levenshtein", sim
	}

	return false, "different", sim
}

func sampleWorkloads(in []illumioapi.Workload, n int) []illumioapi.Workload {
	if n <= 0 || n >= len(in) {
		return in
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	tmp := make([]illumioapi.Workload, len(in))
	copy(tmp, in)

	r.Shuffle(len(tmp), func(i, j int) { tmp[i], tmp[j] = tmp[j], tmp[i] })
	return tmp[:n]
}

func getExistingAppLabelFromWorkload(w *illumioapi.Workload) string {
	if w.Labels == nil {
		return ""
	}
	for _, lbl := range *w.Labels {
		if lbl.Key == "app" {
			return lbl.Value
		}
	}
	return ""
}

// runAutoAILabel executes the main workflow
func runAutoAILabel() {
	pce, err := utils.GetTargetPCEV2(true)
	if err != nil {
		log.Fatalf("Failed to load PCE: %v", err)
	}

	// Compare mode: no apply/create, just comparisons
	if compareExisting {
		if aiEndpoint == "" {
			log.Fatalf("--compare-existing requires --ai-endpoint")
		}
		runCompareExisting(pce)
		log.Println("auto-ai-label compare-existing completed")
		return
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

		topService, conn := findTopMatchingService(flows, details.Services.OpenServicePorts)
		if topService == nil {
			fmt.Println("No matching open service found for traffic flows")
			continue
		}

		fmt.Printf("Top matching service: connections=%d port=%d proto=%d process=%s package=%s win_service_name=%s\n",
			conn,
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

func getWorkloadsWithAppLabels(pce illumioapi.PCE) []illumioapi.Workload {
	params := map[string]string{
		"managed": "true",
		"online":  "true",
		// Require app label exists; no enforcement_modes filter in compare mode
		"labels": fmt.Sprintf(`[["/orgs/%d/labels?key=app&exists=true"]]`, pce.Org),
	}

	_, err := pce.GetWklds(params)
	if err != nil {
		log.Fatalf("Error fetching workloads: %v", err)
	}

	wlds := pce.WorkloadsSlice
	fmt.Printf("Found %d managed+online workloads WITH app labels\n", len(wlds))
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
func findTopMatchingService(flows []FlowSummary, services []OpenServicePort) (*OpenServicePort, int) {
	for _, flow := range flows {
		for i := range services {
			if services[i].Port == flow.Port && services[i].Protocol == flow.Proto {
				return &services[i], flow.NumConnections
			}
		}
	}
	return nil, 0
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

func runCompareExisting(pce illumioapi.PCE) {
	workloads := getWorkloadsWithAppLabels(pce)
	workloads = sampleWorkloads(workloads, sampleSize)

	fmt.Printf("Compare mode: evaluating %d workloads (sample-size=%d)\n", len(workloads), sampleSize)

	// Stats
	total := 0
	match := 0
	mismatch := 0
	noService := 0
	aiFail := 0

	for _, w := range workloads {
		total++

		hostname := "<no hostname>"
		if w.Hostname != nil {
			hostname = *w.Hostname
		}

		existingApp := getExistingAppLabelFromWorkload(&w)
		existingNorm := normalizeForCompare(existingApp)

		fmt.Printf("\n[COMPARE] %s %s\n", hostname, w.Href)
		fmt.Printf("  Existing app label: %q (norm=%q)\n", existingApp, existingNorm)

		queryHref := submitTrafficQueryForDestination(pce, w.Href)
		flows := downloadTrafficFlowsJSON(pce, queryHref)

		details, err := getWorkloadDetails(pce, w.Href)
		if err != nil {
			log.Printf("  ERROR workload details: %v\n", err)
			continue
		}

		topService, conn := findTopMatchingService(flows, details.Services.OpenServicePorts)
		if topService == nil {
			noService++
			fmt.Println("  No matching open service found for traffic flows -> SKIP AI")
			continue
		}

		fmt.Printf("  Matched service: connections=%d port=%d proto=%d process=%q package=%q win_service=%q\n",
			conn, topService.Port, topService.Protocol, topService.ProcessName, topService.Package, topService.WinServiceName)

		aiLabel, err := sendToAIEndpoint(aiEndpoint, w.Href, topService)
		if err != nil {
			aiFail++
			fmt.Printf("  AI ERROR: %v\n", err)
			continue
		}

		aiNorm := normalizeForCompare(aiLabel)
		fmt.Printf("  AI label: %q (norm=%q)\n", aiLabel, aiNorm)

		matchOK, reason, sim := labelsMatchCompareMode(existingApp, aiLabel, 0.85)

		if matchOK {
			match++
			if reason == "levenshtein" {
				fmt.Printf("  RESULT: MATCH (%s %.2f)\n", reason, sim)
			} else {
				fmt.Printf("  RESULT: MATCH (%s)\n", reason)
			}
		} else {
			mismatch++
			fmt.Printf("  RESULT: MISMATCH (%s %.2f)\n", reason, sim)
		}

	}

	fmt.Printf("\n===== COMPARE SUMMARY =====\n")
	fmt.Printf("Total evaluated:     %d\n", total)
	fmt.Printf("Matches:             %d\n", match)
	fmt.Printf("Mismatches:          %d\n", mismatch)
	fmt.Printf("No matching service: %d\n", noService)
	fmt.Printf("AI failures:         %d\n", aiFail)
}
