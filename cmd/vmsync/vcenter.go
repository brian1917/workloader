package vmsync

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
)

// Number of VMs to place in GetTagsFromVMs API call if you have a large number (greater than 500)
const NumVM = 500

// readKeyFile - Reads file that maps TAG names to PCE RAEL labels.   File is added as the first argument.
// the first entry in the CSV should be the VCenter Category.  The second is the PCE label type.
func readKeyFile(filename string) map[string]string {

	keyMap := make(map[string]string)
	// Open CSV File
	file, err := os.Open(filename)
	if err != nil {
		utils.LogError(err.Error())
	}
	defer file.Close()
	reader := csv.NewReader(utils.ClearBOM(bufio.NewReader(file)))

	// Start the counters
	i := 0

	// Iterate through CSV entries
	for {

		// Increment the counter
		i++

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.LogError(err.Error())
		}

		// Skip the header row
		if i == 1 {
			continue
		}
		keyMap[line[0]] = line[1]
	}
	return keyMap
}

// getTagDetail - Get the details of a specific tag by sending the tagID as the filter.
func (vc *VCenter) getTagDetail(tagID string) tagDetail {

	tmpurl := "/api/cis/tagging/tag/" + tagID
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/tag/id:" + tagID
	}

	var obj tagDetail
	vc.Get(tmpurl, nil, false, &obj, "getTagDetail")

	return obj
}

// getTagFromCateegories - Return all the different tagIds for a specific category.
func (vc *VCenter) getTagFromCategories(categoryID string) []string {

	tmpurl := "/api/cis/tagging/tag?action=list-tags-for-category"
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/tag/id:" + categoryID + "?~action=list-tags-for-category"
	}

	var obj []string
	vc.Post(tmpurl, map[string]string{"category_id": categoryID}, &obj, false, "getTagFromCategories")

	return obj
}

// getObjectID - This will call the VCenter API to get the objectID of a Datacenter or Cluster or folder.  These objectID
// are used as filters when getting all the VMs.
func (vc *VCenter) getObjectID(object string, filter []string) []string /*vcenterObjects */ {

	if object != "datacenter" && object != "cluster" && object != "folder" && object != "parent_folders" {
		utils.LogError(fmt.Sprintf("GetObjectID getting invalid object type - %s", object))
	}

	//parent_folder still calls 'folder' API so need to change the object to folder if running parent_folders object
	queryParam := make(map[string][]string)
	if deprecated {
		if object == "parent_folders" {
			object = "folder"
			queryParam["filter.parent_folders"] = filter
		} else {
			queryParam["filter.names"] = filter

		}
	} else {
		if object == "parent_folders" {
			object = "folder"
			queryParam["parent_folders"] = filter
		} else {
			queryParam["names"] = filter
		}
	}

	tmpurl := "/api/vcenter/" + object
	if deprecated {
		tmpurl = "/rest/vcenter/" + object
	}

	//vc.Get will pull objects like datacenter, cluster or folders from VCenter.   Build an array of the VCenter object id to be used in VM get query
	var objs []vcenterObjects
	vc.Get(tmpurl, queryParam, false, &objs, "getObjectID")
	var tmpObj []string
	for _, obj := range objs {
		switch object {
		case "datacenter":
			tmpObj = append(tmpObj, obj.Datacenter)
		case "cluster":
			tmpObj = append(tmpObj, obj.Cluster)
		case "folder":
			tmpObj = append(tmpObj, obj.Folder)
		}
	}

	/* if len(objs) == 0 {
		utils.LogError(fmt.Sprintf("Error Get Vcenter \"%s\" object \"%s\" returned %d entries.  Check for correctness. ", object, filter, len(objs)))
	}*/
	//return objs[0]
	return tmpObj
}

// getCategory - Call GetCategory API to get all the APIs in VCenter.
func (vc *VCenter) getCategories() {

	tmpurl := "/api/cis/tagging/category"
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/category"
	}

	vc.Get(tmpurl, nil, false, &vc.Categories, "getCategories")
}

// getCategoryDetail - Call API with categoryId to get the details about the category.  Specifically pull the
// name of the category which will be used to match with the PCE label types.
func (vc *VCenter) getCategoryDetail(categoryid string) categoryDetail {

	tmpurl := "/api/cis/tagging/category/" + categoryid
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/category/id:" + categoryid
	}
	var obj categoryDetail
	vc.Get(tmpurl, nil, false, &obj, "getCategoryDetail")

	return obj
}

