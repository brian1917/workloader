package autodenyrules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
)

var AutoDenyRulesCmd = &cobra.Command{
	Use:   "auto-deny-rules",
	Short: "Run auto-deny-rules based on traffic queries",
	Long:  `Creates deny rules automatically for workloads with no risky services traffic.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunAutoDenyRules(verboseFlag, excludeBroadcastFlag, excludeMulticastFlag)
	},
}

// optional flags
var verboseFlag bool
var excludeBroadcastFlag bool
var excludeMulticastFlag bool

func init() {
	AutoDenyRulesCmd.Flags().BoolVar(&verboseFlag, "verbose", false, "enable verbose logging")
	AutoDenyRulesCmd.Flags().BoolVar(&excludeBroadcastFlag, "exclude-broadcast", false, "exclude broadcast traffic")
	AutoDenyRulesCmd.Flags().BoolVar(&excludeMulticastFlag, "exclude-multicast", false, "exclude multicast traffic")
}

var verbose bool
var pce illumioapi.PCE

type Label struct {
	Href  string `json:"href"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ServicePort struct {
	Port   int `json:"port"`
	Proto  int `json:"proto"`
	ToPort int `json:"to_port,omitempty"`
}

type Service struct {
	Href         string        `json:"href"`
	Name         string        `json:"name"`
	ServicePorts []ServicePort `json:"service_ports"`
}

type denyRuleInfo struct {
	env     illumioapi.Label
	service illumioapi.Service
	apps    []illumioapi.Label
}

var (
	totalDenyRules int64
	doneDenyRules  int64
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

func vlog(format string, v ...interface{}) {
	if verbose {
		log.Printf(format, v...)
	}
}

func printProgress(done, total int64) {
	percent := float64(done) / float64(total) * 100
	log.Printf("Progress: %.1f%% (%d/%d)", percent, done, total)
}

func apiRequestWithRetry(method, urlStr string, payload interface{}) ([]byte, error) {
	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		vlog("Payload: %s", string(body))
	}

	var lastErr error
	retries := 3
	for i := 0; i < retries; i++ {
		req, err := http.NewRequest(method, urlStr, bytes.NewBuffer(body))
		if err != nil {
			return nil, err
		}
		req.SetBasicAuth(pce.User, pce.Key)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			defer resp.Body.Close()
			data, _ := ioutil.ReadAll(resp.Body)
			vlog("Response Status: %s", resp.Status)
			vlog("RAW RESPONSE BODY: %s", string(data))

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return data, nil
			}
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
		}
		time.Sleep(time.Duration(1<<i)*time.Second + time.Duration(rand.Intn(500))*time.Millisecond)
	}
	return nil, fmt.Errorf("apiRequest failed after %d retries: %v", retries, lastErr)
}

func getEnvs() ([]illumioapi.Label, error) {
	// Load labels with key=env via illumioapi (handles async, populates pce.LabelsSlice)
	_, err := pce.GetLabels(map[string]string{"key": "env"})
	if err != nil {
		return nil, fmt.Errorf("getEnvs GetLabels: %w", err)
	}

	seen := make(map[string]struct{}, len(pce.LabelsSlice))
	out := make([]illumioapi.Label, 0, len(pce.LabelsSlice))

	for _, l := range pce.LabelsSlice {
		if l.Key != "env" || illumioapi.PtrToVal(l.Deleted) {
			continue
		}
		if l.Href == "" || l.Value == "" {
			continue
		}
		if _, ok := seen[l.Href]; ok {
			continue
		}
		seen[l.Href] = struct{}{}
		out = append(out, l)
	}

	// Sort by value
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Value) < strings.ToLower(out[j].Value)
	})

	if len(out) == 0 {
		return nil, fmt.Errorf("getEnvs: no env labels found")
	}
	return out, nil
}

