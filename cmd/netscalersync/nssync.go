package netscalersync

import (
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/ns"
	"github.com/brian1917/workloader/utils"
	"github.com/google/uuid"
	"github.com/spf13/viper"
)

func nsSync(pce illumioapi.PCE, netscaler ns.NetScaler) {

	// Get all the Virtual Services in Illumio
	pceVirtualServices, api, err := pce.GetVirtualServices(nil, "draft")
	utils.LogAPIResp("GetVirtualServices", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	utils.LogInfo(fmt.Sprintf("get illumio virtual services - %d", api.StatusCode), true)

	// Get Illumio unmanaged workloads from the external dataset
	pceUMWLs, api, err := pce.GetWklds(map[string]string{"managed": "false", "external_data_set": externalDataSet})
	utils.LogAPIResp("GetWklds", api)
	if err != nil {
		log.Fatal(err.Error())
	}
	utils.LogInfo(fmt.Sprintf("get illumio unmanaged workloads - %d", api.StatusCode), true)

	// Get the NetScaler virtual servers
	nsVirtualServers, nsAPI, err := netscaler.GetVirtualServers()
	if err != nil {
		utils.LogError(err.Error())
	}
	nsVirtualServerMap := make(map[string]ns.VirtualServer)
	utils.LogInfo(fmt.Sprintf("get netscaler virtual servers - %d", nsAPI.StatusCode), true)

	// Get the NetScaler SNAT IPs
	nsSnatIPs, nsAPI, err := netscaler.GetNSIP()
	if err != nil {
		utils.LogError(err.Error())
	}
	nsSNIPMap := make(map[string]ns.NSIP)
	utils.LogInfo(fmt.Sprintf("get netscaler snat ips - %d", nsAPI.StatusCode), true)

	// Create slices for create and updates
	var createVirtualServices, updateVirtualServices, removeVirtualServices []illumioapi.VirtualService
	var createUMWLs, updateUMWLs, removeUMWLs []illumioapi.Workload

	// Iterate through each netscaler virtual services
	fmt.Println()
	utils.LogInfo("processing netscaler virtual servers...", true)
	for _, nsvs := range nsVirtualServers {
		// Add it to the map
		nsVirtualServerMap[nsvs.Name] = nsvs
		if virtualService, exists := pce.VirtualServices[nsvs.Name]; exists {
			// If it exists, first check if it's managed by workloader
			if virtualService.ExternalDataSet != externalDataSet {
				utils.LogWarning(fmt.Sprintf("%s exists in the pce with an external datast of %s. workloader is managing %s. skipping.", virtualService.Name, virtualService.ExternalDataSet, externalDataSet), true)
				continue
			}
			// Check to see if we have to update it.
			update := false
			msgSlice := []string{}
			// Check IP address
			if nsvs.Ipv46 != virtualService.IPOverrides[0] {
				update = true
				msgSlice = append(msgSlice, fmt.Sprintf("ip address to be updated from %s to %s", virtualService.IPOverrides[0], nsvs.Ipv46))
			}
			// Check Port
			if nsvs.Port != virtualService.ServicePorts[0].Port {
				update = true
				msgSlice = append(msgSlice, fmt.Sprintf("port to be updated from %d to %d", virtualService.ServicePorts[0].Port, nsvs.Port))
			}
			// Log the pending update, edit the virtual service, and append to the update list
			if update {
				utils.LogInfo(fmt.Sprintf("%s exists but requires updates - %s", nsvs.Name, strings.Join(msgSlice, ". ")), true)
				virtualService.Service = getService(nsvs.Port, nsvs.Servicetype)
				virtualService.ServicePorts = getServicePorts(nsvs.Port, nsvs.Servicetype)
				virtualService.IPOverrides = []string{nsvs.Ipv46}
				updateVirtualServices = append(updateVirtualServices, virtualService)
			} else {
				utils.LogInfo(fmt.Sprintf("%s already exists and requires no changes", virtualService.Name), true)
			}
		} else {
			// Create the virtual service if it has a port, IP, and name
			if nsvs.Name == "" || nsvs.Port == 0 || nsvs.Ipv46 == "" {
				utils.LogWarning(fmt.Sprintf("%s - does not have a name, ip, and/or port. skipping", nsvs.Name), true)
				continue
			}
			// Log the pending create and append
			utils.LogInfo(fmt.Sprintf("%s to be created - port: %d - ip: %s", nsvs.Name, nsvs.Port, nsvs.Ipv46), true)
			createVirtualServices = append(createVirtualServices, illumioapi.VirtualService{Name: nsvs.Name, IPOverrides: []string{nsvs.Ipv46}, Service: getService(nsvs.Port, nsvs.Servicetype), ServicePorts: getServicePorts(nsvs.Port, nsvs.Servicetype), ExternalDataSet: "workloader-netscaler-sync", ExternalDataReference: uuid.New().String()})
		}
	}

	// Iterate through each snat IP
	fmt.Println()
	utils.LogInfo("processing netscaler SNAT IPs...", true)
	for _, nsSnatIP := range nsSnatIPs {
		// Skip if it's not a SNAT IP
		if strings.ToUpper(nsSnatIP.Type) != "SNIP" {
			continue
		}
		hostname := fmt.Sprintf("%s-snat", snipName(nsSnatIP.Ipaddress, nsSnatIP.Netmask))
		nsSNIPMap[hostname] = nsSnatIP
		if wkld, exists := pce.Workloads[hostname]; exists {
			// If it exists, check if it needs to be updated
			if wkld.Interfaces[0].Address != nsSnatIP.Ipaddress {
				utils.LogInfo(fmt.Sprintf("%s exists but requires updates - ip to change from %s to %s", hostname, wkld.Interfaces[0].Address, nsSnatIP.Ipaddress), true)
				wkld.Interfaces = []*illumioapi.Interface{{Address: nsSnatIP.Ipaddress, Name: "umwl0"}}
				updateUMWLs = append(updateUMWLs, wkld)
			}
		} else {
			// If it does not exist, create the workload
			utils.LogInfo(fmt.Sprintf("%s to be created - ip: %s", hostname, nsSnatIP.Ipaddress), true)
			createUMWLs = append(createUMWLs, illumioapi.Workload{Hostname: hostname, Interfaces: []*illumioapi.Interface{{Address: nsSnatIP.Ipaddress, Name: "umwl0"}}, ExternalDataSet: utils.StrToPtr("workloader-netscaler-sync"), ExternalDataReference: utils.StrToPtr(uuid.New().String())})
		}
	}

	// Check the PCE virtual services that should be removed.
	fmt.Println()
	utils.LogInfo("processing pce virtual services that should be removed because virtual server no longer exists...", true)
	for _, pceVS := range pceVirtualServices {
		// Only process if it's in the external dataset
		if pceVS.ExternalDataSet != externalDataSet {
			continue
		}
		// If the VS name doesn't exist in the PCE, get it ready for removal.
		if _, exists := nsVirtualServerMap[pceVS.Name]; !exists {
			utils.LogInfo(fmt.Sprintf("%s - %s - to be deleted", pceVS.Name, pceVS.Href), true)
			removeVirtualServices = append(removeVirtualServices, pceVS)

		}
	}

	// Check for UMWLs that should be removed
	fmt.Println()
	utils.LogInfo("processing pce unmanaged workloads that should be removed because SNIP no longer exists...", true)
	for _, pceUMWL := range pceUMWLs {
		// Only process if it's in the external dataset
		if utils.PtrToStr(pceUMWL.ExternalDataSet) != externalDataSet {
			continue
		}
		// If the UMWL name doesn't exist in the PCE, get it ready for removal.
		if _, exists := nsSNIPMap[pceUMWL.Hostname]; !exists {
			utils.LogInfo(fmt.Sprintf("%s - %s - to be deleted", pceUMWL.Hostname, pceUMWL.Href), true)
			removeUMWLs = append(removeUMWLs, pceUMWL)
		}
	}
	fmt.Println()

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo("See workloader.log for more details. To do the import, run again using --update-pce flag.", true)

		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("\r\n%s [PROMPT] - workloader will create %d virtual services (vips), create %d unmanaged workloads (snips), update %d virtual services (vips), update %d unmanaged workloads (snips), remove %d virtual services (vips), and remove %d unmanaged workloads (snips) in %s (%s). do you want to run the import (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), len(createVirtualServices), len(createUMWLs), len(updateVirtualServices), len(updateUMWLs), len(removeVirtualServices), len(removeUMWLs), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied.", true)

			return
		}
	}

	provisionHrefs := []string{}

	// Create the virtual services
	for _, vs := range createVirtualServices {
		newVS, api, _ := pce.CreateVirtualService(vs)
		utils.LogAPIResp("CreateVirutalService", api)
		if api.StatusCode > 200 && api.StatusCode < 300 {
			utils.LogInfo(fmt.Sprintf("created %s - %s", newVS.Name, newVS.Href), true)
			provisionHrefs = append(provisionHrefs, newVS.Href)
		} else {
			utils.LogWarning(fmt.Sprintf("error creating %s - %d status code - %s", vs.Name, api.StatusCode, api.RespBody), true)
		}
	}

	// Create the unmanaged workloads
	for _, wkld := range createUMWLs {
		newWkld, api, _ := pce.CreateWkld(wkld)
		utils.LogAPIResp("CreateWkld", api)
		if api.StatusCode > 200 && api.StatusCode < 300 {
			utils.LogInfo(fmt.Sprintf("created %s - %s", newWkld.Hostname, newWkld.Href), true)
		} else {
			utils.LogWarning(fmt.Sprintf("error creating %s - %d status code - %s", wkld.Hostname, api.StatusCode, api.RespBody), true)
		}
	}

	// Update the virtual services
	for _, vs := range updateVirtualServices {
		api, _ := pce.UpdateVirtualService(vs)
		utils.LogAPIResp("UpdateVirtualService", api)
		if api.StatusCode > 200 && api.StatusCode < 300 {
			utils.LogInfo(fmt.Sprintf("update %s - %s", vs.Name, vs.Href), true)
			provisionHrefs = append(provisionHrefs, vs.Href)
		} else {
			utils.LogWarning(fmt.Sprintf("error updating %s - %d status code - %s", vs.Name, api.StatusCode, api.RespBody), true)
		}
	}

	// Update the workloads
	for _, wkld := range updateUMWLs {
		api, _ := pce.UpdateWkld(wkld)
		utils.LogAPIResp("UpdateWkld", api)
		if api.StatusCode > 200 && api.StatusCode < 300 {
			utils.LogInfo(fmt.Sprintf("update %s - %s", wkld.Hostname, wkld.Href), true)
		} else {
			utils.LogWarning(fmt.Sprintf("error updating %s - %d status code - %s", wkld.Hostname, api.StatusCode, api.RespBody), true)
		}
	}

	// Delete virtual services
	for _, vs := range removeVirtualServices {
		api, _ := pce.DeleteHref(vs.Href)
		utils.LogAPIResp("DeleteHref", api)
		if api.StatusCode > 200 && api.StatusCode < 300 {
			utils.LogInfo(fmt.Sprintf("delete %s - %s", vs.Name, vs.Href), true)
			provisionHrefs = append(provisionHrefs, vs.Href)
		} else {
			utils.LogWarning(fmt.Sprintf("error updating %s - %d status code - %s", vs.Name, api.StatusCode, api.RespBody), true)
		}
	}

	// Delete unmanaged workloads
	for _, wkld := range removeUMWLs {
		api, _ := pce.DeleteHref(wkld.Href)
		utils.LogAPIResp("DeleteHref", api)
		if api.StatusCode > 200 && api.StatusCode < 300 {
			utils.LogInfo(fmt.Sprintf("delete %s - %s", wkld.Hostname, wkld.Href), true)
		} else {
			utils.LogWarning(fmt.Sprintf("error updating %s - %d status code - %s", wkld.Hostname, api.StatusCode, api.RespBody), true)
		}
	}

	// Provision changes to Virtual Services
	api, err = pce.ProvisionHref(provisionHrefs, "workloader netscaler-sync")
	utils.LogAPIResp("ProvisionHref", api)
	if err != nil {
		utils.LogError(err.Error())
	}
	utils.LogInfo(fmt.Sprintf("provisioning virtual service changes - %d", api.StatusCode), true)

}