// getVMNetworkDetail - Get specifics about VMs network interfaces - requires VMtools
func (vc *VCenter) getVMNetworkDetail(vm string) []Netinterfaces {

	tmpurl := "/api/vcenter/vm/" + vm + "/guest/networking/interfaces"
	if deprecated {
		tmpurl = "/rest/vcenter/vm/" + vm + "/guest/networking/interfaces"
	}
	var obj []Netinterfaces
	vc.Get(tmpurl, nil, false, &obj, "getVMNetworkDetail")

	return obj

}

// getVMIdentity - Get specifics about VMs like IP, Hostname and OS Family - requires VMtools
func (vc *VCenter) getVMIdentity(vm string) VMIdentity {

	tmpurl := "/api/vcenter/vm/" + vm + "/guest/identity"
	if deprecated {
		tmpurl = "/rest/vcenter/vm/" + vm + "/guest/identity"
	}
	var obj VMIdentity
	vc.Get(tmpurl, nil, false, &obj, "getVMIdentity")

	return obj
}

// getVCenterVMs - VCenter API call to get all the VCenter VMs.  The call will return no more than 4000 objects.
// To make sure you can fit all the VMs into a single call you can use the 'datacenter' and 'cluster' filter.
// Currently only powered on machines are returned.
func (vc *VCenter) getVCenterVMs() {

	tmpurl := "/api/vcenter/vm"
	if deprecated {
		tmpurl = "/rest/vcenter/vm"
	}

	queryParam := make(map[string][]string)

	if !ignoreState {
		if deprecated {
			queryParam["filter.power_states"] = []string{"POWERED_ON"}
		} else {
			queryParam["power_states"] = []string{"POWERED_ON"}
		}
	}

	if datacenter != "" {
		object := vc.getObjectID("datacenter", []string{datacenter})
		if deprecated {
			queryParam["filter.datacenters"] = object
		} else {
			queryParam["datacenters"] = object
		}
	}

	if cluster != "" {
		object := vc.getObjectID("cluster", []string{cluster})
		if deprecated {
			queryParam["filter.clusters"] = object
		} else {
			queryParam["clusters"] = object
		}
	}

	//When filtering on folder you can enter multiple folders in the CLI seperating via a comma.  Additionally sub folders
	//will be discovered.  The endless for loop will walk all folders until there are no more sub folders.
	if folder != "" {
		objectId := vc.getObjectID("folder", strings.Split(folder, ","))
		tmpObjectIds := objectId
		for {
			subFolderIds := vc.getObjectID("parent_folders", objectId)
			objectId = subFolderIds
			if len(subFolderIds) == 0 {
				break
			}
			tmpObjectIds = append(tmpObjectIds, subFolderIds...)
		}

		if deprecated {
			queryParam["filter.folders"] = tmpObjectIds
		} else {
			queryParam["folders"] = tmpObjectIds
		}
	}
	vc.Get(tmpurl, queryParam, false, &vc.VCVMSlice, "getVCenterVMs")

}

// getTagsfromVMs - Function that will get all VMs that have a certain tag.
func (vc *VCenter) getTagsfromVMs(vms map[string]vcenterVM, tags map[string]vcenterTags) []responseObject {

	tmpurl := "/api/cis/tagging/tag-association?action=list-attached-tags-on-objects"
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/tag-association?~action=list-attached-tags-on-objects"
	}

	var totalResObject []responseObject

	//loop through all the VMs and make a JSON object with no more than NumVM (const = 500) to send to get the Tags for each VM.
	var tmpvm []objects
	var count = 0
	for vmid := range vms {
		tmpvm = append(tmpvm, objects{Type: "VirtualMachine", ID: vmid})
		count++
		if count != len(vms) {
			if count < NumVM {
				continue
			}
		}
		//Build request body with all the VMs you want to get Tags for.
		tmpvms := requestObject{ObjectId: tmpvm}

		var obj []responseObject
		vc.Post(tmpurl, tmpvms, &obj, true, "getTagsfromVMs")

		totalResObject = append(totalResObject, obj...)
	}

	return totalResObject
}

// getSessionToken - Function to call VCenter login API so that the session token can be captures.
func (vc *VCenter) getSessionToken() string {

	tmpurl := "/api/session"
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/session"
	}
	var obj string
	vc.Post(tmpurl, nil, &obj, true, "getSessionToken")
	return obj

}