func getRansomServices() ([]illumioapi.Service, error) {
	// Fetch draft services filtered by ransomware
	_, err := pce.GetServices(map[string]string{"is_ransomware": "true"}, "draft")
	if err != nil {
		return nil, fmt.Errorf("getRansomServices GetServices: %w", err)
	}

	// pce.ServicesSlice now contains the filtered services
	// Dedupe by href and skip deleted
	seen := make(map[string]struct{}, len(pce.ServicesSlice))
	out := make([]illumioapi.Service, 0, len(pce.ServicesSlice))
	for _, s := range pce.ServicesSlice {
		if s.Href == "" {
			continue
		}
		if _, ok := seen[s.Href]; ok {
			continue
		}
		seen[s.Href] = struct{}{}
		out = append(out, s)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("getRansomServices: no ransomware services found")
	}
	return out, nil
}

func getWorkloadsForEnv(pce illumioapi.PCE, env illumioapi.Label) ([]illumioapi.Label, error) {
	// Input validation
	if env.Href == "" {
		return nil, fmt.Errorf("environment label href cannot be empty")
	}

	vlog("Fetching workloads for env %s (href: %s)", env.Value, env.Href)

	// Create query parameters using the library's approach
	queryParameters := map[string]string{
		"managed":           "true",
		"online":            "true",
		"labels":            fmt.Sprintf("[[%q]]", env.Href),
		"enforcement_modes": "[\"idle\",\"selective\",\"visibility_only\"]",
	}

	// Use the library's GetWklds method
	api, err := pce.GetWklds(queryParameters)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workloads for env %s: %w", env.Value, err)
	}

	// Access workloads from pce.WorkloadsSlice
	workloads := pce.WorkloadsSlice

	vlog("API call details: %+v", api)

	// Extract unique app labels
	uniqueApps := make(map[string]illumioapi.Label)
	for _, workload := range workloads {
		// Dereference the pointer to get the slice
		if workload.Labels != nil {
			for _, label := range *workload.Labels {
				if label.Key == "app" && label.Href != "" {
					uniqueApps[label.Href] = label
				}
			}
		}
	}

	// Convert map to slice
	apps := make([]illumioapi.Label, 0, len(uniqueApps))
	for _, label := range uniqueApps {
		apps = append(apps, label)
	}

	vlog("Found %d unique apps for env %s", len(apps), env.Value)
	return apps, nil
}

func submitTrafficQuery(
	pce illumioapi.PCE,
	envHref, appHref string,
	service illumioapi.Service,
	excludeBroadcast, excludeMulticast bool,
) (bool, error) {
	now := time.Now().UTC()
	start24h := now.Add(-24 * time.Hour).Format(time.RFC3339)
	start89d := now.Add(-89 * 24 * time.Hour).Format(time.RFC3339)
	end := now.Format(time.RFC3339)

	buildPayload := func(start string) map[string]interface{} {
		ports := []map[string]interface{}{}
		for _, sp := range illumioapi.PtrToVal(service.ServicePorts) {
			m := map[string]interface{}{"proto": sp.Protocol}
			if port := illumioapi.PtrToVal(sp.Port); port != 0 {
				m["port"] = port
			}
			if sp.ToPort != 0 {
				m["to_port"] = sp.ToPort
			}
			ports = append(ports, m)
		}

		return map[string]interface{}{
			"query_name": fmt.Sprintf("Query Env: %s App: %s", envHref, appHref),
			"sources": map[string]interface{}{
				"include": []interface{}{[]interface{}{}},
				"exclude": []interface{}{},
			},
			"destinations": map[string]interface{}{
				"include": [][]map[string]map[string]string{
					{
						{"label": {"href": envHref}},
						{"label": {"href": appHref}},
					},
				},
				"exclude": buildDestExclusions(excludeBroadcast, excludeMulticast),
			},
			"services": map[string]interface{}{
				"include": ports,
				"exclude": []interface{}{},
			},
			"sources_destinations_query_op":        "and",
			"start_date":                           start,
			"end_date":                             end,
			"policy_decisions":                     []string{}, // REQUIRED
			"boundary_decisions":                   []string{}, // REQUIRED
			"exclude_workloads_from_ip_list_query": true,       // REQUIRED
			"max_results":                          1,
		}
	}

	// 24h query
	hasFlows, err := runSingleAsyncQuery(pce, buildPayload(start24h))
	if err != nil {
		return false, err
	}
	if hasFlows {
		return false, nil
	}

	// 89d query
	hasFlows, err = runSingleAsyncQuery(pce, buildPayload(start89d))
	if err != nil {
		return false, err
	}
	if hasFlows {
		return false, nil
	}

	return true, nil
}

