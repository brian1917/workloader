package containmentswitch

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var start, end, objectName string
var skipAllow, skipModeChange, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

func init() {
	ContainmentSwitchCmd.Flags().StringVarP(&start, "start", "s", time.Now().AddDate(0, 0, -14).In(time.UTC).Format("2006-01-02"), "start date in the format of yyyy-mm-dd.")
	ContainmentSwitchCmd.Flags().StringVarP(&end, "end", "e", time.Now().AddDate(0, 0, -7).In(time.UTC).Format("2006-01-02"), "end date in the format of yyyy-mm-dd.")
	ContainmentSwitchCmd.Flags().BoolVar(&skipAllow, "skip-allow", false, "do not analyze traffic to see where traffic should be allowed.")
	ContainmentSwitchCmd.Flags().BoolVar(&skipModeChange, "skip-mode-change", false, "do not move all visibility-only workloads into selective-enforcement.")
	ContainmentSwitchCmd.Flags().StringVar(&objectName, "object-name", "", "name for created policy objects (virtual services, rules, and enforcement boundaries). if none is provided the default is \"Workloader-Containment-Switch-Port-Protocol\"")

	ContainmentSwitchCmd.Flags().SortFlags = false
}

// TrafficCmd runs the workload identifier
var ContainmentSwitchCmd = &cobra.Command{
	Use:   "containment-switch [port protocol]",
	Short: "Isolate a port on all workloads and optionally keep open the port on workloads that had traffic to that port in a configurable past window.",
	Long: `
Isolate a port on all workloads and optionally keep open the port on workloads that had traffic to that port in a configurable past window.

The command does the following:
1. Queries explorer to identify workloads with traffic to the target port in the window specified.
2. Creates a virtual service for the target port.
3. Binds the identified workloads from the explorer query to the virtual service.
4. Creates a ruleset and a rule allowing traffic to access the created virtual service.
5. Creates an enforcement boundary for any IP address to all workloads on the target port.
6. Provisions all created objects.
7. Moves all visibility-only workloads into selective enforcement.

This process allows any workload that has had flows to the target port in the defined window to continue accepting connections while closing the target port on all other workloads. The traffic analysis start and end dates can be configured with the --start (-s) and --end (-e) flags. The default start is 14 days from today and the default end is 7 days from today.

Steps 1 through 4 can be skipped with the --skip-allow flag to bypass creating any rules to allow traffic on the blocked port.

Step 7 can be skipped with the --skip-mode-change so visibility-only workloads are not put into selective-enforcement.

The --update-pce flag is required for Steps 2 through 7. If the --update-pce flag is not set workloader will run the explorer query and provide information for how many workloads would be bound to the virtual service for the allow rule.
`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get User Input
		if len(args) != 2 {
			fmt.Println("Command requires 2 arguments for the port and protocol. The input should be in the format of 445 tcp. See usage help.")
			os.Exit(0)
		}
		targetPort, err := strconv.Atoi(args[0])
		if err != nil {
			utils.LogError(fmt.Sprintf("invalid input - %s is not an integer.", args[0]))
		}

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		portLock(targetPort, args[1])
	},
}