// getVCenterVersion - Gets the version of VCenter running so we can make sure to correctly build the VCenter APIs
// After 7.0.u2 there is new syntax for the api.
// pre 7.0.u2 - https:<vcenter>/rest/com/vmware/cis/<tag APIs> and https:<vcenter>/rest/vcenter/vm<VM APIs>
// post 7.0.u2 - https:<vcenter>/api/vcenter/<All APIs>
func (vc *VCenter) getVCenterVersion() {

	tmpurl := "/api/appliance/system/version"

	vc.Get(tmpurl, nil, false, &vc.VCVersion, "getVCenterVersion")
	//Validate HTTP Get
	// utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{"getVCenterVersion": response})
	// if err != nil {
	// 	utils.LogError(fmt.Sprintf("marshal validateVCenterVersion response failed - %s", err))
	// }
	utils.LogInfo(fmt.Sprintf("The current version of VCenter is %s", vc.VCVersion.Version), false)
	if ver := strings.Split(vc.VCVersion.Version, "."); ver[0] == "6" || (ver[0] == "7" && (ver[2] == "1" || ver[2] == "2")) {
		utils.LogError("Currently this feature only support VCenter above '7.0.u2'.")
	}
}

// setupVCenterSession -
func (vc *VCenter) setupVCenterSession() {

	if vc.User == "" || vc.Secret == "" {
		utils.LogError("Either USER or/and SECRET are empty.  Both are required.")
	}

	//Ignore SSL Certs
	if vc.DisableTLSChecking {
		utils.LogInfo(("Ignoring SSL certificates via --insecure option"), false)
	}
	//Call the VCenter API to get the session token
	vc.Header["Content-Type"] = "application/json"
	vc.Header["vmware-api-session-id"] = vc.getSessionToken()

	//Get if VCenter is 7.0.u2 or older
	if !deprecated {
		vc.getVCenterVersion()
	}
}

// buildVCTagMap - Call the VCenter APIs to build a list of Tags and their category.  These will be used when finding all the VMs
// that will be discovered based on the filters and options used.
func (vc *VCenter) buildVCTagMap(keyMap map[string]string) {

	//Get all VCenter Categories
	utils.LogInfo("Call Get Category VCenter API - ", false)
	vc.getCategories()

	vc.VCTags = make(map[string]vcenterTags)
	//Cycle through all the categories storing those categories that map to a PCE label type
	//For any category that has a PCE label type get all the tags (aka labels) for that category(aka label type)
	//VCenter API stores categories and tags as UUID without human readable data.  You much get the Category or Tag
	//Detail to find that.  That is what getCategoryDetail and getTagDetail are doing.
	for _, category := range vc.Categories {
		catDetail := vc.getCategoryDetail(category)
		if _, ok := keyMap[catDetail.Name]; ok {
			tagIDS := vc.getTagFromCategories(catDetail.ID)

			for _, tagid := range tagIDS {
				taginfo := vc.getTagDetail(tagid)
				vc.VCTags[tagid] = vcenterTags{Category: catDetail.Name, LabelType: keyMap[catDetail.Name], CategoryID: catDetail.ID, Tag: taginfo.Name}
			}
		}
	}
}

// validateKeyMap - Check the KepMap file so it has correct Category to LabelType mapping.  Exit if not correct.
func validateKeyMap(keyMap map[string]string, pce *illumioapi.PCE) {

	needLabelDimensions := false
	if pce.Version.Major > 22 || (pce.Version.Major == 22 && pce.Version.Minor >= 5) && len(pce.LabelDimensionsSlice) == 0 {
		needLabelDimensions = true
	}

	//Call PCE load data to get all the machines.
	apiResps, err := pce.Load(illumioapi.LoadInput{Workloads: true, Labels: true, LabelDimensions: needLabelDimensions}, utils.UseMulti())
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Create a map of label keys and depending on version either populate with API or with role, app, env, and loc.
	labelKeysMap := make(map[string]bool)
	if pce.Version.Major > 22 || (pce.Version.Major == 22 && pce.Version.Minor >= 5) {
		for _, l := range pce.LabelDimensionsSlice {
			labelKeysMap[l.Key] = true
		}
	} else {
		labelKeysMap["role"] = true
		labelKeysMap["app"] = true
		labelKeysMap["env"] = true
		labelKeysMap["loc"] = true
	}
	utils.LogInfo(fmt.Sprintf("label keys map: %v", labelKeysMap), false)

	for _, val := range keyMap {
		if !labelKeysMap[val] {
			utils.LogError(fmt.Sprintf("Following PCE LabelType '%s' is not configured on the PCE", val))
		}
	}
}