func runSingleAsyncQuery(pce illumioapi.PCE, payload map[string]interface{}) (bool, error) {
	url := fmt.Sprintf(
		"https://%s:%d/api/v2/orgs/%d/traffic_flows/async_queries",
		pce.FQDN, pce.Port, pce.Org,
	)

	respBytes, err := apiRequestWithRetry("POST", url, payload)
	if err != nil {
		return false, err
	}

	var resp struct {
		Href string `json:"href"`
	}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return false, err
	}
	if resp.Href == "" {
		return false, fmt.Errorf("no href returned for async query")
	}

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return false, fmt.Errorf("traffic query timed out")
		case <-ticker.C:
			pollURL := fmt.Sprintf("https://%s:%d/api/v2%s",
				pce.FQDN, pce.Port, resp.Href)
			pollBytes, err := apiRequestWithRetry("GET", pollURL, nil)
			if err != nil {
				return false, err
			}

			var pollResp struct {
				Status     string  `json:"status"`
				FlowsCount float64 `json:"flows_count"`
			}
			if err := json.Unmarshal(pollBytes, &pollResp); err != nil {
				return false, err
			}

			if pollResp.Status == "completed" {
				return pollResp.FlowsCount > 0, nil
			}
		}
	}
}

// Helper function to build structured destination exclusions
func buildDestExclusionsStructured(excludeBroadcast, excludeMulticast bool) [][]illumioapi.ConsumerOrProvider {
	var exclusions [][]illumioapi.ConsumerOrProvider

	if excludeBroadcast {
		exclusions = append(exclusions, []illumioapi.ConsumerOrProvider{
			{IPList: &illumioapi.IPList{
				IPRanges: &[]illumioapi.IPRange{
					{FromIP: "255.255.255.255", ToIP: "255.255.255.255"},
				},
			}},
		})
	}

	if excludeMulticast {
		exclusions = append(exclusions, []illumioapi.ConsumerOrProvider{
			{IPList: &illumioapi.IPList{
				IPRanges: &[]illumioapi.IPRange{
					{FromIP: "224.0.0.0", ToIP: "239.255.255.255"},
				},
			}},
		})
	}

	return exclusions
}
func createRuleset(name string) (string, error) {
	enabled := true

	rs := illumioapi.RuleSet{
		Name:        name,
		Description: ptrString("Created by Auto Deny Rules script."),
		Enabled:     &enabled,
		// Use empty array instead of array with empty object
		Scopes: &[][]illumioapi.Scopes{
			{}, // This creates an empty array, not an array with empty object
		},
	}

	createdRS, _, err := pce.CreateRuleset(rs)
	if err != nil {
		return "", fmt.Errorf("failed to create ruleset: %w", err)
	}

	return createdRS.Href, nil
}

func ptrString(s string) *string {
	return &s
}

