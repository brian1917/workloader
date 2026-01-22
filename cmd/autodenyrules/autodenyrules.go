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

const (
	maxAppsPerRule         = 198
	maxConcurrentQueries   = 2
	trafficQueryTimeout    = 5 * time.Minute
	trafficQueryPollPeriod = 5 * time.Second
	targetIPListName       = "Any (0.0.0.0/0 and ::/0)"
)

type trafficDirection string

const (
	inbound  trafficDirection = "inbound"
	outbound trafficDirection = "outbound"
)

// denyRuleInfo holds information needed to create a deny rule
type denyRuleInfo struct {
	env       illumioapi.Label   // The environment label
	service   illumioapi.Service // The service to deny
	apps      []illumioapi.Label // The applications with no traffic
	direction trafficDirection
}

type labelFilter struct {
	include map[string]map[string]bool
	exclude map[string]map[string]bool
}

var AutoDenyRulesCmd = &cobra.Command{
	Use:   "auto-deny-rules",
	Short: "Creates deny rules automatically for workloads, based on traffic query results on ransomware services.",
	Long:  `Creates deny rules automatically for workloads, based on traffic query results on ransomware services.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunAutoDenyRules(
			verboseEnabled,
			excludeBroadcastFlag,
			excludeMulticastFlag,
			dryRun,
			includeLabels,
			excludeLabels,
			directionFlag,
		)
	},
}

// optional flags
var (
	verboseEnabled       bool
	excludeBroadcastFlag bool
	excludeMulticastFlag bool
	dryRun               bool
	directionFlag        string
	includeLabels        []string
	excludeLabels        []string
	pce                  illumioapi.PCE
)

func init() {
	AutoDenyRulesCmd.Flags().BoolVar(&verboseEnabled, "verbose", false, "enable verbose logging")
	AutoDenyRulesCmd.Flags().BoolVar(&excludeBroadcastFlag, "exclude-broadcast", false, "exclude broadcast traffic")
	AutoDenyRulesCmd.Flags().BoolVar(&excludeMulticastFlag, "exclude-multicast", false, "exclude multicast traffic")
	AutoDenyRulesCmd.Flags().StringSliceVar(&includeLabels, "include-label", nil, "only include env/app labels matching key:value (repeatable, e.g. --include-label env:Dev)")
	AutoDenyRulesCmd.Flags().StringSliceVar(&excludeLabels, "exclude-label", nil, "exclude env/app labels matching key:value (repeatable, e.g. --exclude-label env:Prod)")
	AutoDenyRulesCmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be created without creating rule sets or rules")
	AutoDenyRulesCmd.Flags().StringVar(&directionFlag, "direction", "", "traffic direction to process: 'inbound' or 'outbound'; omit flag to process both")
}

// ---------- Logging / helpers ----------

func vlog(format string, v ...interface{}) {
	if verboseEnabled {
		log.Printf(format, v...)
	}
}

func logQueryProgress(
	env illumioapi.Label,
	app illumioapi.Label,
	svc illumioapi.Service,
	direction trafficDirection,
	done, total int64,
) {
	percent := float64(done) / float64(total) * 100
	log.Printf("[QUERY:%s] Env:%s  App:%s  Service:%s  →  %.1f%% (%d/%d)",
		strings.ToUpper(string(direction)), env.Value, app.Value, svc.Name, percent, done, total)
}

func ptrString(s string) *string { return &s }
func ptrBool(b bool) *bool       { return &b }

func parseDirectionFlag(direction string) ([]trafficDirection, error) {
	switch strings.ToLower(strings.TrimSpace(direction)) {
	case "":
		// default: both
		return []trafficDirection{inbound, outbound}, nil
	case "inbound":
		return []trafficDirection{inbound}, nil
	case "outbound":
		return []trafficDirection{outbound}, nil
	default:
		return nil, fmt.Errorf("invalid --direction %q (allowed: inbound, outbound; omit flag to process both)", direction)
	}
}

// chunkApps splits apps into chunks of size n.
func chunkApps(apps []illumioapi.Label, n int) [][]illumioapi.Label {
	if len(apps) == 0 || n <= 0 {
		return nil
	}
	chunks := make([][]illumioapi.Label, 0, (len(apps)+n-1)/n)
	for i := 0; i < len(apps); i += n {
		end := i + n
		if end > len(apps) {
			end = len(apps)
		}
		chunks = append(chunks, apps[i:end])
	}
	return chunks
}

// ---------- Label filtering ----------

func buildLabelFilter(includes, excludes []string) (*labelFilter, error) {
	lf := &labelFilter{
		include: map[string]map[string]bool{},
		exclude: map[string]map[string]bool{},
	}

	parse := func(in []string, target map[string]map[string]bool) error {
		for _, s := range in {
			parts := strings.SplitN(s, ":", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid label filter %q, expected key:value", s)
			}
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if key == "" || val == "" {
				return fmt.Errorf("invalid label filter %q, empty key or value", s)
			}
			if target[key] == nil {
				target[key] = map[string]bool{}
			}
			target[key][val] = true
		}
		return nil
	}

	if err := parse(includes, lf.include); err != nil {
		return nil, err
	}
	if err := parse(excludes, lf.exclude); err != nil {
		return nil, err
	}
	return lf, nil
}

func labelAllowed(key, value string, lf *labelFilter) bool {
	if lf == nil {
		return true
	}
	if vals, ok := lf.exclude[key]; ok && vals[value] {
		return false
	}
	if vals, ok := lf.include[key]; ok {
		return vals[value]
	}
	return true
}

// ---------- PCE fetchers ----------

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
func getWorkloadsForEnv(pce illumioapi.PCE, env illumioapi.Label, lf *labelFilter) ([]illumioapi.Label, error) {
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
	for _, wkld := range pce.WorkloadsSlice {
		if wkld.Labels == nil {
			continue
		}
		for _, label := range *wkld.Labels {
			if label.Key != "app" || label.Href == "" {
				continue
			}
			if !labelAllowed("app", label.Value, lf) {
				continue
			}
			uniqueApps[label.Href] = label
		}
	}

	apps := make([]illumioapi.Label, 0, len(uniqueApps))
	for _, label := range uniqueApps {
		apps = append(apps, label)
	}

	vlog("Found %d unique apps for env %s", len(apps), env.Value)
	return apps, nil
}

// ---------- Traffic query ----------

func submitTrafficQuery(
	pce illumioapi.PCE,
	envHref, appHref string,
	service illumioapi.Service,
	excludeBroadcast, excludeMulticast bool,
	direction trafficDirection,
) (bool, error) {
	now := time.Now().UTC()
	end := now.Format(time.RFC3339)
	start24h := now.Add(-24 * time.Hour).Format(time.RFC3339)
	start89d := now.Add(-89 * 24 * time.Hour).Format(time.RFC3339)

	buildPayload := func(start string) map[string]interface{} {
		ports := make([]map[string]interface{}, 0)
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

		var sources, destinations map[string]interface{}
		exclusions := buildDestExclusions(excludeBroadcast, excludeMulticast)

		labelsSelector := [][]map[string]map[string]string{
			{
				{"label": {"href": envHref}},
				{"label": {"href": appHref}},
			},
		}

		if direction == inbound {
			sources = map[string]interface{}{
				"include": []interface{}{[]interface{}{}}, // Any
				"exclude": []interface{}{},
			}
			destinations = map[string]interface{}{
				"include": labelsSelector,
				"exclude": exclusions,
			}
		} else {
			sources = map[string]interface{}{
				"include": labelsSelector,
				"exclude": []interface{}{},
			}
			destinations = map[string]interface{}{
				"include": []interface{}{[]interface{}{}}, // Any
				"exclude": exclusions,
			}
		}

		return map[string]interface{}{
			"query_name":   fmt.Sprintf("Query Env: %s App: %s", envHref, appHref),
			"sources":      sources,
			"destinations": destinations,
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
		return false, nil // Found flows, so not unused
	}

	// If no flows in 24h, check the last 89 days
	hasFlows, err = runSingleAsyncQuery(pce, buildPayload(start89d))
	if err != nil {
		return false, fmt.Errorf("failed to query 89d traffic: %w", err)
	}
	if hasFlows {
		return false, nil
	}

	// No flows found in either time period => unused service for this env/app/direction.
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

	timeout := time.After(trafficQueryTimeout)
	ticker := time.NewTicker(trafficQueryPollPeriod)
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

			_, err := pce.GetHref(resp.Href, &pollResp)
			if err != nil {
				return false, fmt.Errorf("failed to poll async query: %w", err)
			}

			switch pollResp.Status {
			case "completed":
				return pollResp.FlowsCount > 0, nil
			case "failed":
				return false, fmt.Errorf("async query failed")
			}
		}
	}
}

func buildDestExclusions(broadcast, multicast bool) []interface{} {
	excl := make([]interface{}, 0, 2)
	if broadcast {
		excl = append(excl, map[string]string{"transmission": "broadcast"})
	}
	if multicast {
		excl = append(excl, map[string]string{"transmission": "multicast"})
	}
	return excl
}

// ---------- Rule creation ----------

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

func labelsToCOP(apps []illumioapi.Label) []illumioapi.ConsumerOrProvider {
	out := make([]illumioapi.ConsumerOrProvider, 0, len(apps))
	for _, app := range apps {
		out = append(out, illumioapi.ConsumerOrProvider{
			Label: &illumioapi.Label{Href: app.Href},
		})
	}
	return out
}

func createDenyRule(
	pce illumioapi.PCE,
	rulesetHref string,
	serviceHref string,
	apps []illumioapi.Label,
	env illumioapi.Label,
	ipListHref string,
	direction trafficDirection,
) error {

	var providers, consumers []illumioapi.ConsumerOrProvider

	if direction == inbound {
		providers = append(providers, illumioapi.ConsumerOrProvider{Label: &illumioapi.Label{Href: env.Href}})
		providers = append(providers, labelsToCOP(apps)...)
		consumers = []illumioapi.ConsumerOrProvider{{IPList: &illumioapi.IPList{Href: ipListHref}}}
	} else {
		providers = []illumioapi.ConsumerOrProvider{{IPList: &illumioapi.IPList{Href: ipListHref}}}
		consumers = append(consumers, illumioapi.ConsumerOrProvider{Label: &illumioapi.Label{Href: env.Href}})
		consumers = append(consumers, labelsToCOP(apps)...)
	}

	ingressServices := []illumioapi.IngressServices{{Href: serviceHref}}

	rule := illumioapi.Rule{
		RuleType:        "deny",
		Providers:       &providers,
		Consumers:       &consumers,
		IngressServices: &ingressServices,
		Enabled:         ptrBool(true),
		Description:     ptrString(""),
	}

	_, api, err := pce.CreateRule(rulesetHref, rule)
	if err != nil {
		return fmt.Errorf("failed to create deny rule: %w", err)
	}
	vlog("Created deny rule successfully. API response: %s", api.RespBody)
	return nil
}

func getIPListHref(pce illumioapi.PCE, targetName string) (string, error) {
	_, err := pce.GetIPLists(map[string]string{"name": targetName}, "draft")
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

// ---------- Main runner ----------

func RunAutoDenyRules(
	verbose bool,
	excludeBroadcast bool,
	excludeMulticast bool,
	dryRun bool,
	includeLabels []string,
	excludeLabels []string,
	direction string,
) error {

	// Ensure vlog() respects the parameter (fixes shadowing / makes --verbose work).
	verboseEnabled = verbose

	var err error

	selectedDirs, err := parseDirectionFlag(direction)
	if err != nil {
		return err
	}
	vlog("Selected direction(s): %v", selectedDirs)

	pce, err = utils.GetTargetPCEV2(true)
	if err != nil {
		utils.LogError(err.Error())
		return err
	}

	labelFilter, err := buildLabelFilter(includeLabels, excludeLabels)
	if err != nil {
		return err
	}

	envs, err := getEnvs()
	if err != nil {
		return fmt.Errorf("failed to load environments: %w", err)
	}

	services, err := getRansomServices()
	if err != nil {
		return fmt.Errorf("failed to load ransomware services: %w", err)
	}

	friendly := time.Now().Format("Jan 02, 2006 15:04:05")
	rulesetName := fmt.Sprintf("Auto Deny Rules - %s", friendly)

	var rulesetHref string
	if dryRun {
		log.Printf("[DRY-RUN] Would create rule set %q", rulesetName)
	} else {
		rulesetHref, err = createRuleset(rulesetName)
		if err != nil {
			return fmt.Errorf("failed to create rule set: %w", err)
		}
		log.Printf("Created Auto Deny Rules rule set %s", rulesetHref)
	}

	type envInfo struct {
		env  illumioapi.Label
		apps []illumioapi.Label
	}

	envInfos := make([]envInfo, 0, len(envs))
	for _, env := range envs {
		if !labelAllowed("env", env.Value, labelFilter) {
			vlog("Skipping env %s due to label filter", env.Value)
			continue
		}

		apps, err := getWorkloadsForEnv(pce, env, labelFilter)
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
		totalQueries += int64(len(services) * len(ei.apps) * len(selectedDirs))
	}
	if totalQueries == 0 {
		log.Println("No queries to run - exiting.")
		return nil
	}
	log.Printf("Total traffic queries to execute: %d", totalQueries)

	ipListHref, err := getIPListHref(pce, targetIPListName)
	if err != nil {
		return fmt.Errorf("failed to locate IP-list %q: %w", targetIPListName, err)
	}
	log.Printf("Using the Any IP-list href: %s", ipListHref)

	sem := make(chan struct{}, maxConcurrentQueries)

	var (
		wg          sync.WaitGroup
		denyRules   []denyRuleInfo
		denyRulesMu sync.Mutex
		doneQueries int64
	)

	for _, ei := range envInfos {
		ei := ei // capture

		for _, svc := range services {
			svc := svc // capture

			appsNoTraffic := make(map[trafficDirection][]illumioapi.Label, len(selectedDirs))
			for _, d := range selectedDirs {
				appsNoTraffic[d] = []illumioapi.Label{}
			}
			var appsMu sync.Mutex

			for _, app := range ei.apps {
				app := app // capture

				for _, dir := range selectedDirs {
					dir := dir // capture

					wg.Add(1)
					sem <- struct{}{}

					go func() {
						defer wg.Done()
						defer func() { <-sem }()

						ok, err := submitTrafficQuery(
							pce,
							ei.env.Href,
							app.Href,
							svc,
							excludeBroadcast,
							excludeMulticast,
							dir,
						)

						if err != nil {
							log.Printf(
								"[QUERY] %s Env:%s App:%s Service:%s → error: %v",
								dir, ei.env.Value, app.Value, svc.Name, err,
							)
						} else if ok {
							appsMu.Lock()
							appsNoTraffic[dir] = append(appsNoTraffic[dir], app)
							appsMu.Unlock()
						}

						atomic.AddInt64(&doneQueries, 1)
						logQueryProgress(ei.env, app, svc, dir, atomic.LoadInt64(&doneQueries), totalQueries)
					}()
				}
			}

			wg.Wait()

			for dir, apps := range appsNoTraffic {
				if len(apps) == 0 {
					continue
				}
				denyRulesMu.Lock()
				denyRules = append(denyRules, denyRuleInfo{
					env:       ei.env,
					service:   svc,
					apps:      apps,
					direction: dir,
				})
				denyRulesMu.Unlock()
			}
		}
	}

	// Create deny rules
	if len(denyRules) > 0 {
		log.Printf("Creating %d deny rule(s)...", len(denyRules))
		var doneDenyRules int64

		for _, dr := range denyRules {
			appChunks := chunkApps(dr.apps, maxAppsPerRule)

			if len(appChunks) > 1 {
				log.Printf("Service %s in env %s has %d apps — splitting into %d rules",
					dr.service.Name, dr.env.Value, len(dr.apps), len(appChunks))
			}

			for idx, apps := range appChunks {
				if dryRun {
					log.Printf("[DRY-RUN] Would create deny rule %d/%d: env=%s service=%s apps=%d",
						idx+1, len(appChunks), dr.env.Value, dr.service.Name, len(apps))
					continue
				}

				err := createDenyRule(pce, rulesetHref, dr.service.Href, apps, dr.env, ipListHref, dr.direction)
				if err != nil {
					log.Printf("Failed to create deny rule %d/%d for env %s service %s: %v",
						idx+1, len(appChunks), dr.env.Value, dr.service.Name, err)
					continue
				}

				atomic.AddInt64(&doneDenyRules, 1)
			}
		}
	}

	// If no deny rules were created, delete ruleset (unless dry-run).
	if len(denyRules) == 0 {
		if dryRun {
			log.Printf("No deny rules needed - no rule set was created (dry-run)")
		} else {
			log.Printf("No deny rules needed - deleting empty rule set %s", rulesetHref)
			api, err := pce.DeleteHref(rulesetHref)
			if err != nil {
				log.Printf("Failed to delete empty rule set %s: %v", rulesetHref, err)
			} else {
				log.Printf("Deleted empty rule set %s (Status: %d)", rulesetHref, api.StatusCode)
			}
		}
	}

	log.Println("All queries and deny rules completed.")
	return nil
}