// isIPv6 - CHecks to see if the IP address provided is Ipv6. returns true if yes
func isIPv6(ip string) bool {
	return strings.Count(ip, ":") >= 2

}

// buildWkldImport - Function that gets the data structure to build a wkld import file and import.
func buildWkldImport(pce *illumioapi.PCE) {

	var outputFileName string
	// Set up the csv headers
	csvData := [][]string{{"hostname", "description"}}
	if umwl {
		csvData[0] = append(csvData[0], "interfaces")
	}
	for _, illumioLabelType := range vc.KeyMap {
		csvData[0] = append(csvData[0], illumioLabelType)
	}

	//csvData := [][]string{{"hostname", "role", "app", "env", "loc", "interfaces", "name"}
	for _, vm := range vc.VCVMs {
		csvRow := []string{vm.Name, vm.VMID + " - " + "VCenterName = " + vm.VCName}
		var tmpInf string
		if umwl {
			for c, inf := range vm.Interfaces {
				if c != 0 {
					tmpInf = tmpInf + ";"
				}
				tmpInf = tmpInf + fmt.Sprintf("%s:%s", inf[0], inf[1])
			}
			csvRow = append(csvRow, tmpInf)
		}
		for index, header := range csvData[0] {

			// Skip hostname and interfaces if umwls ...they are statically added above
			if index < 2 {
				continue
			} else if umwl && index == 2 {
				continue
			}
			//process hostname by finding Name TAG
			csvRow = append(csvRow, vm.Tags[header])
		}
		csvData = append(csvData, csvRow)
	}

	if len(vc.VCVMs) <= 0 {
		utils.LogInfo("No Vcenter VMs found", true)
	} else {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-vcenter-sync-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, nil, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d VCenter vms with label data exported", len(csvData)-1), true)

		utils.LogInfo("passing output into wkld-import...", true)

		wkldimport.ImportWkldsFromCSV(wkldimport.Input{
			PCE:             *pce,
			ImportFile:      outputFileName,
			RemoveValue:     "vcenter-label-delete",
			Umwl:            umwl,
			UpdateWorkloads: true,
			UpdatePCE:       updatePCE,
			NoPrompt:        noPrompt,
			MaxUpdate:       maxUpdate,
			MaxCreate:       maxCreate,
		})

		// Delete the temp file
		if !keepFile {
			if err := os.Remove(outputFileName); err != nil {
				utils.LogWarning(fmt.Sprintf("Could not delete %s", outputFileName), true)
			} else {
				utils.LogInfo(fmt.Sprintf("Deleted %s", outputFileName), false)
			}
		}
	}
}

