package svcimport

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/svcexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/viper"
)

// Input is the input object for the ImportServices Command
type Input struct {
	PCE          illumioapi.PCE
	Data         [][]string
	UpdatePCE    bool
	NoPrompt     bool
	Provision    bool
	UpdateOnName bool
	Meta         bool
	Headers      map[string]int
}

type csvService struct {
	service  illumioapi.Service
	csvLines []int
}

func intSliceToStrSlice(integers []int) []string {
	a := []string{}
	for _, i := range integers {
		a = append(a, strconv.Itoa(i))
	}
	return a
}

// ImportServices imports services
func ImportServices(input Input) {

	// Check for duplicate service names
	svcNameMap := make(map[string]int)
	for _, svc := range input.PCE.ServicesSlice {
		svcNameMap[svc.Name] = svcNameMap[svc.Name] + 1
	}
	for svc, count := range svcNameMap {
		if count > 1 {
			utils.LogWarning(fmt.Sprintf("The PCE has %d service objects named %s. It is not recommended to use workloader svc-import unless service names are unique.", count, input.PCE.Services[svc].Name), true)
		}
	}

	// Process the headers
	input.processHeaders(input.Data[0])

	// Create the csvServicesMap the key is going to be the name of the service
	csvSvcMap := make(map[string]csvService)

	// Start the slices for storing updates and new services
	updatedServices := []csvService{}
	newServices := []csvService{}

	// Iterate through the services provided in the CSV
	for r, data := range input.Data {

		// Set the csv line for logging
		csvLine := r + 1

		// Skip the header column
		if r == 0 {
			continue
		}

		// Process the meta import (names, descriptions, ransomware, etc.)
		if input.Meta {
			update := false

			// Set the match flag
			matchStr := ""

			// Get the href
			var href, name string
			if hrefCol, ok := input.Headers[svcexport.HeaderHref]; !ok {
				// utils.LogError("href header required with meta flag")
			} else {
				href = data[hrefCol]
				if href == "" {
					utils.LogErrorf("csv line %d - no header provided.", csvLine)
				}
				matchStr = href
			}

			if matchStr == "" {
				if nameCol, ok := input.Headers[svcexport.HeaderName]; !ok {
					utils.LogError("either href or name column must be present")
				} else {
					name = data[nameCol]
					if name == "" {
						utils.LogErrorf("csv line %d - no name or href provided.", csvLine)
					}
					matchStr = name
				}

			}

			// Get the service
			var newSvc illumioapi.Service
			var ok bool
			if newSvc, ok = input.PCE.Services[matchStr]; !ok {
				utils.LogErrorf("csv line %d - %s does not exist", csvLine, href)
			}
			href = newSvc.Href
			if illumioapi.PtrToVal(illumioapi.PtrToVal(newSvc.RiskDetails).Ransomware).Category == "" {
				newSvc.RiskDetails = nil
			}

			// Check the name
			if nameCol, ok := input.Headers[svcexport.HeaderName]; ok {
				if newSvc.Name != data[nameCol] {
					utils.LogInfof(false, "csv line %d - %s - name to be updated from %s to %s", csvLine, newSvc.Href, newSvc.Name, data[nameCol])
					newSvc.Name = data[nameCol]
					update = true
				}
			}

			// Check the description
			if descCol, ok := input.Headers[svcexport.HeaderDescription]; ok {
				if newSvc.Description != data[descCol] {
					utils.LogInfof(false, "csv line %d - %s - description to be updated from %s to %s", csvLine, newSvc.Href, newSvc.Description, data[descCol])
					newSvc.Description = data[descCol]
					update = true
				}
			}

			// Check the ransomware category
			if catCol, ok := input.Headers[svcexport.HeaderRansomwareCategory]; ok {
				existing := ""
				if newSvc.RiskDetails != nil && newSvc.RiskDetails.Ransomware != nil {
					existing = newSvc.RiskDetails.Ransomware.Category
				}
				if existing != data[catCol] {
					utils.LogInfof(false, "csv line %d - %s - ransomware category to be updated from %s to %s", csvLine, href, existing, data[catCol])
					if newSvc.RiskDetails == nil || newSvc.RiskDetails.Ransomware == nil {
						newSvc.RiskDetails = &illumioapi.RiskDetail{Ransomware: &illumioapi.Ransomware{Category: data[catCol]}}
					} else {
						newSvc.RiskDetails.Ransomware.Category = data[catCol]
					}
					update = true
				}
			}

			// Check the ransomware severity
			if sevCol, ok := input.Headers[svcexport.HeaderRansomwareSeverity]; ok {
				existing := ""
				if newSvc.RiskDetails != nil && newSvc.RiskDetails.Ransomware != nil {
					existing = newSvc.RiskDetails.Ransomware.Severity
				}
				if existing != data[sevCol] {
					utils.LogInfof(false, "csv line %d - %s - ransomware severity to be updated from %s to %s", csvLine, href, existing, data[sevCol])
					if newSvc.RiskDetails == nil || newSvc.RiskDetails.Ransomware == nil {
						newSvc.RiskDetails = &illumioapi.RiskDetail{Ransomware: &illumioapi.Ransomware{Severity: data[sevCol]}}
					} else {
						newSvc.RiskDetails.Ransomware.Severity = data[sevCol]
					}
					update = true
				}
			}

			// Check the ransomware OS
			if osCol, ok := input.Headers[svcexport.HeaderRansomWareOs]; ok {
				existing := []string{}
				if newSvc.RiskDetails != nil && newSvc.RiskDetails.Ransomware != nil {
					existing = newSvc.RiskDetails.Ransomware.OsPlatforms
				}
				csv := strings.Replace(data[osCol], " ", "", -1)
				csvSlice := []string{}
				if len(csv) != 0 {
					csvSlice = strings.Split(csv, ";")
				}
				equal, log := utils.SliceComare(existing, csvSlice, "pce", "csv")
				if !equal {
					utils.LogInfof(false, "csv line %d - %s - ransomware os - %s", csvLine, href, log)
					if newSvc.RiskDetails == nil || newSvc.RiskDetails.Ransomware == nil {
						newSvc.RiskDetails = &illumioapi.RiskDetail{Ransomware: &illumioapi.Ransomware{OsPlatforms: strings.Split(csv, ";")}}
					} else {
						newSvc.RiskDetails.Ransomware.OsPlatforms = csvSlice
					}
					update = true
				}
			}

			if update {
				updatedServices = append(updatedServices, csvService{csvLines: []int{csvLine}, service: newSvc})
			}
		}

		// Process the non Meta input
		if !input.Meta {
			// Process other imports by building the

			// Get the service type
			var isWinSvc bool
			var err error
			if col, ok := input.Headers[svcexport.HeaderWinService]; ok {
				isWinSvc, err = strconv.ParseBool(data[col])
				if err != nil {
					utils.LogError(fmt.Sprintf("csv line %d - invalid boolean value for %s", csvLine, svcexport.HeaderWinService))
				}
			}

			// Create or update the entry in the map
			if nameCol, ok := input.Headers[svcexport.HeaderName]; !ok {
				utils.LogError("name header is required")
			} else {
				// If the name column is blank, error
				if data[nameCol] == "" {
					utils.LogError(fmt.Sprintf("csv line %d - name required", csvLine))
				}
				if data[nameCol] == "All Services" {
					utils.LogInfo(fmt.Sprintf("csv line %d - skipping All Services", csvLine), true)
					continue
				}
				// If the service exists already, add to it
				if csvSvc, ok := csvSvcMap[data[nameCol]]; ok {
					winSvc, svcPort := processServices(input, data, csvLine)
					if isWinSvc {
						if csvSvc.service.WindowsServices == nil {
							csvSvc.service.WindowsServices = &[]illumioapi.WindowsService{winSvc}
						} else {
							*csvSvc.service.WindowsServices = append(*csvSvc.service.WindowsServices, winSvc)
						}
					} else {
						if csvSvc.service.ServicePorts == nil {
							csvSvc.service.ServicePorts = &[]illumioapi.ServicePort{svcPort}
						} else {
							*csvSvc.service.ServicePorts = append(*csvSvc.service.ServicePorts, svcPort)
						}
					}
					csvSvcMap[data[nameCol]] = csvService{
						csvLines: append(csvSvc.csvLines, csvLine),
						service:  csvSvc.service}

				} else {
					// If the service doesn't already exist, create it.
					winSvc, svcPort := processServices(input, data, csvLine)
					svc := illumioapi.Service{Name: data[nameCol]}
					if isWinSvc {
						svc.WindowsServices = &[]illumioapi.WindowsService{winSvc}
					} else {
						svc.ServicePorts = &[]illumioapi.ServicePort{svcPort}
					}

					// Add the href
					if col, ok := input.Headers[svcexport.HeaderHref]; ok {
						svc.Href = data[col]
					} else if input.UpdateOnName {
						svc.Href = input.PCE.Services[data[nameCol]].Href
					}

					// Add the description
					if col, ok := input.Headers[svcexport.HeaderDescription]; ok {
						svc.Description = data[col]
					}

					csvSvcMap[data[nameCol]] = csvService{
						csvLines: []int{csvLine},
						service:  svc}
				}
			}
		}
	}

	// Iterate through the CSV Map
	// Conver the CSVMap
	if !input.Meta {
		for _, csvSvc := range csvSvcMap {
			if csvSvc.service.Href == "" {
				// Check if the service exists in the PCE.
				if _, ok := svcNameMap[csvSvc.service.Name]; ok {
					utils.LogError(fmt.Sprintf("csv line %s - %s already exists in the PCE. add an href to update it or use the --update-on-name flag.", strings.Join(intSliceToStrSlice(csvSvc.csvLines), ", "), csvSvc.service.Name))
				}
				newServices = append(newServices, csvSvc)
				utils.LogInfo(fmt.Sprintf("csv line(s) %s - %s to be created", strings.Join(intSliceToStrSlice(csvSvc.csvLines), ", "), csvSvc.service.Name), false)
			} else {
				// Href is provided so we need to check if we need to update
				if pceSvc, ok := input.PCE.Services[csvSvc.service.Href]; !ok {
					utils.LogError(fmt.Sprintf("csv line(s) %s - %s does not exist in the PCE", strings.Join(intSliceToStrSlice(csvSvc.csvLines), ", "), csvSvc.service.Href))
				} else {

					// Create a map of the pceSvc. The key is going to be name-port-toport-protocol-process-svc-icmpcode-icmptype
					pceSvcMapSvcs := make(map[string]string)
					for _, ws := range illumioapi.PtrToVal(pceSvc.WindowsServices) {
						pceSvcMapSvcs[fmt.Sprintf("%s-%d-%d-%d-%s-%s-%d-%d", pceSvc.Href, illumioapi.PtrToVal(ws.Port), ws.ToPort, ws.Protocol, ws.ProcessName, ws.ServiceName, ws.IcmpCode, ws.IcmpType)] = fmt.Sprintf("Port: %d; To Port: %d; Proto: %d; ProcessName: %s; Service: %s; ICMP Code: %d; ICMP Type: %d", illumioapi.PtrToVal(ws.Port), ws.ToPort, ws.Protocol, ws.ProcessName, ws.ServiceName, ws.IcmpCode, ws.IcmpType)
					}
					for _, svp := range illumioapi.PtrToVal(pceSvc.ServicePorts) {
						pceSvcMapSvcs[fmt.Sprintf("%s-%d-%d-%d-%d-%d", pceSvc.Href, illumioapi.PtrToVal(svp.Port), svp.ToPort, svp.Protocol, svp.IcmpCode, svp.IcmpType)] = fmt.Sprintf("Port: %d; To Port: %d; Proto: %d; ICMP Code: %d; ICMP Type: %d", illumioapi.PtrToVal(svp.Port), svp.ToPort, svp.Protocol, svp.IcmpCode, svp.IcmpType)
					}

					// Create a map of csvSvc with the same key
					csvSvcMapSvcs := make(map[string]string)
					for _, ws := range illumioapi.PtrToVal(csvSvc.service.WindowsServices) {
						csvSvcMapSvcs[fmt.Sprintf("%s-%d-%d-%d-%s-%s-%d-%d", csvSvc.service.Href, illumioapi.PtrToVal(ws.Port), ws.ToPort, ws.Protocol, ws.ProcessName, ws.ServiceName, ws.IcmpCode, ws.IcmpType)] = fmt.Sprintf("Port: %d; To Port: %d; Proto: %d; ProcessName: %s; Service: %s; ICMP Code: %d; ICMP Type: %d", illumioapi.PtrToVal(ws.Port), ws.ToPort, ws.Protocol, ws.ProcessName, ws.ServiceName, ws.IcmpCode, ws.IcmpType)
					}
					for _, svp := range illumioapi.PtrToVal(csvSvc.service.ServicePorts) {
						csvSvcMapSvcs[fmt.Sprintf("%s-%d-%d-%d-%d-%d", csvSvc.service.Href, illumioapi.PtrToVal(svp.Port), svp.ToPort, svp.Protocol, svp.IcmpCode, svp.IcmpType)] = fmt.Sprintf("Port: %d; To Port: %d; Proto: %d; ICMP Code: %d; ICMP Type: %d", illumioapi.PtrToVal(svp.Port), svp.ToPort, svp.Protocol, svp.IcmpCode, svp.IcmpType)
					}

					update := false
					// Are all the services in the CSV entry in the PCE?
					for s := range csvSvcMapSvcs {
						if _, ok := pceSvcMapSvcs[s]; !ok {
							update = true
							utils.LogInfo(fmt.Sprintf("csv line(s) %s - %s exists in the CSV but not the PCE. It will be added", strings.Join(intSliceToStrSlice(csvSvc.csvLines), ", "), csvSvcMapSvcs[s]), true)
						}
					}

					for s := range pceSvcMapSvcs {
						if _, ok := csvSvcMapSvcs[s]; !ok {
							update = true
							utils.LogInfo(fmt.Sprintf("csv line(s) %s - %s exists in the PCE but not the CSV. It will be removed", strings.Join(intSliceToStrSlice(csvSvc.csvLines), ", "), pceSvcMapSvcs[s]), true)
						}
					}

					if update {
						updatedServices = append(updatedServices, csvSvc)
					}
				}
			}
		}
	}

	// End run if we have nothing to do
	if len(newServices) == 0 && len(updatedServices) == 0 {
		utils.LogInfo("nothing to be done.", true)

		return
	}

	if !input.UpdatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d services to create and %d services to update. See workloader.log for all identified changes. To do the import, run again using --update-pce flag", len(newServices), len(updatedServices)), true)
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if input.UpdatePCE && !input.NoPrompt {
		var prompt string
		fmt.Printf("[PROMPT] - workloader will create %d services and update %d services in %s (%s). Do you want to run the import (yes/no)? ", len(newServices), len(updatedServices), input.PCE.FriendlyName, viper.Get(input.PCE.FriendlyName+".fqdn").(string))

		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("Prompt denied for creating %d iplists and updating %d iplists.", len(newServices), len(updatedServices)), true)

			return
		}
	}

	// Create new services
	var createdCount, updatedCount, skippedCount int
	provisionableSvcs := []string{}
	for _, newSvc := range newServices {
		svc, a, err := input.PCE.CreateService(newSvc.service)
		utils.LogAPIRespV2("CreateService", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("Ending run - %d services created - %d services Lists updated.", createdCount, updatedCount))
			utils.LogError(err.Error())
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("csv line(s) %s - %s - 406 Not Acceptable - See workloader.log for more details", strings.Join(intSliceToStrSlice(newSvc.csvLines), ", "), newSvc.service.Name), true)
			utils.LogWarning(a.RespBody, false)
			skippedCount++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("csv line(s) %s - %s created - status code %d", strings.Join(intSliceToStrSlice(newSvc.csvLines), ", "), svc.Name, a.StatusCode), true)
			createdCount++
			provisionableSvcs = append(provisionableSvcs, svc.Href)
		}
	}

	// Update Services
	for _, updateSvc := range updatedServices {
		a, err := input.PCE.UpdateService(updateSvc.service)
		utils.LogAPIRespV2("UpdateService", a)
		if err != nil && a.StatusCode != 406 {
			utils.LogError(fmt.Sprintf("Ending run - %d services created - %d services updated.", createdCount, updatedCount))
			utils.LogError(err.Error())
		}
		if a.StatusCode == 406 {
			utils.LogWarning(fmt.Sprintf("csv line(s) %s - %s - 406 Not Acceptable - See workloader.log for more details", strings.Join(intSliceToStrSlice(updateSvc.csvLines), ", "), updateSvc.service.Name), true)
			utils.LogWarning(a.RespBody, false)
			skippedCount++
		}
		if err == nil {
			utils.LogInfo(fmt.Sprintf("csv line(s) %s - %s updated - status code %d", strings.Join(intSliceToStrSlice(updateSvc.csvLines), ", "), updateSvc.service.Name, a.StatusCode), true)
			updatedCount++
			provisionableSvcs = append(provisionableSvcs, updateSvc.service.Href)
		}
	}

	if input.Provision {
		if input.Provision {
			a, err := input.PCE.ProvisionHref(provisionableSvcs, "workloader svc-import")
			utils.LogAPIRespV2("ProvisionHrefs", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			utils.LogInfo(fmt.Sprintf("Provisioning successful - status code %d", a.StatusCode), true)
		}

	}
}
