package ebimport

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/cmd/ebexport"
	"github.com/brian1917/workloader/cmd/ruleimport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Input is the data structure for the ImportRulesFromCSV command
type Input struct {
	PCE                                          illumioapi.PCE
	ImportFile                                   string
	ProvisionComment                             string
	Headers                                      map[string]int
	Provision, UpdatePCE, NoPrompt, CreateLabels bool
}

// Decluare a global input and debug variable
var cmdInput Input

func init() {
	EbImportCmd.Flags().BoolVar(&cmdInput.CreateLabels, "create-labels", false, "create labels if they do not exist.")
	EbImportCmd.Flags().BoolVar(&cmdInput.Provision, "provision", false, "provision eb creations/changes.")
	EbImportCmd.Flags().StringVar(&cmdInput.ProvisionComment, "provision-comment", "", "comment for when provisioning changes.")
}

// RuleImportCmd runs the upload command
var EbImportCmd = &cobra.Command{
	Use:   "eb-import [csv file to import]",
	Short: "Create and update enforcement boundaries in the PCE from a CSV file.",
	Long: `
Create and update enforcement boundaries in the PCE from a CSV file.

An easy way to get the input format is to run the workloader eb-export command.

If an href is provided, the existing boundary will be updated. If it's not provided it will be created.

The order of the CSV columns do not matter. The input format accepts the following header values:
- ` + ebexport.HeaderName + `
- ` + ebexport.HeaderHref + ` (no href will create a boundary.)
- ` + ebexport.HeaderEnabled + ` (true/false)
- ` + ebexport.HeaderProviderAllWorkloads + `(true/false)
- ` + ebexport.HeaderProviderLabels + ` (semi-colon separated list in format of key:value. e.g., app:erp;role:db)
- ` + ebexport.HeaderProviderLabelGroups + ` (names of label groups multiple separated by ";")
- ` + ebexport.HeaderProviderIPLists + ` (names of ip lists. multiple separated by ";")
- ` + ebexport.HeaderConsumerAllWorkloads + ` (true/false)
- ` + ebexport.HeaderConsumerLabels + ` (semi-colon separated list in format of key:value. e.g., app:erp;role:db)
- ` + ebexport.HeaderConsumerLabelGroups + ` (names of label groups multiple separated by ";")
- ` + ebexport.HeaderConsumerIPLists + ` (names of ip lists. multiple separated by ";")
- ` + ebexport.HeaderServices + ` (service name, port/proto, or port range/proto. multiple separated by ";")

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		var err error
		cmdInput.PCE, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(fmt.Sprintf("error getting pce - %s", err))
		}

		// Set the CSV file
		if len(args) != 1 {
			utils.LogError("command requires 1 argument for the csv file.")
		}
		cmdInput.ImportFile = args[0]

		// Get the debug value from viper
		cmdInput.UpdatePCE = viper.Get("update_pce").(bool)
		cmdInput.NoPrompt = viper.Get("no_prompt").(bool)

		ImportBoundariesFromCSV(cmdInput)
	},
}

// ImportRulesFromCSV imports a CSV to modify/create rules
func ImportBoundariesFromCSV(input Input) {

	// Load the PCE
	utils.LogInfo("getting boundaries, labels, label groups, iplists, and services...", true)
	apiResps, err := input.PCE.Load(illumioapi.LoadInput{
		EnforcementBoundaries: true,
		Labels:                true,
		IPLists:               true,
		LabelGroups:           true,
		Services:              true,
	}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create a processedBoundary data struct
	type processedBoundary struct {
		boundary illumioapi.EnforcementBoundary
		csvLine  int
	}
	newBoundaries := []processedBoundary{}
	updatedBoundaries := []processedBoundary{}

	// Parse the CSV
	csvData, err := utils.ParseCSV(input.ImportFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Make the headers map
	input.Headers = make(map[string]int)

	// Iterate through the CSV
	for rowIndex, row := range csvData {

		// Process headers on first row and then skip
		if rowIndex == 0 {
			for colIndex, col := range row {
				input.Headers[col] = colIndex
			}
			continue
		}

		// Start the update switch and set some row variables
		update := false

		// Start the row href variable
		var rowHref string

		// If there is an href provided makae sure it exists
		if c, ok := input.Headers[ebexport.HeaderHref]; ok && row[c] != "" {
			rowHref = row[c]
			if _, ebCheck := input.PCE.EnforcementBoundaries[row[c]]; !ebCheck {
				utils.LogWarning(fmt.Sprintf("csv line %d - %s href does not exist. skipping.", rowIndex+1, row[input.Headers[ebexport.HeaderHref]]), true)
				continue
			}
		}

		// Create a mock rule for leverage rule-import logic
		mockRule := illumioapi.Rule{}
		if eb, ok := input.PCE.EnforcementBoundaries[rowHref]; ok {
			mockRule.Href = eb.Href
			mockRule.Providers = eb.Providers
			mockRule.Consumers = eb.Consumers
			mockRule.IngressServices = eb.IngressServices
		}

		// ******************** Consumers ********************
		consumers := []illumioapi.ConsumerOrProvider{}

		// All workloads
		if c, ok := input.Headers[ebexport.HeaderConsumerAllWorkloads]; ok {
			csvAllWorkloads, err := strconv.ParseBool(row[c])
			if err != nil {
				utils.LogError(fmt.Sprintf("csv line %d - %s is not valid boolean for consumer_all_workloads", rowIndex+1, row[c]))
			}
			if eb, ok := input.PCE.EnforcementBoundaries[rowHref]; ok {
				pceAllWklds := false
				for _, cons := range illumioapi.PtrToVal(eb.Consumers) {
					if illumioapi.PtrToVal(cons.Actors) == "ams" {
						pceAllWklds = true
					}
				}
				if pceAllWklds != csvAllWorkloads {
					utils.LogInfo(fmt.Sprintf("csv line %d - consumer_all_workloads needs to be updated from %t to %t", rowIndex+1, pceAllWklds, csvAllWorkloads), false)
					update = true
				}
			}

			if csvAllWorkloads {
				consumers = append(consumers, illumioapi.ConsumerOrProvider{Actors: illumioapi.Ptr("ams")})
			}
		}

		// IP Lists
		if c, ok := input.Headers[ebexport.HeaderConsumerIPLists]; ok {
			consCSVipls := strings.Split(strings.ReplaceAll(row[c], "; ", ";"), ";")
			if row[c] == "" {
				consCSVipls = nil
			}

			// Leverage the IPL Change
			iplChange, ipls := ruleimport.IplComparison(consCSVipls, mockRule, input.PCE.IPLists, rowIndex+1, false)
			if iplChange {
				update = true
			}
			for _, ipl := range ipls {
				consumers = append(consumers, illumioapi.ConsumerOrProvider{IPList: &ipl})
			}
		}

		// Label Groups
		if c, ok := input.Headers[ebexport.HeaderConsumerLabelGroups]; ok {
			consCSVlgs := strings.Split(strings.ReplaceAll(row[c], "; ", ";"), ";")
			if row[c] == "" {
				consCSVlgs = nil
			}
			lgChange, lgs := ruleimport.LabelGroupComparison(consCSVlgs, mockRule, input.PCE.LabelGroups, rowIndex+1, false)
			if lgChange {
				update = true
			}
			for _, lg := range lgs {
				consumers = append(consumers, illumioapi.ConsumerOrProvider{LabelGroup: &lg})
			}
		}

		// Labels
		if row[input.Headers[ebexport.HeaderConsumerLabels]] != "" {
			csvLabels := []illumioapi.Label{}
			// Split at the semi-colons
			userProvidedLabels := strings.Split(strings.Replace(row[input.Headers[ebexport.HeaderConsumerLabels]], "; ", ";", -1), ";")
			for _, label := range userProvidedLabels {
				key := strings.Split(label, ":")[0]
				value := strings.TrimPrefix(label, key+":")
				csvLabels = append(csvLabels, illumioapi.Label{Key: key, Value: value})
			}
			labelUpdate, labels := ruleimport.LabelComparison(csvLabels, input.PCE, mockRule, rowIndex+1, false)
			if labelUpdate {
				update = true
			}
			for _, l := range labels {
				consumers = append(consumers, illumioapi.ConsumerOrProvider{Label: &illumioapi.Label{Href: l.Href}})
			}
		}

		// ******************** Providers ********************
		providers := []illumioapi.ConsumerOrProvider{}

		// All workloads
		if c, ok := input.Headers[ebexport.HeaderProviderAllWorkloads]; ok {
			csvAllWorkloads, err := strconv.ParseBool(row[c])
			if err != nil {
				utils.LogError(fmt.Sprintf("csv line %d - %s is not valid boolean for provider_all_workloads", rowIndex+1, row[c]))
			}
			if eb, ok := input.PCE.EnforcementBoundaries[rowHref]; ok {
				pceAllWklds := false
				for _, provs := range illumioapi.PtrToVal(eb.Providers) {
					if illumioapi.PtrToVal(provs.Actors) == "ams" {
						pceAllWklds = true
					}
				}
				if pceAllWklds != csvAllWorkloads {
					utils.LogInfo(fmt.Sprintf("csv line %d - provider_all_workloads needs to be updated from %t to %t", rowIndex+1, pceAllWklds, csvAllWorkloads), false)
					update = true
				}
			}
			if csvAllWorkloads {
				providers = append(providers, illumioapi.ConsumerOrProvider{Actors: illumioapi.Ptr("ams")})
			}
		}

		// IP Lists
		if c, ok := input.Headers[ebexport.HeaderProviderIPLists]; ok {
			provsCSVipls := strings.Split(strings.ReplaceAll(row[c], "; ", ";"), ";")
			if row[c] == "" {
				provsCSVipls = nil
			}

			// Leverage the IPL Change
			iplChange, ipls := ruleimport.IplComparison(provsCSVipls, mockRule, input.PCE.IPLists, rowIndex+1, true)
			if iplChange {
				update = true
			}
			for _, ipl := range ipls {
				providers = append(providers, illumioapi.ConsumerOrProvider{IPList: &ipl})
			}
		}

		// Label Groups
		if c, ok := input.Headers[ebexport.HeaderProviderLabelGroups]; ok {
			provsCSVlgs := strings.Split(strings.ReplaceAll(row[c], "; ", ";"), ";")
			if row[c] == "" {
				provsCSVlgs = nil
			}
			lgChange, lgs := ruleimport.LabelGroupComparison(provsCSVlgs, mockRule, input.PCE.LabelGroups, rowIndex+1, true)
			if lgChange {
				update = true
			}
			for _, lg := range lgs {
				providers = append(providers, illumioapi.ConsumerOrProvider{LabelGroup: &lg})
			}
		}

		// Labels
		if row[input.Headers[ebexport.HeaderProviderLabels]] != "" {
			csvLabels := []illumioapi.Label{}
			// Split at the semi-colons
			userProvidedLabels := strings.Split(strings.Replace(row[input.Headers[ebexport.HeaderProviderLabels]], "; ", ";", -1), ";")
			for _, label := range userProvidedLabels {
				key := strings.Split(label, ":")[0]
				value := strings.TrimPrefix(label, key+":")
				csvLabels = append(csvLabels, illumioapi.Label{Key: key, Value: value})
			}
			labelUpdate, labels := ruleimport.LabelComparison(csvLabels, input.PCE, mockRule, rowIndex+1, true)
			if labelUpdate {
				update = true
			}
			for _, l := range labels {
				providers = append(providers, illumioapi.ConsumerOrProvider{Label: &illumioapi.Label{Href: l.Href}})
			}
		}

		// ******************** Services ********************
		var ingressSvc []illumioapi.IngressServices
		var svcChange bool
		if c, ok := input.Headers[ebexport.HeaderServices]; ok {
			csvServices := strings.Split(strings.ReplaceAll(row[c], "; ", ";"), ";")
			if row[c] == "" {
				csvServices = nil
			}
			svcChange, ingressSvc = ruleimport.ServiceComparison(csvServices, mockRule, input.PCE.Services, rowIndex+1)
			if svcChange {
				update = true
			}
			if ingressSvc == nil {
				ingressSvc = append(ingressSvc, illumioapi.IngressServices{})
			}
		}

		// ******************** Network Type ********************/
		var networkType string
		if c, ok := input.Headers[ebexport.HeaderNetworkType]; ok {
			networkType = strings.ToLower(row[c])
			if networkType != "brn" && networkType != "non_brn" && networkType != "all" && networkType != "" {
				utils.LogError(fmt.Sprintf("csv line %d - %s is not valid network type. must be brn, non_brn, or all", rowIndex+1, row[c]))
			}
			if rowHref != "" {
				if input.PCE.EnforcementBoundaries[rowHref].NetworkType != networkType {
					update = true
					utils.LogInfo(fmt.Sprintf("csv line %d - network_type needs to be updated from %s to %s.", rowIndex+1, input.PCE.EnforcementBoundaries[rowHref].NetworkType, networkType), false)
				}
			}
		}

		// ******************** Name ********************/
		var name string
		if c, ok := input.Headers[ebexport.HeaderName]; ok {
			if rowHref != "" && input.PCE.EnforcementBoundaries[rowHref].Name != row[c] {
				update = true
				utils.LogInfo(fmt.Sprintf("csv line %d - name needs to be updated from %s to %s.", rowIndex+1, input.PCE.EnforcementBoundaries[rowHref].Name, row[c]), false)
			}
			name = row[c]
		}

		// ******************** Enabled ********************
		var enabled bool
		if c, ok := input.Headers[ebexport.HeaderEnabled]; ok {
			enabled, err = strconv.ParseBool(row[c])
			if err != nil {
				utils.LogError(fmt.Sprintf("csv line %d - %s is not valid boolean for rule_enabled", rowIndex+1, row[c]))
			}
			if rowHref != "" && *input.PCE.EnforcementBoundaries[rowHref].Enabled != enabled {
				update = true
				utils.LogInfo(fmt.Sprintf("csv line %d - rule_enabled needs to be updated from %t to %t.", rowIndex+1, !enabled, enabled), false)
			}

		}

		// Create the enforcement boundary
		eb := illumioapi.EnforcementBoundary{
			Name:            name,
			Providers:       &providers,
			Consumers:       &consumers,
			IngressServices: &ingressSvc,
			NetworkType:     networkType,
			Enabled:         &enabled,
		}

		// Adjust the network type. If PCE is older, clear it. If it's newwer and has a blank value, set it as brn.
		if input.PCE.Version.Major < 22 || (input.PCE.Version.Major == 22 && input.PCE.Version.Minor <= 2) {
			eb.NetworkType = ""
		} else if eb.NetworkType == "" {
			eb.NetworkType = "brn"
		}

		// Add to the right slice
		if rowHref == "" {
			newBoundaries = append(newBoundaries, processedBoundary{csvLine: rowIndex + 1, boundary: eb})
			utils.LogInfo(fmt.Sprintf("csv line %d - create new boundary", rowIndex+1), false)
		} else {
			if update {
				eb.Href = rowHref
				updatedBoundaries = append(updatedBoundaries, processedBoundary{csvLine: rowIndex + 1, boundary: eb})
			}
		}
	}

	// End run if we have nothing to do
	if len(newBoundaries) == 0 && len(updatedBoundaries) == 0 {
		utils.LogInfo("nothing to be done", true)
		utils.LogEndCommand("eb-import")
		return
	}

	// Log findings
	if !input.UpdatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d boundaries to create and %d boundaries to update. see workloader.log for details. to do the import, run again using --update-pce flag.", len(newBoundaries), len(updatedBoundaries)), true)
		utils.LogEndCommand("eb-import")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if input.UpdatePCE && !input.NoPrompt {
		var prompt string
		fmt.Printf("\r\n[PROMPT] - workloader identified %d boundaries to create and %d boundaries to update in %s (%s). do you want to run the import (yes/no)? ", len(newBoundaries), len(updatedBoundaries), input.PCE.FriendlyName, viper.Get(input.PCE.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied.", true)
			utils.LogEndCommand("rule-import")
			return
		}
	}

	// Create the new rules
	provisionHrefs := []string{}
	if len(newBoundaries) > 0 {
		for _, nb := range newBoundaries {
			eb, a, err := input.PCE.CreateEnforcementBoundary(nb.boundary)
			utils.LogAPIRespV2("CreateEnforcementBoundary", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			provisionHrefs = append(provisionHrefs, strings.Split(eb.Href, "/sec_rules")[0])
			utils.LogInfo(fmt.Sprintf("csv line %d - created boundary %s - %d", nb.csvLine, eb.Href, a.StatusCode), true)
		}
	}

	// Update the new rules
	if len(updatedBoundaries) > 0 {
		for _, ub := range updatedBoundaries {
			a, err := input.PCE.UpdateEnforcementBoundary(ub.boundary)
			utils.LogAPIRespV2("UpdateEnforcementBoundary", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			provisionHrefs = append(provisionHrefs, strings.Split(ub.boundary.Href, "/sec_rules")[0])
			utils.LogInfo(fmt.Sprintf("csv line %d - updated rule %s - %d", ub.csvLine, ub.boundary.Href, a.StatusCode), true)
		}
	}

	if input.Provision {
		a, err := input.PCE.ProvisionHref(provisionHrefs, input.ProvisionComment)
		utils.LogAPIRespV2("ProvisionHref", a)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("provisioning complete - status code %d", a.StatusCode), true)
	}

	// Log end
	utils.LogEndCommand("rule-import")

}