func snipName(ip, mask string) string {
	addr := net.ParseIP(mask).To4()

	sz, _ := net.IPv4Mask(addr[0], addr[1], addr[2], addr[3]).Size()

	_, ipNet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip, sz))
	if err != nil {
		fmt.Printf("creating snip name - %s", err.Error())
	}

	return ipNet.String()
}

// getServices takes the port and serviceType reported by Netscaler
// and returns the Service that should be attached to the PCE's virtual service
func getService(port int, serviceType string) *illumioapi.Service {
	// Use All Services if the port is 65535 or service type is "any"
	if port == 65535 && serviceType == "any" {
		services, api, err := pce.GetServices(map[string]string{"name": "All Services"}, "draft")
		utils.LogAPIResp("GetServices", api)
		if err != nil {
			utils.LogError(err.Error())
		}
		return &illumioapi.Service{Href: services[0].Href}
	}

	// Service will be nil if not using Any Service
	return nil

}

// getServicePorts takes the port and serviceType reported by Netscaler
// and returns the ServicePorts that should be attached to the PCE's virtual service
func getServicePorts(port int, serviceType string) []*illumioapi.ServicePort {
	// No service ports defined if port is 65535 or serviceType is any since using All Services object
	if port == 65535 && serviceType == "any" {
		return nil
	}

	// Otherwise, use the port and protocol
	proto := 6
	if strings.ToLower(serviceType) == "udp" {
		proto = 17
	}
	return []*illumioapi.ServicePort{{Port: port, Protocol: proto}}
}
