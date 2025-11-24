package autodenyrules

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
)

// denyRuleInfo holds information needed to create a deny rule
type denyRuleInfo struct {
	env     illumioapi.Label   // The environment label
	service illumioapi.Service // The service to deny
	apps    []illumioapi.Label // The applications with no traffic
}

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

var (
	verbose        bool
	pce            illumioapi.PCE
	totalDenyRules int64
	doneDenyRules  int64
)

// Helper functions
func vlog(format string, v ...interface{}) {
	if verbose {
		log.Printf(format, v...)
	}
}

func logQueryProgress(env illumioapi.Label, app illumioapi.Label, svc illumioapi.Service, done, total int64) {
	percent := float64(done) / float64(total) * 100
	log.Printf("[Query] Env:%s  App:%s  Service:%s  →  Progress: %.1f%% (%d/%d)",
		env.Value, app.Value, svc.Name, percent, done, total)
}

func ptrString(s string) *string { return &s }

// Fetch environments
func getEnvs() ([]illumioapi.Label, error) {
	_, err := pce.GetLabels(map[string]string{"key": "env"})
	if err != nil {
		return nil, fmt.Errorf("getEnvs GetLabels: %w", err)
	}

	seen := make(map[string]struct{}, len(pce.LabelsSlice))
	out := make([]illumioapi.Label, 0, len(pce.LabelsSlice))
	for _, l := range pce.LabelsSlice {
		if l.Key != "env" || illumioapi.PtrToVal(l.Deleted) || l.Href == "" || l.Value == "" {
			continue
		}
		if _, ok := seen[l.Href]; ok {
			continue
		}
		seen[l.Href] = struct{}{}
		out = append(out, l)
	}

	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Value) < strings.ToLower(out[j].Value)
	})

	if len(out) == 0 {
		return nil, fmt.Errorf("getEnvs: no env labels found")
	}
	return out, nil
}

// Fetch ransomware services
func getRansomServices() ([]illumioapi.Service, error) {
	_, err := pce.GetServices(map[string]string{"is_ransomware": "true"}, "draft")
	if err != nil {
		return nil, fmt.Errorf("getRansomServices GetServices: %w", err)
	}

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

// Fetch workloads and extract unique apps
func getWorkloadsForEnv(pce illumioapi.PCE, env illumioapi.Label) ([]illumioapi.Label, error) {
	if env.Href == "" {
		return nil, fmt.Errorf("environment label href cannot be empty")
	}

	vlog("Fetching workloads for env %s (href: %s)", env.Value, env.Href)

	queryParameters := map[string]string{
		"managed":           "true",
		"online":            "true",
		"labels":            fmt.Sprintf("[[%q]]", env.Href),
		"enforcement_modes": "[\"idle\",\"selective\",\"visibility_only\"]",
	}

	_, err := pce.GetWklds(queryParameters)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch workloads for env %s: %w", env.Value, err)
	}

	uniqueApps := make(map[string]illumioapi.Label)
	for _, workload := range pce.WorkloadsSlice {
		if workload.Labels != nil {
			for _, label := range *workload.Labels {
				if label.Key == "app" && label.Href != "" {
					uniqueApps[label.Href] = label
				}
			}
		}
	}

	apps := make([]illumioapi.Label, 0, len(uniqueApps))
	for _, label := range uniqueApps {
		apps = append(apps, label)
	}

	vlog("Found %d unique apps for env %s", len(apps), env.Value)
	return apps, nil
}

// Submit traffic query
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
			"policy_decisions":                     []string{},
			"boundary_decisions":                   []string{},
			"exclude_workloads_from_ip_list_query": true,
			"max_results":                          1,
		}
	}

	// Check for flows in the last 24 hours first
	hasFlows, err := runSingleAsyncQuery(pce, buildPayload(start24h))
	if err != nil {
		return false, fmt.Errorf("failed to query 24h traffic: %w", err)
	}
	if hasFlows {
		return false, nil // Found flows, so this is not an unused service
	}

	// If no flows in 24h, check the last 89 days
	hasFlows, err = runSingleAsyncQuery(pce, buildPayload(start89d))
	if err != nil {
		return false, fmt.Errorf("failed to query 89d traffic: %w", err)
	}
	if hasFlows {
		return false, nil // Found flows, so this is not an unused service
	}

	// No flows found in either time period
	return true, nil
}

func runSingleAsyncQuery(pce illumioapi.PCE, payload map[string]interface{}) (bool, error) {
	var resp struct {
		Href string `json:"href"`
	}
	_, err := pce.Post("traffic_flows/async_queries", payload, &resp)
	if err != nil {
		return false, fmt.Errorf("failed to create async query: %w", err)
	}

	if resp.Href == "" {
		return false, fmt.Errorf("no href returned for async query")
	}

	// Poll the async query status using the package's GetHref method
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return false, fmt.Errorf("traffic query timed out")
		case <-ticker.C:
			var pollResp struct {
				Status     string  `json:"status"`
				FlowsCount float64 `json:"flows_count"`
			}

			// Use GetHref to poll the async query status
			_, err := pce.GetHref(resp.Href, &pollResp)
			if err != nil {
				return false, fmt.Errorf("failed to poll async query: %w", err)
			}

			if pollResp.Status == "completed" {
				return pollResp.FlowsCount > 0, nil
			}

			// Optional: Handle failed status
			if pollResp.Status == "failed" {
				return false, fmt.Errorf("async query failed")
			}
		}
	}
}

