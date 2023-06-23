package vmsync

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/viper"
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
func getTagDetail(headers [][2]string, tagID string) tagDetail {

	tmpurl := "/api/cis/tagging/tag/" + tagID
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/tag/id:" + tagID
	}
	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getTagDetail URL Parse Failed - %s", err))
	}

	var response illumioapi.APIResponse
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{"getTagDetail": response})
	if err != nil {
		utils.LogInfo(fmt.Sprintf("VM Detail access to VCenter failed - %s", err), false)
	}

	//Unmarshal using new API response
	var tmptag tagDetail
	err = json.Unmarshal([]byte(response.RespBody), &tmptag)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
	}
	return tmptag
}

// getTagFromCateegories - Return all the different tagIds for a specific category.
func getTagFromCategories(headers [][2]string, categoryID string) []string {

	tmpurl := "/api/cis/tagging/tag?action=list-tags-for-category"
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/tag/id:" + categoryID + "?~action=list-tags-for-category"
	}
	tmpbody := []byte{}
	if !deprecated {
		tmpbody, err = json.Marshal(map[string]string{"category_id": categoryID})
		if err != nil {
			utils.LogError(fmt.Sprintf("getTagFromCategories Marshal failed - %s", err))
		}
	}
	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getTagFromCategories URL Parse Failed - %s", err))
	}

	var response illumioapi.APIResponse
	response, err = httpCall("POST", apiURL.String(), tmpbody, headers, false)
	utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{"getTagFromCategories": response})
	if err != nil {
		utils.LogInfo(fmt.Sprintf("Get Tags from Category API failed - %s", err), false)
	}

	var tagFromCat []string
	err = json.Unmarshal([]byte(response.RespBody), &tagFromCat)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for Get Tags from Category - %s", err))
	}
	return tagFromCat
}

// getObjectID - This will call the VCenter API to get the objectID of a Datacenter or Cluster.  These objectID
// are used as filters when getting all the VMs.
func getObjectID(headers [][2]string, object, filter string) vcenterObjects {

	if object != "datacenter" && object != "cluster" && object != "folder" {
		utils.LogError(fmt.Sprintf("GetObjectID getting invalid object type - %s", object))
	}
	tmpurl := "/api/vcenter/"
	if deprecated {
		tmpurl = "/rest/vcenter/"
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl + object)
	if err != nil {
		utils.LogError(fmt.Sprintf("getAllVMs URL Parse Failed - %s", err))
	}
	var escapedParams string
	if deprecated {
		escapedParams = "filter.names=" + filter
	} else {
		escapedParams = "names=" + filter
	}
	apiURL.RawQuery = url.PathEscape(escapedParams)

	var response illumioapi.APIResponse
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{"getObjectID": response})
	if err != nil {
		utils.LogInfo(fmt.Sprintf("VCenter API call for Objects(datacenter, cluster, folder) failed - %s", err), false)
	}

	var obj []vcenterObjects
	err = json.Unmarshal([]byte(response.RespBody), &obj)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
	}

	if len(obj) > 1 {
		utils.LogError(fmt.Sprintf("Get Vcenter Objects return more than one answer - %d", len(obj)))
	}
	return obj[0]
}

// getCategory - Call GetCategory API to get all the APIs in VCenter.
func getCategories(headers [][2]string) []string {

	tmpurl := "/api/cis/tagging/category"
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/category"
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getCategories URL Parse Failed - %s", err))
	}

	var response illumioapi.APIResponse
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{"getCategories": response})
	if err != nil {
		utils.LogError(fmt.Sprintf("Get All Categories access to VCenter failed - %s", err))
	}

	var cat []string
	err = json.Unmarshal([]byte(response.RespBody), &cat)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for Get All Categories - %s", err))
	}
	return cat
}

// getCategoryDetail - Call API with categoryId to get the details about the category.  Specifically pull the
// name of the category which will be used to match with the PCE label types.
func getCategoryDetail(headers [][2]string, categoryid string) categoryDetail {

	tmpurl := "/api/cis/tagging/category/" + categoryid
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/category/id:" + categoryid
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl)

	if err != nil {
		utils.LogError(fmt.Sprintf("getCategoryDetail URL Parse Failed - %s", err))
	}

	var response illumioapi.APIResponse
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{"getCategoryDetail": response})
	if err != nil {
		utils.LogInfo(fmt.Sprintf("getCategory API call failed - %s", err), false)
	}

	var catDetail categoryDetail
	err = json.Unmarshal([]byte(response.RespBody), &catDetail)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for getCategoryt - %s", err))
	}
	return catDetail
}

