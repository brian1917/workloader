package autodenyrules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
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
	env     Label
	service Service
	apps    []Label
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

func getEnvs() ([]Label, error) {
	urlStr := fmt.Sprintf("https://%s:%d/api/v2/orgs/%d/labels?key=env", pce.FQDN, pce.Port, pce.Org)
	data, err := apiRequestWithRetry("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("getEnvs: %w", err)
	}
	var labels []Label
	if err := json.Unmarshal(data, &labels); err != nil {
		return nil, fmt.Errorf("getEnvs unmarshal: %w", err)
	}
	return labels, nil
}

func getRansomServices() ([]Service, error) {
	urlStr := fmt.Sprintf("https://%s:%d/api/v2/orgs/%d/sec_policy/draft/services?is_ransomware=true", pce.FQDN, pce.Port, pce.Org)
	data, err := apiRequestWithRetry("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("getRansomServices: %w", err)
	}
	var services []Service
	if err := json.Unmarshal(data, &services); err != nil {
		return nil, fmt.Errorf("getRansomServices unmarshal: %w", err)
	}
	return services, nil
}

func getWorkloadsForEnv(env Label) ([]Label, error) {
	urlStr := fmt.Sprintf(
		"https://%s:%d/api/v2/orgs/%d/workloads?managed=true&online=true&labels=[[\"%s\"]]&enforcement_modes=[\"idle\",\"selective\",\"visibility_only\"]",
		pce.FQDN, pce.Port, pce.Org, env.Href,
	)
	vlog("Fetching workloads for env %s", env.Value)

	data, err := apiRequestWithRetry("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("getWorkloadsForEnv %s: %w", env.Value, err)
	}

	var workloads []struct {
		Labels []Label `json:"labels"`
	}
	if err := json.Unmarshal(data, &workloads); err != nil {
		return nil, fmt.Errorf("getWorkloadsForEnv unmarshal: %w", err)
	}

	uniqueApps := make(map[string]Label)
	for _, w := range workloads {
		for _, l := range w.Labels {
			if l.Key == "app" {
				uniqueApps[l.Href] = l
			}
		}
	}
	apps := make([]Label, 0, len(uniqueApps))
	for _, l := range uniqueApps {
		apps = append(apps, l)
	}
	return apps, nil
}

func submitTrafficQuery(
	envHref, appHref string,
	service Service,
	excludeBroadcast, excludeMulticast bool,
) (bool, error) {
	now := time.Now().UTC()
	start24h := now.Add(-24 * time.Hour).Format(time.RFC3339)
	start89d := now.Add(-89 * 24 * time.Hour).Format(time.RFC3339)
	end := now.Format(time.RFC3339)

	var ports []map[string]interface{}
	for _, sp := range service.ServicePorts {
		p := map[string]interface{}{
			"port":  sp.Port,
			"proto": sp.Proto,
		}
		if sp.ToPort != 0 {
			p["to_port"] = sp.ToPort
		}
		ports = append(ports, p)
	}

	payload := func(start string) map[string]interface{} {
		return map[string]interface{}{
			"sources": map[string]interface{}{
				"include": []interface{}{[]interface{}{}},
				"exclude": []interface{}{},
			},
			"destinations": map[string]interface{}{
				"include": [][]map[string]map[string]string{
					{{"label": {"href": envHref}}, {"label": {"href": appHref}}},
				},
				"exclude": buildDestExclusions(excludeBroadcast, excludeMulticast),
			},
			"services": map[string]interface{}{
				"include": ports,
				"exclude": []interface{}{},
			},
			"sources_destinations_query_op": "and",
			"start_date":                    start,
			"end_date":                      end,
			"policy_decisions":              []string{},
			"boundary_decisions":            []string{},
			"query_name": fmt.Sprintf(
				"Query Env: %s App: %s", envHref, appHref,
			),
			"exclude_workloads_from_ip_list_query": true,
			"max_results":                          1,
		}
	}

	url := fmt.Sprintf("https://%s:%d/api/v2/orgs/%d/traffic_flows/async_queries", pce.FQDN, pce.Port, pce.Org)

	// 24-hour query
	if hasFlows, err := runSingleAsyncQuery(url, payload(start24h)); err != nil {
		return false, err
	} else if hasFlows {
		return false, nil
	}

	// 89-day query - only reached when 24h had no traffic
	if hasFlows, err := runSingleAsyncQuery(url, payload(start89d)); err != nil {
		return false, err
	} else if hasFlows {
		return false, nil
	}

	// both windows reported zero flows → safe to deny
	return true, nil
}