func buildDestExclusions(broadcast, multicast bool) []interface{} {
	excl := []interface{}{}
	if broadcast {
		excl = append(excl, map[string]string{"transmission": "broadcast"})
	}
	if multicast {
		excl = append(excl, map[string]string{"transmission": "multicast"})
	}
	return excl
}

func createRuleset(name string) (string, error) {
	enabled := true
	rs := illumioapi.RuleSet{
		Name:        name,
		Description: ptrString("Created by Auto Deny Rules script."),
		Enabled:     &enabled,
		Scopes:      &[][]illumioapi.Scopes{{}}, // global scope
	}

	createdRS, _, err := pce.CreateRuleset(rs)
	if err != nil {
		return "", fmt.Errorf("failed to create ruleset: %w", err)
	}
	return createdRS.Href, nil
}

func createDenyRule(pce illumioapi.PCE, rulesetHref, serviceHref string, apps []illumioapi.Label, env illumioapi.Label, ipListHref string) error {
	boolPtr := func(b bool) *bool { return &b }
	stringPtr := func(s string) *string { return &s }

	providers := []illumioapi.ConsumerOrProvider{{Label: &illumioapi.Label{Href: env.Href}}}
	for _, app := range apps {
		providers = append(providers, illumioapi.ConsumerOrProvider{Label: &illumioapi.Label{Href: app.Href}})
	}
	consumers := []illumioapi.ConsumerOrProvider{{IPList: &illumioapi.IPList{Href: ipListHref}}}
	ingressServices := []illumioapi.IngressServices{{Href: serviceHref}}

	rule := illumioapi.Rule{
		RuleType:        "deny",
		Providers:       &providers,
		Consumers:       &consumers,
		IngressServices: &ingressServices,
		Enabled:         boolPtr(true),
		Description:     stringPtr(""),
	}

	_, api, err := pce.CreateRule(rulesetHref, rule)
	if err != nil {
		return fmt.Errorf("failed to create deny rule: %w", err)
	}
	vlog("Created deny rule successfully. API response: %s", api.RespBody)
	return nil
}

func getIPListHref(pce illumioapi.PCE, targetName string) (string, error) {
	queryParams := map[string]string{"name": targetName}
	_, err := pce.GetIPLists(queryParams, "draft")
	if err != nil {
		return "", fmt.Errorf("failed to get IP lists: %w", err)
	}
	if len(pce.IPListsSlice) == 0 {
		return "", fmt.Errorf("no IP list found with name %q", targetName)
	}
	for _, ipList := range pce.IPListsSlice {
		if ipList.Name == targetName {
			return ipList.Href, nil
		}
	}
	return pce.IPListsSlice[0].Href, nil
}

// Main runner
func RunAutoDenyRules(verboseFlag bool, excludeBroadcast, excludeMulticast bool) error {
	verbose = verboseFlag

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

					ok, err := submitTrafficQuery(pce, ei.env.Href, a.Href, service, excludeBroadcast, excludeMulticast)
					if err != nil {
						log.Printf("[Query] Env:%s  App:%s  Service:%s  →  error: %v", ei.env.Value, a.Value, service.Name, err)
					} else if ok {
						appsMu.Lock()
						appsNoTraffic = append(appsNoTraffic, a)
						appsMu.Unlock()
					}

					atomic.AddInt64(&doneQueries, 1)
					logQueryProgress(ei.env, a, service, atomic.LoadInt64(&doneQueries), totalQueries)
				}(app)
			}

			wg.Wait()

			if len(appsNoTraffic) > 0 {
				denyRulesMu.Lock()
				denyRules = append(denyRules, denyRuleInfo{env: ei.env, service: service, apps: appsNoTraffic})
				denyRulesMu.Unlock()
			}
		}
	}

	// Create deny rules
	totalDenyRules = int64(len(denyRules))
	if totalDenyRules == 0 {
		log.Println("No deny rules needed - skipping rule creation.")
	} else {
		log.Printf("Creating %d deny rule(s)...", totalDenyRules)
		for _, dr := range denyRules {
			if err := createDenyRule(pce, rulesetHref, dr.service.Href, dr.apps, dr.env, ipListHref); err != nil {
				log.Printf("Failed to create deny rule for env %s service %s: %v", dr.env.Value, dr.service.Name, err)
			} else {
				atomic.AddInt64(&doneDenyRules, 1)
				percent := float64(atomic.LoadInt64(&doneDenyRules)) / float64(totalDenyRules) * 100
				log.Printf("Created deny rule for env %s service %s (apps: %d) – Progress: %.1f%% (%d/%d)",
					dr.env.Value, dr.service.Name, len(dr.apps), percent, atomic.LoadInt64(&doneDenyRules), totalDenyRules)
			}
		}
	}

	if len(denyRules) == 0 {
		log.Printf("No deny rules needed - you may delete the empty rule set %s", rulesetHref)
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