// getVMDetail - Get specifics about VMs
func getVMNetworkDetail(headers [][2]string, vm string) []Netinterfaces {

	tmpurl := "/api/vcenter/vm/" + vm + "/guest/networking/interfaces"
	if deprecated {
		tmpurl = "/rest/vcenter/vm/" + vm + "/guest/networking/interfaces"
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getVMNetworkDetail URL Parse Failed - %s", err))
	}

	var obj []Netinterfaces
	var response illumioapi.APIResponse
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	//Check to see if the response says you dont have VMware Tools installed
	if response.StatusCode == 503 && strings.Contains(response.RespBody, "VMware Tools") && !viper.Get("debug").(bool) {
		return obj
	}
	utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{"getVMNetworkDetail": response})
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for getVMNetworkDetail - %s", err))
	}
	err = json.Unmarshal([]byte(response.RespBody), &obj)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for getVMNetworkDetail - %s", err))
	}
	return obj

}

// getAllVMs - VCenter API call to get all the VCenter VMs.  The call will return no more than 4000 objects.
// To make sure you can fit all the VMs into a single call you can use the 'datacenter' and 'cluster' filter.
// Currently only powered on machines are returned.
func getVCenterVMs(headers [][2]string) []vcenterVM {

	var datacenterId, clusterId, folderId string

	tmpurl := "/api/vcenter/vm/"
	if deprecated {
		tmpurl = "/rest/vcenter/vm"
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getAllVMs URL Parse Failed - %s", err))
	}
	params := apiURL.Query()

	//Add filter parameters for getting the VMs
	state := "POWERED_ON"
	if !ignoreState {
		if deprecated {
			params.Set("filter.power_states", state)
		} else {
			params.Set("power_states", state)
		}
	}

	if datacenter != "" {
		object := getObjectID(headers, "datacenter", datacenter)
		datacenterId = object.Datacenter
		if deprecated {
			params.Set("filter.datacenters", datacenterId)
		} else {
			params.Set("datacenters", datacenterId)
		}
	}

	if cluster != "" {
		object := getObjectID(headers, "cluster", cluster)
		clusterId = object.Cluster
		if deprecated {
			params.Set("filter.clusters", clusterId)
		} else {
			params.Set("clusters", clusterId)
		}
	}

	if folder != "" {
		object := getObjectID(headers, "folder", folder)
		folderId = object.Folder
		if deprecated {
			params.Set("filter.folders", folderId)
		} else {
			params.Set("folders", folderId)
		}
	}

	apiURL.RawQuery = params.Encode()

	var response illumioapi.APIResponse
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{"getVcenterVMs": response})
	if err != nil {
		utils.LogError(fmt.Sprintf("getAllVMs to VCenter failed - %s", err))
	}

	var vms []vcenterVM
	err = json.Unmarshal([]byte(response.RespBody), &vms)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for getAllVMs - %s", err))
	}
	return vms
}

// GetVMsFromTags - Function that will get all VMs that have a certain tag.
func getTagsfromVMs(headers [][2]string, vms map[string]vcenterVM, tags map[string]vcenterTags) []responseObject {

	tmpurl := "/api/cis/tagging/tag-association?action=list-attached-tags-on-objects"
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/tag-association?~action=list-attached-tags-on-objects"
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getTagsFromVMs URL Parse Failed - %s", err))
	}

	var totalResObject []responseObject

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
		body, err := json.Marshal(tmpvms)
		if err != nil {
			utils.LogError(fmt.Sprintf("GetTagsFromVms Marshal failed - %s", err))
		}

		//Send Request for tags per VM.
		response, err := httpCall("POST", apiURL.String(), body, headers, false)
		utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{"getTagsfromVMs": response})
		if err != nil {
			utils.LogError(fmt.Sprintf("GetTagsFromVMs API call failed - %s", err))
		}

		//Build Response object to store returned tags for each VM sent in the request.
		var resObject []responseObject
		err = json.Unmarshal([]byte(response.RespBody), &resObject)
		if err != nil {
			utils.LogError(fmt.Sprintf("GetTagFromVM Unmarshall Failed - %s", err))
		}
		totalResObject = append(totalResObject, resObject...)
	}

	return totalResObject
}