func runSingleAsyncQuery(baseURL string, payload map[string]interface{}) (bool, error) {
	respBytes, err := apiRequestWithRetry("POST", baseURL, payload)
	if err != nil {
		return false, err
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return false, err
	}
	href, ok := resp["href"].(string)
	if !ok || href == "" {
		return false, fmt.Errorf("query failed to return href")
	}

	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return false, fmt.Errorf("query timed out after 5 minutes")
		case <-ticker.C:
			pollBytes, err := apiRequestWithRetry("GET",
				fmt.Sprintf("https://%s:%d/api/v2%s", pce.FQDN, pce.Port, href), nil)
			if err != nil {
				return false, err
			}
			var poll map[string]interface{}
			if err := json.Unmarshal(pollBytes, &poll); err != nil {
				return false, err
			}
			status, _ := poll["status"].(string)
			flowsCount, _ := poll["flows_count"].(float64)

			if status == "completed" {
				return flowsCount > 0, nil
			}
		}
	}
}

func createRuleset(name string) (string, error) {
	payload := map[string]interface{}{
		"name":        name,
		"description": "Created by Auto Deny Rules script.",
		"scopes":      [][]interface{}{{}},
	}
	url := fmt.Sprintf("https://%s:%d/api/v2/orgs/%d/sec_policy/draft/rule_sets", pce.FQDN, pce.Port, pce.Org)
	data, err := apiRequestWithRetry("POST", url, payload)
	if err != nil {
		return "", err
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	href, _ := resp["href"].(string)
	return href, nil
}

func createDenyRule(
	rulesetHref string,
	serviceHref string,
	apps []Label,
	env Label,
	ipListHref string,
) error {
	providers := []map[string]map[string]string{
		{"label": {"href": env.Href}},
	}
	for _, a := range apps {
		providers = append(providers, map[string]map[string]string{
			"label": {"href": a.Href},
		})
	}

	payload := map[string]interface{}{
		"providers": providers,
		"consumers": []map[string]map[string]string{
			{"ip_list": {"href": ipListHref}},
		},
		"enabled": true,
		"ingress_services": []map[string]string{
			{"href": serviceHref},
		},
		"egress_services": []interface{}{},
		"network_type":    "brn",
		"description":     "",
	}

	url := fmt.Sprintf("https://%s:%d/api/v2%s/deny_rules", pce.FQDN, pce.Port, rulesetHref)
	_, err := apiRequestWithRetry("POST", url, payload)
	return err
}

func getIPListHref(targetName string) (string, error) {
	escapedName := url.QueryEscape(targetName)
	urlStr := fmt.Sprintf(
		"https://%s:%d/api/v2/orgs/%d/sec_policy/draft/ip_lists?max_results=500&name=%s",
		pce.FQDN,
		pce.Port,
		pce.Org,
		escapedName,
	)

	data, err := apiRequestWithRetry("GET", urlStr, nil)
	if err != nil {
		return "", err
	}
	var lists []struct {
		Href string `json:"href"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &lists); err != nil {
		return "", err
	}
	if len(lists) == 0 {
		return "", fmt.Errorf("no IP-list found with name %q", targetName)
	}
	return lists[0].Href, nil
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

func logQueryProgress(env, app Label, svc Service, done, total int64) {
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
		env  Label
		apps []Label
	}
	var envInfos []envInfo
	for _, env := range envs {
		apps, err := getWorkloadsForEnv(env)
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
	ipListHref, err := getIPListHref(targetIPListName)
	if err != nil {
		log.Fatalf("Failed to locate IP-list %q: %v", targetIPListName, err)
	}
	log.Printf("Using the Any IP-list href: %s", ipListHref)

	var doneQueries int64
	for _, ei := range envInfos {
		for _, service := range services {
			var appsNoTraffic []Label
			var appsMu sync.Mutex

			for _, app := range ei.apps {
				wg.Add(1)
				sem <- struct{}{}
				go func(a Label) {
					defer wg.Done()
					defer func() { <-sem }()

					ok, err := submitTrafficQuery(
						ei.env.Href, a.Href, service,
						excludeBroadcast, excludeMulticast,
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
					logQueryProgress(ei.env, a, service,
						atomic.LoadInt64(&doneQueries), totalQueries)
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
			if err := createDenyRule(rulesetHref, dr.service.Href, dr.apps, dr.env, ipListHref); err != nil {
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
		// Uncomment to delete automatically:
		/*
			deleteURL := fmt.Sprintf("https://%s:%d/api/v2%s", pce.FQDN, pce.Port, rulesetHref)
			if _, err := apiRequestWithRetry("DELETE", deleteURL, nil); err != nil {
				log.Printf("Failed to delete empty rule set: %v", err)
			} else {
				log.Printf("Deleted empty rule set %s", rulesetHref)
			}
		*/
	}

	log.Println("All queries and deny rules completed.")
	return nil
}