func portLock(port int, protocol string) {
	// Log the start of the command
	utils.LogStartCommand("containment-switch")

	// Get visibility only workloads
	visOnlywklds, api, err := pce.GetWklds(map[string]string{"managed": "true", "enforcement_mode": "visibility_only"})
	utils.LogAPIResp("GetAllWorkloadsQP", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	if port < 0 || port > 65535 {
		utils.LogError(fmt.Sprintf("invalid input - %d is not a valid port.", port))
	}

	// Validate the protocol
	protocol = strings.ToLower(protocol)
	if protocol != "tcp" && protocol != "udp" {
		utils.LogError(fmt.Sprintf("invalid input - %s is not a valid protocol", strings.ToLower(protocol)))
	}
	// Set to the right protocol
	protocolNum := 17
	if protocol == "tcp" {
		protocolNum = 6
	}

	// Set the Object Name
	if objectName == "" {
		objectName = fmt.Sprintf("Workloader-Containment-Switch-%d-%s", port, protocol)
	}

	// Get the Any IP List for use in the rule and/or enfourcement boundary.
	// Get it here so it's available in the traffic conditional as well as in the EB
	anyIPList, api, err := pce.GetIPListByName("Any (0.0.0.0/0 and ::/0)", "active")
	utils.LogAPIResp("GetIPList", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	if !skipAllow {
		// Build the explorer query
		tq := illumioapi.TrafficQuery{
			MaxFLows:                        100000,
			PolicyStatuses:                  []string{"potentially_blocked", "unknown"},
			PortProtoInclude:                [][2]int{{port, protocolNum}},
			ExcludeWorkloadsFromIPListQuery: true,
		}

		// Get the start date
		tq.StartTime, err = time.Parse("2006-01-02 MST", fmt.Sprintf("%s %s", start, "UTC"))
		if err != nil {
			utils.LogError(err.Error())
		}
		tq.StartTime = tq.StartTime.In(time.UTC)

		// Get the end date
		tq.EndTime, err = time.Parse("2006-01-02 15:04:05 MST", fmt.Sprintf("%s 23:59:59 %s", end, "UTC"))
		if err != nil {
			utils.LogError(err.Error())
		}
		tq.EndTime = tq.EndTime.In(time.UTC)

		// Run traffic query
		traffic, api, err := pce.GetTrafficAnalysis(tq)
		utils.LogAPIResp("GetTrafficAnalysis", api)
		if err != nil {
			utils.LogError(fmt.Sprintf("making explorer API call - %s", err))
		}
		utils.LogInfo(fmt.Sprintf("explorer query returned %d records", len(traffic)), true)

		// Get all the workloads with the inbound traffic
		targetWorkloads := make(map[string]illumioapi.Workload)
		for _, t := range traffic {
			if t.Dst.Workload != nil && t.Dst.Workload.Href != "" && !strings.Contains(t.Dst.Workload.Href, "/container_workloads/") {
				targetWorkloads[t.Dst.Workload.Href] = *t.Dst.Workload
			}
		}
		utils.LogInfo(fmt.Sprintf("identified %d workloads to bind to the virtual service. See workloader.log for list.", len(targetWorkloads)), true)
		for _, t := range targetWorkloads {
			name := t.Hostname
			if name == "" {
				name = t.Name
			}
			utils.LogInfo(fmt.Sprintf("%s - %s", name, t.Href), false)
		}

		// Check that we should make changes to the PCE.
		if !updatePCE {
			utils.LogInfo("run with --update-pce and optionally --no-prompt flag to implement containment-switch.", true)
			utils.LogEndCommand("containment-switch")
			return
		}

		if !noPrompt {
			changes := []string{}
			if len(targetWorkloads) > 0 {
				changes = append(changes, fmt.Sprintf("create the %s virtual service and bind %d workloads to it", objectName, len(targetWorkloads)))
				changes = append(changes, fmt.Sprintf("create the %s ruleset allowing traffic to the created virtual service on %d %s", objectName, port, protocol))
			}
			changes = append(changes, fmt.Sprintf("create the %s enforcement boundary for any IP address to all workloads on %d %s", objectName, port, protocol))
			if !skipModeChange {
				changes = append(changes, fmt.Sprintf("move %d workloads from visibility-only to selective-enforcement to enforce created boundary", len(visOnlywklds)))
			}

			var prompt string
			fmt.Printf("\r\n%s[PROMPT] - workloader will do the following in %s (%s):\r\n", time.Now().Format("2006-01-02 15:04:05 "), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
			for i, c := range changes {
				fmt.Printf("%s [PROMPT] - %d) %s\r\n", time.Now().Format("2006-01-02 15:04:05"), i+1, c)
			}
			fmt.Printf("%s [PROMPT] - Do you want to run the containment-switch (yes/no)? ", time.Now().Format("2006-01-02 15:04:05"))
			fmt.Scanln(&prompt)
			if strings.ToLower(prompt) != "yes" {
				utils.LogInfo("prompt denied", true)
				utils.LogEndCommand("containment-switch")
				return
			}
			fmt.Println()
		}

		// Create the virutal service if we have workloads that need it.
		if len(targetWorkloads) > 0 {

			// Create the virtual service
			vs := illumioapi.VirtualService{
				Description:  fmt.Sprintf("created by workloader containment-switch for %d %s", port, protocol),
				Name:         objectName,
				ServicePorts: []*illumioapi.ServicePort{{Port: port, Protocol: protocolNum}}}

			vs, api, err = pce.CreateVirtualService(vs)
			utils.LogAPIResp("CreateVirtualService", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("created virtual service - %s - %s - status code: %d", vs.Name, vs.Href, api.StatusCode), true)

			// Provision the virutal service
			api, err = pce.ProvisionHref([]string{vs.Href}, "provisioned by workloader containment-switch")
			utils.LogAPIResp("ProvisionHref", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("provisioned virtual service - status code: %d", api.StatusCode), true)

			// Bind the workloads
			serviceBindings := []illumioapi.ServiceBinding{}
			for _, w := range targetWorkloads {
				serviceBindings = append(serviceBindings, illumioapi.ServiceBinding{VirtualService: vs, Workload: w})
			}
			_, api, err = pce.CreateServiceBinding(serviceBindings)
			utils.LogAPIResp("CreateServiceBinding", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("bound %d workloads to %s virtual service - status code: %d", len(targetWorkloads), vs.Name, api.StatusCode), true)

			// Create a new ruleset
			rs := illumioapi.RuleSet{
				Description: "created by workloader containment-switch",
				Name:        objectName,
			}
			rs.Scopes = append(rs.Scopes, []*illumioapi.Scopes{})
			rs, api, err = pce.CreateRuleset(rs)
			utils.LogAPIResp("CreateRuleSet", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("created %s ruleset - %s - status code: %d", rs.Name, rs.Href, api.StatusCode), true)

			// Create a new rule
			enabled := true
			rule := illumioapi.Rule{
				Providers:       []*illumioapi.Providers{{VirtualService: &illumioapi.VirtualService{Href: vs.Href}}},
				Consumers:       []*illumioapi.Consumers{{IPList: &illumioapi.IPList{Href: anyIPList.Href}}},
				Enabled:         &enabled,
				ResolveLabelsAs: &illumioapi.ResolveLabelsAs{Consumers: []string{"workloads"}, Providers: []string{"virtual_services"}},
				IngressServices: &[]*illumioapi.IngressServices{},
			}
			rule, api, err = pce.CreateRule(rs.Href, rule)
			utils.LogAPIResp("CreateRuleSetRule", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("created rule in %s - %s - status code: %d", rs.Name, rule.Href, api.StatusCode), true)

			// Provision the ruleset
			api, err = pce.ProvisionHref([]string{rs.Href}, "provisioned by workloader containment-switch")
			utils.LogAPIResp("ProvisionHref", api)
			if err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("provisioned ruleset - status code: %d", api.StatusCode), true)

		}
	}

	// Create the enforcement boundary
	eb := illumioapi.EnforcementBoundary{
		Name:            objectName,
		Consumers:       []illumioapi.Consumers{{IPList: &illumioapi.IPList{Href: anyIPList.Href}}},
		Providers:       []illumioapi.Providers{{Actors: "ams"}},
		IngressServices: []illumioapi.IngressServices{{Port: &port, Protocol: &protocolNum}},
	}
	eb, api, err = pce.CreateEnforcementBoundary(eb)
	utils.LogAPIResp("CreateEnforcementBoundary", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	utils.LogInfo(fmt.Sprintf("created enforcement boundary - %s - %s - status code: %d", eb.Name, eb.Href, api.StatusCode), true)

	// Provision enforcement boundary
	api, err = pce.ProvisionHref([]string{eb.Href}, "provisioned by workloader containment-switch")
	utils.LogAPIResp("ProvisionHref", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	utils.LogInfo(fmt.Sprintf("provisioned enforcement boundary - status code: %d", api.StatusCode), true)

	// Move all visibility-only workloads into selective enforcement
	if !skipModeChange {
		updateWklds := []illumioapi.Workload{}
		for _, w := range visOnlywklds {
			w.EnforcementMode = "selective"
			updateWklds = append(updateWklds, w)
		}
		utils.LogInfo(fmt.Sprintf("identified %d workloads in visibility only requiring move to selective", len(updateWklds)), true)
		if len(updateWklds) > 0 {
			apiResps, err := pce.BulkWorkload(updateWklds, "update", true)
			for _, a := range apiResps {
				utils.LogAPIResp("BulkWorkload", a)
			}
			if err != nil {
				utils.LogError(err.Error())
			}
		}
	}
	// Log the end of the command
	utils.LogEndCommand("containment-switch")
}