func createDenyRule(
	pce illumioapi.PCE,
	rulesetHref string,
	serviceHref string,
	apps []illumioapi.Label,
	env illumioapi.Label,
	ipListHref string,
) error {

	boolPtr := func(b bool) *bool { return &b }
	stringPtr := func(s string) *string { return &s }

	// Providers (env + apps)
	providers := []illumioapi.ConsumerOrProvider{
		{Label: &illumioapi.Label{Href: env.Href}},
	}
	for _, app := range apps {
		providers = append(providers, illumioapi.ConsumerOrProvider{
			Label: &illumioapi.Label{Href: app.Href},
		})
	}

	// Consumers (IP list)
	consumers := []illumioapi.ConsumerOrProvider{
		{IPList: &illumioapi.IPList{Href: ipListHref}},
	}

	// Ingress services
	ingressServices := []illumioapi.IngressServices{
		{Href: serviceHref},
	}

	// Build deny rule
	rule := illumioapi.Rule{
		RuleType:        "deny", // REQUIRED
		Providers:       &providers,
		Consumers:       &consumers,
		IngressServices: &ingressServices,
		Enabled:         boolPtr(true),
		Description:     stringPtr(""),
		// Do NOT set NetworkType on deny rules
	}

	_, api, err := pce.CreateRule(rulesetHref, rule)
	if err != nil {
		return fmt.Errorf("failed to create deny rule: %w", err)
	}

	vlog("Created deny rule successfully. API response: %s", api.RespBody)
	return nil
}

func getIPListHref(pce illumioapi.PCE, targetName string) (string, error) {
	// Use query parameters to filter by name
	queryParams := map[string]string{
		"name": targetName,
	}

	// Get IP lists with name filter - use "draft" for policy draft
	_, err := pce.GetIPLists(queryParams, "draft")
	if err != nil {
		return "", fmt.Errorf("failed to get IP lists: %w", err)
	}

	// Check if any IP lists were found
	if len(pce.IPListsSlice) == 0 {
		return "", fmt.Errorf("no IP list found with name %q", targetName)
	}

	// Find exact name match (in case API returns partial matches)
	for _, ipList := range pce.IPListsSlice {
		if ipList.Name == targetName {
			return ipList.Href, nil
		}
	}

	// If no exact match, return the first one
	return pce.IPListsSlice[0].Href, nil
}

func buildDestExclusions(broadcast, multicast bool) []interface{} {
	excl := make([]interface{}, 0)
	if broadcast {
		excl = append(excl, map[string]string{"transmission": "broadcast"})
	}
	if multicast {
		excl = append(excl, map[string]string{"transmission": "multicast"})
	}
	return excl
}

func logQueryProgress(env illumioapi.Label, app illumioapi.Label, svc illumioapi.Service, done, total int64) {
	percent := float64(done) / float64(total) * 100
	log.Printf("[Query] Env:%s  App:%s  Service:%s  →  Progress: %.1f%% (%d/%d)",
		env.Value, app.Value, svc.Name, percent, done, total)
}

