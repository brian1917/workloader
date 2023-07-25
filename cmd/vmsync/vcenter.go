package vmsync

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/brian1917/illumioapi/v2"
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
func (vc *VCenter) getObjectID(object, filter string) vcenterObjects {

	if object != "datacenter" && object != "cluster" && object != "folder" {
		utils.LogError(fmt.Sprintf("GetObjectID getting invalid object type - %s", object))
	}
	tmpurl := "/api/vcenter/"
	if deprecated {
		tmpurl = "/rest/vcenter/"
	}

	queryParam := make(map[string]string)
	if deprecated {
		queryParam["filter.names"] = filter
	} else {
		queryParam["names="] = filter
	}

	var obj []vcenterObjects
	vc.Get(tmpurl, queryParam, false, &obj, "getObjectID")

	if len(obj) > 1 {
		utils.LogError(fmt.Sprintf("Get Vcenter Objects return more than one answer - %d", len(obj)))
	}
	return obj[0]
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

	tmpurl := "/api/vcenter/vm/"
	if deprecated {
		tmpurl = "/rest/vcenter/vm"
	}

	queryParam := make(map[string]string)
	if !ignoreState {
		if deprecated {
			queryParam["filter.power_states"] = "POWERED_ON"
		} else {
			queryParam["power_states"] = "POWERED_ON"
		}
	}

	if datacenter != "" {
		object := vc.getObjectID("datacenter", datacenter)
		if deprecated {
			queryParam["filter.datacenters"] = object.Datacenter
		} else {
			queryParam["datacenters"] = object.Datacenter
		}
	}

	if cluster != "" {
		object := vc.getObjectID("cluster", cluster)
		if deprecated {
			queryParam["filter.cluster"] = object.Cluster
		} else {
			queryParam["cluster"] = object.Cluster
		}
	}

	if folder != "" {
		object := vc.getObjectID("folder", folder)
		if deprecated {
			queryParam["filter.cluster"] = object.Folder
		} else {
			queryParam["cluster"] = object.Folder
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
func validateKeyMap(keyMap map[string]string) {

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

// compileVMData - Function that will pull categories, tags, and vms.  These will map to PCE labeltypes, labels and workloads.
// The function will find all the tags for each vm that is either running a VEN or desired all machines that are not running a VEN.
// The output will of the function will be easily imported buy the workload wkld.import feature.
func (vc *VCenter) compileVMData() {

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
		var tmpName string
		//Search VMTools Hostname for existing PCE workload or use VCenter Name to match
		tmpName = tmpvm.Name
		if !vcName {
			tmpvm.VMDetail = vc.getVMIdentity(tmpvm.VMID)
			if name := tmpvm.VMDetail.HostName; name != "" {
				tmpName = name
			}
		}

		if wkld, ok := tmpWklds[strings.ToLower(nameCheck(tmpName))]; ok {
			if !umwl {
				vc.VCVMs[tmpvm.VMID] = vcenterVM{Name: *wkld.Hostname, VMID: tmpvm.VMID, PowerState: tmpvm.PowerState, VMDetail: tmpvm.VMDetail}
			}
			continue
		}

		if umwl {
			var tmpintfs [][]string
			if allIPs {
				tmpvm.VMInterfaces = vc.getVMNetworkDetail(tmpvm.VMID)
				if len(tmpvm.VMInterfaces) <= 0 {
					continue
				}
				count := 0
				for _, intf := range tmpvm.VMInterfaces {
					count++
					//VMware will provide the same IP multiple times but PCE doesnt like that.  Only get unique IPs
					uniqueIP := make(map[string]bool)
					for _, ips := range intf.IP.IPAddresses {
						if ok := uniqueIP[ips.IPAddress]; ok {
							continue
						} else {
							uniqueIP[ips.IPAddress] = true
						}
						tmpintfips := []string{fmt.Sprint("eth" + fmt.Sprintf("%d", count)), ips.IPAddress}
						tmpintfs = append(tmpintfs, tmpintfips)
					}
				}

			} else {
				//If we are using VCenter Name to match no VMtools was called earlier.  Call to get an IP if available

				if vcName {
					tmpvm.VMDetail = vc.getVMIdentity(tmpvm.VMID)
				}

				//Make sure there is an IP to add otherwise skip to next VM.  You need an IP for an UWML
				if tmpvm.VMDetail.IPAddress != "" {
					tmpintfs = [][]string{{"eth0", tmpvm.VMDetail.IPAddress}}
				} else {
					continue
				}
			}

			vc.VCVMs[tmpvm.VMID] = vcenterVM{Name: tmpName, VMID: tmpvm.VMID, PowerState: tmpvm.PowerState, Interfaces: tmpintfs, VMDetail: tmpvm.VMDetail}

		}

	}
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
		if found {
			count++
		}
		vc.VCVMs[object.ObjectId.ID] = vcenterVM{VMID: vc.VCVMs[object.ObjectId.ID].VMID, Name: vc.VCVMs[object.ObjectId.ID].Name, PowerState: vc.VCVMs[object.ObjectId.ID].PowerState, Tags: tmpTags, Interfaces: vc.VCVMs[object.ObjectId.ID].Interfaces}

	}

	utils.LogInfo(fmt.Sprintf("Total VMs found - %d.  Total VMs with Illumio Labels - %d", len(vc.VCVMs), count), true)
}