// getSessionToken - Function to call VCenter login API so that the session token can be captures.
func getSessionToken() string {

	tmpurl := "/api/session"
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/session"
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getSessionToken URL Parse Failed - %s", err))
	}
	response, err := httpCall("POST", apiURL.String(), []byte{}, nil, true)
	utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{"getSessionToken": response})
	if err != nil {
		utils.LogError(fmt.Sprintf("Sessions Access to VCenter failed - %s", err))
	}

	var session string
	err = json.Unmarshal([]byte(response.RespBody), &session)
	if err != nil {
		return ""
	}
	return session

} // validateKeyMap - Check the KepMap file so it has correct Category to LabelType mapping.  Exit if not correct.
func validateKeyMap(keyMap map[string]string) {

	// Get the PCE version
	version, api, err := pce.GetVersion()
	utils.LogAPIRespV2("GetVersion", api)
	if err != nil {
		utils.LogError(err.Error())
	}

	needLabelDimensions := false
	if (version.Major > 22 || (version.Major == 22 && version.Minor >= 5)) && len(pce.LabelDimensionsSlice) == 0 {
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
	if version.Major > 22 || (version.Major == 22 && version.Minor >= 5) {
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

// vcenterGetPCEInputData - Function that will pull categories, tags, and vms.  These will map to PCE labeltypes, labels and workloads.
// The function will find all the tags for each vm that is either running a VEN or desired all machines that are not running a VEN.
// The output will of the function will be easily imported buy the workload wkld.import feature.
func vcenterBuildPCEInputData(keyMap map[string]string) map[string]vcenterVM {

	if userID == "" || secret == "" {
		utils.LogError("Either USER or/and SECRET are empty.  Both are required.")
	}

	//Call the VCenter API to get the session token
	httpHeader := [][2]string{{"Content-Type", "application/json"}, {"vmware-api-session-id", getSessionToken()}}

	//Get if VCenter is 7.0.u2 or older
	if !deprecated {
		validateVCenterVersion(httpHeader)

	}

	//Get all VCenter Categories
	utils.LogInfo("Call Get Category VCenter API - ", false)
	categories := getCategories(httpHeader)

	var vcTags = make(map[string]vcenterTags)
	//var totalTags []string

	//Cycle through all the categories storing those categories that map to a PCE label type
	//For any category that has a PCE label type get all the tags (aka labels) for that category(aka label type)
	//VCenter API stores categories and tags as UUID without human readable data.  You much get the Category or Tag
	//Detail to find that.  That is what getCategoryDetail and getTagDetail are doing.
	for _, category := range categories {
		catDetail := getCategoryDetail(httpHeader, category)
		if _, ok := keyMap[catDetail.Name]; ok {
			tagIDS := getTagFromCategories(httpHeader, catDetail.ID)

			for _, tagid := range tagIDS {
				taginfo := getTagDetail(httpHeader, tagid)
				vcTags[tagid] = vcenterTags{Category: catDetail.Name, LabelType: keyMap[catDetail.Name], CategoryID: catDetail.ID, Tag: taginfo.Name}
			}
		}
	}

	//return totaltags
	//var allvms = make(map[string]vmwareDetail)
	listvms := getVCenterVMs(httpHeader)

	//******WHY
	tmpWklds := make(map[string]illumioapi.Workload)
	for key, wkldStruct := range pce.Workloads {
		tmpWklds[strings.ToLower(key)] = wkldStruct
	}

	//Cycle through all VMs looking for match if labeling VMs or no match to creat umwls
	vms := make(map[string]vcenterVM)
	for _, tmpvm := range listvms {
		if wkld, ok := tmpWklds[strings.ToLower(tmpvm.Name)]; ok {
			if umwl {
				continue
			}
			if strings.EqualFold(nameCheck(*wkld.Hostname), (nameCheck(tmpvm.Name))) {
				vms[tmpvm.VMID] = vcenterVM{Name: *wkld.Hostname, VMID: tmpvm.VMID, PowerState: tmpvm.PowerState}
			}
		}
		if umwl {

			//get all the interfaces from the VM if VM tools installed
			infs := getVMNetworkDetail(httpHeader, tmpvm.VMID)
			var tmpintfs [][]string
			count := 0
			for _, intf := range infs {
				count++
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
				vms[tmpvm.VMID] = vcenterVM{Name: tmpvm.Name, VMID: tmpvm.VMID, PowerState: tmpvm.PowerState, Interfaces: tmpintfs}
			}
		}

	}
	totalVMs := getTagsfromVMs(httpHeader, vms, vcTags)

	//Cycle through all the VMs that returned with tags and add the Tags that are importable.  All other VMs will not have Tags.
	count := 0
	for _, object := range totalVMs {
		tmpTags := make(map[string]string)
		//Variable to store if VM has Illumio Labels or not
		found := false
		for _, tag := range object.TagIds {

			//Check for a tag and to see if you have adont have 2 Tags with the same Category on the same VM

			if _, ok := vcTags[tag]; ok {
				found = true
				if _, ok := tmpTags[vcTags[tag].LabelType]; ok {
					utils.LogInfo(fmt.Sprintf("VM has 2 or more Tags with the same Category - %s ", vms[object.ObjectId.ID].Name), true)
					continue
				}
				tmpTags[vcTags[tag].LabelType] = vcTags[tag].Tag
			}
			//If we VM has Illumio Labels count this VM.

		}
		if found {
			count++
		}
		vms[object.ObjectId.ID] = vcenterVM{VMID: vms[object.ObjectId.ID].VMID, Name: vms[object.ObjectId.ID].Name, PowerState: vms[object.ObjectId.ID].PowerState, Tags: tmpTags, Interfaces: vms[object.ObjectId.ID].Interfaces}

	}

	utils.LogInfo(fmt.Sprintf("Total VMs found - %d.  Total VMs with Illumio Labels - %d", len(vms), count), true)
	return vms

}