func RunAutoDenyRules(verboseFlag bool, excludeBroadcast, excludeMulticast bool) error {
	verbose = verboseFlag

	// Initialize PCE from workloader config
	var err error
	pce, err = utils.GetTargetPCEV2(true)
	if err != nil {
		utils.LogError(err.Error())
		return err
	}

	envs, err := getEnvs()
	if err != nil {
		return fmt.Errorf("Failed to load environments: %v", err)
	}
	services, err := getRansomServices()
	if err != nil {
		return fmt.Errorf("Failed to load ransomware services: %v", err)
	}

	friendly := time.Now().Format("Jan 02, 2006 15:04:05")
	rulesetName := fmt.Sprintf("Auto Deny Rules - %s", friendly)
	rulesetHref, err := createRuleset(rulesetName)
	if err != nil {
		log.Fatalf("Failed to create rule set: %v", err)
	}
	log.Printf("Created Auto Deny Rules rule set %s", rulesetHref)

	type envInfo struct {
		env  illumioapi.Label
		apps []illumioapi.Label
	}
	var envInfos []envInfo
	for _, env := range envs {
		apps, err := getWorkloadsForEnv(pce, env)
		if err != nil {
			vlog("Failed to get workloads for env %s: %v", env.Value, err)
			continue
		}
		if len(apps) == 0 {
			continue
		}
		envInfos = append(envInfos, envInfo{env: env, apps: apps})
	}

	var totalQueries int64
	for _, ei := range envInfos {
		totalQueries += int64(len(services) * len(ei.apps))
	}
	if totalQueries == 0 {
		log.Println("No queries to run - exiting.")
		return nil
	}
	log.Printf("Total traffic queries to execute: %d", totalQueries)

	var wg sync.WaitGroup
	sem := make(chan struct{}, 2) // max 2 concurrent queries
	var denyRules []denyRuleInfo
	var denyRulesMu sync.Mutex

	targetIPListName := "Any (0.0.0.0/0 and ::/0)"
	ipListHref, err := getIPListHref(pce, targetIPListName)
	if err != nil {
		log.Fatalf("Failed to locate IP-list %q: %v", targetIPListName, err)
	}
	log.Printf("Using the Any IP-list href: %s", ipListHref)

	var doneQueries int64
	for _, ei := range envInfos {
		for _, service := range services {
			var appsNoTraffic []illumioapi.Label
			var appsMu sync.Mutex

			for _, app := range ei.apps {
				wg.Add(1)
				sem <- struct{}{}
				go func(a illumioapi.Label) {
					defer wg.Done()
					defer func() { <-sem }()

					ok, err := submitTrafficQuery(
						pce, // <-- your PCE object
						ei.env.Href,
						a.Href,
						service,
						excludeBroadcast,
						excludeMulticast,
					)
					if err != nil {
						log.Printf("[Query] Env:%s  App:%s  Service:%s  →  error: %v",
							ei.env.Value, a.Value, service.Name, err)
					} else if ok { // no traffic found
						appsMu.Lock()
						appsNoTraffic = append(appsNoTraffic, a)
						appsMu.Unlock()
					}

					// update and print query progress (always shown)
					atomic.AddInt64(&doneQueries, 1)
					logQueryProgress(ei.env, a, service, atomic.LoadInt64(&doneQueries), totalQueries)
				}(app)
			}

			// wait for all apps of this service to finish before moving on
			wg.Wait()

			if len(appsNoTraffic) > 0 {
				denyRulesMu.Lock()
				denyRules = append(denyRules, denyRuleInfo{
					env:     ei.env,
					service: service,
					apps:    appsNoTraffic,
				})
				denyRulesMu.Unlock()
			}
		}
	}

	// Create deny rules in the single rule-set - with progress tracking
	totalDenyRules = int64(len(denyRules))
	if totalDenyRules == 0 {
		log.Println("No deny rules needed - skipping rule creation.")
	} else {
		log.Printf("Creating %d deny rule(s)...", totalDenyRules)
		for _, dr := range denyRules {
			if err := createDenyRule(pce, rulesetHref, dr.service.Href, dr.apps, dr.env, ipListHref); err != nil {
				log.Printf("Failed to create deny rule for env %s service %s: %v",
					dr.env.Value, dr.service.Name, err)
			} else {
				// Combined log line
				atomic.AddInt64(&doneDenyRules, 1)
				percent := float64(atomic.LoadInt64(&doneDenyRules)) / float64(totalDenyRules) * 100
				log.Printf("Created deny rule for env %s service %s (apps: %d) – Progress: %.1f%% (%d/%d)",
					dr.env.Value, dr.service.Name, len(dr.apps),
					percent, atomic.LoadInt64(&doneDenyRules), totalDenyRules)
				// End combined line
			}
		}
	}

	// Optional clean-up: delete the rule-set if it stayed empty
	if len(denyRules) == 0 {
		log.Printf("No deny rules needed - you may delete the empty rule set %s", rulesetHref)

		// Use the illumioapi package's DeleteHref method
		api, err := pce.DeleteHref(rulesetHref)
		if err != nil {
			log.Printf("Failed to delete empty rule set: %v", err)
		} else {
			log.Printf("Deleted empty rule set %s (Status: %d)", rulesetHref, api.StatusCode)
		}
	}

	log.Println("All queries and deny rules completed.")
	return nil
}