// compileVMData - Function that will pull categories, tags, and vms.  These will map to PCE labeltypes, labels and workloads.
// The function will find all the tags for each vm that is either running a VEN or desired all machines that are not running a VEN.
// The output will of the function will be easily imported buy the workload wkld.import feature.
func (vc *VCenter) compileVMData(keyMap map[string]string) {

	//Get all the PCE data
	pce, err := utils.GetTargetPCEV2(false)
	if err != nil {
		utils.LogError(fmt.Sprintf("Error getting PCE - %s", err.Error()))
	}

	//Make sure the keyMap file doesnt have incorrect labeltypes.  Exit if it does.
	validateKeyMap(keyMap, &pce)

	//return totaltags
	vc.getVCenterVMs()

	//Have to build a map of PCE wklds with all the names lowercase
	tmpWklds := make(map[string]illumioapi.Workload)
	for key, wkldStruct := range pce.Workloads {
		tmpWklds[strings.ToLower(nameCheck(key))] = wkldStruct
	}

	//Cycle through all VMs looking for match if labeling VMs or no match to creat umwls
	vc.VCVMs = make(map[string]vcenterVM)
	//variable to store the correct name

	for _, tmpvm := range vc.VCVMSlice {

		var tmpintfs [][]string
		count := 0
		vmTools := true

		tmpvm.IPs = make(map[string]bool)
		//Search VMTools Hostname for existing PCE workload or use VCenter Name to match
		tmpvm.VCName = tmpvm.Name
		vmIdentity := vc.getVMIdentity(tmpvm.VMID)
		//If not hostname then VMtools not installed.  Ignore Hostname if using vcenter Name as match.
		if name := vmIdentity.HostName; name != "" {
			if !vcName {
				tmpvm.Name = name
			}
			//IP address is found with getVMIdentity so add it to the map to make sure its only added 1 time to this machine.
			if vmIdentity.IPAddress != "" {
				count++
				tmpvm.IPs[vmIdentity.IPAddress] = true
				tmpintfips := []string{fmt.Sprint("eth" + fmt.Sprintf("%d", count)), vmIdentity.IPAddress}
				tmpintfs = [][]string{tmpintfips}
			}
		} else {
			vmTools = false
		}

		if wkld, ok := tmpWklds[strings.ToLower(nameCheck(tmpvm.Name))]; ok {
			if !umwl {
				vc.VCVMs[tmpvm.VMID] = vcenterVM{VCName: tmpvm.VCName, Name: *wkld.Hostname, VMID: tmpvm.VMID, PowerState: tmpvm.PowerState}
			}
			continue
		}

		if umwl {

			if allIPs {
				//If you the VMIdentity API doesnt provide data then no VMtools - Skip getting NetworkDetails.
				if vmTools {
					tmpvm.VMInterfaces = vc.getVMNetworkDetail(tmpvm.VMID)
				}
				if len(tmpintfs) == 0 && len(tmpvm.VMInterfaces) == 0 {
					continue
				}
				for _, intf := range tmpvm.VMInterfaces {
					//increment eth for more interfaces
					count++
					//VMware will provide the same IP multiple times but PCE doesnt like that.  Only get unique IPs
					for _, ips := range intf.IP.IPAddresses {
						if isIPv6(ips.IPAddress) && !ipv6 {
							// bydefault skip all IPv6 address unless added as an option
							continue
						}
						if ok := tmpvm.IPs[ips.IPAddress]; ok {
							continue
						} else {
							tmpvm.IPs[ips.IPAddress] = true
						}

						tmpintfips := []string{fmt.Sprint("eth" + fmt.Sprintf("%d", count)), ips.IPAddress}
						tmpintfs = append(tmpintfs, tmpintfips)
					}
				}

			}

			//Make sure there is an is an interface that has an IP otherwise VM should not be added as an UWM
			if len(tmpintfs) == 0 {
				continue
			}
			vc.VCVMs[tmpvm.VMID] = vcenterVM{VCName: tmpvm.VCName, Name: tmpvm.Name, VMID: tmpvm.VMID, PowerState: tmpvm.PowerState, Interfaces: tmpintfs}
		}
	}

	//After getting all the VMs build a keymap for all the Categories matched in the keyMap to be used for labeling
	vc.buildVCTagMap(keyMap)

	//Get all the Tags for VMs that were found above.
	totalVMs := vc.getTagsfromVMs(vc.VCVMs, vc.VCTags)

	//Cycle through all the VMs that returned with tags and add the Tags that are importable.  All other VMs will not have Tags.
	count := 0
	for _, object := range totalVMs {
		tmpTags := make(map[string]string)
		//Variable to store if VM has Illumio Labels or not
		found := false
		for _, tag := range object.TagIds {

			//Check for a tag and to see if you have adont have 2 Tags with the same Category on the same VM

			if _, ok := vc.VCTags[tag]; ok {
				found = true
				if _, ok := tmpTags[vc.VCTags[tag].LabelType]; ok {
					utils.LogInfo(fmt.Sprintf("VM has 2 or more Tags with the same Category - %s ", vc.VCVMs[object.ObjectId.ID].Name), true)
					continue
				}
				tmpTags[vc.VCTags[tag].LabelType] = vc.VCTags[tag].Tag
			}
			//If we VM has Illumio Labels count this VM.

		}

		//count up all the VMs and total labels.
		if found {
			count++
		}
		vc.VCVMs[object.ObjectId.ID] = vcenterVM{VMID: vc.VCVMs[object.ObjectId.ID].VMID, VCName: vc.VCVMs[object.ObjectId.ID].VCName, Name: vc.VCVMs[object.ObjectId.ID].Name, PowerState: vc.VCVMs[object.ObjectId.ID].PowerState, Tags: tmpTags, Interfaces: vc.VCVMs[object.ObjectId.ID].Interfaces}
	}

	utils.LogInfo(fmt.Sprintf("Total VMs found - %d.  Total VMs with Illumio Labels - %d", len(vc.VCVMs), count), true)

	//Build call wkld-Import using the VMs and the tags found in VCenter.
	buildWkldImport(&pce)
}
