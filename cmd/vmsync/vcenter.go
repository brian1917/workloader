package vmsync

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
)

// CallWkldImport - Function that gets the data structure to build a wkld import file.
func callWkldImport(keyMap map[string]string, pce *illumioapi.PCE, vmMap map[string]vcenterVM) {

	var outputFileName string
	// Set up the csv headers
	csvData := [][]string{{"hostname", "description"}}
	if umwl {
		csvData[0] = append(csvData[0], "interfaces")
	}
	for _, illumioLabelType := range keyMap {
		csvData[0] = append(csvData[0], illumioLabelType)
	}

	//csvData := [][]string{{"hostname", "role", "app", "env", "loc", "interfaces", "name"}
	for _, vm := range vmMap {
		csvRow := []string{vm.Name, vm.VMID}
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

	if len(vmMap) > 0 {
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
		})

	} else {
		utils.LogInfo("no Vcenter vms found", true)
	}
	// Delete the temp file
	if !keepTempFile {
		if err := os.Remove(outputFileName); err != nil {
			utils.LogWarning(fmt.Sprintf("Could not delete %s", outputFileName), true)
		} else {
			utils.LogInfo(fmt.Sprintf("Deleted %s", outputFileName), false)
		}
	}
	utils.LogEndCommand(fmt.Sprintf("%s-sync", "vcenter"))
}

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
	var response apiResponse

	tmpurl := "/api/cis/tagging/tag/" + tagID
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/tag/id:" + tagID
	}
	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getCategoryDetail URL Parse Failed - %s", err))
	}
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogInfo(fmt.Sprintf("VM Detail access to VCenter failed - %s", err), false)
	}

	//Unmarshal older API response
	if deprecated {
		var tmptag struct {
			Value tagDetail `json:"value"`
		}
		err = json.Unmarshal([]byte(response.RespBody), &tmptag)
		if err != nil {
			utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
		}
		return tmptag.Value
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
	var response apiResponse

	tmpurl := "/api/cis/tagging/tag?action=list-tags-for-category"
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/tag/id:" + categoryID + "?~action=list-tags-for-category"
	}
	tmpbody := []byte{}
	if !deprecated {
		tmpbody, _ = json.Marshal(map[string]string{"category_id": categoryID})
	}
	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getTagFromCategories URL Parse Failed - %s", err))
	}
	response, err = httpCall("POST", apiURL.String(), tmpbody, headers, false)
	if err != nil {
		utils.LogInfo(fmt.Sprintf("Get Tags from Category API failed - %s", err), false)
	}
	//If using older API make sure to use a different object for unmarshaling response
	if deprecated {
		var tagFromCat struct {
			Value []string `json:"value"`
		}
		err = json.Unmarshal([]byte(response.RespBody), &tagFromCat)
		if err != nil {
			utils.LogError(fmt.Sprintf("JSON parsing failed for Get Tags from Category - %s", err))
		}
		return tagFromCat.Value
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
	var response apiResponse

	if object != "datacenter" && object != "cluster" && object != "folder" {
		utils.LogError(fmt.Sprintf("GetObjectID getting invalid object type - %s", object))
	}
	tmpurl := "/api/vcenter/"
	if deprecated {
		tmpurl = "/rest/vcenter/"
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl + object)
	if err != nil {
		utils.LogError(fmt.Sprintf("getCategoryDetail URL Parse Failed - %s", err))
	}
	var escapedParams string
	if deprecated {
		escapedParams = "filter.names=" + filter
	} else {
		escapedParams = "names=" + filter
	}
	apiURL.RawQuery = url.PathEscape(escapedParams)

	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogInfo(fmt.Sprintf("VCenter API call for Objects(datacenter, cluster, folder) failed - %s", err), false)
	}
	if deprecated {
		var obj struct {
			Objects []vcenterObjects `json:"value"`
		}
		err = json.Unmarshal([]byte(response.RespBody), &obj)
		if err != nil {
			utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
		}

		if len(obj.Objects) > 1 {
			utils.LogError(fmt.Sprintf("Get Datacenter ID return more than one answer - %d", len(obj.Objects)))
		}
		return obj.Objects[0]
	}
	var obj []vcenterObjects
	err = json.Unmarshal([]byte(response.RespBody), &obj)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
	}

	if len(obj) > 1 {
		utils.LogError(fmt.Sprintf("Get Datacenter ID return more than one answer - %d", len(obj)))
	}
	return obj[0]
}

// getCategory - Call GetCategory API to get all the APIs in VCenter.
func getCategories(headers [][2]string) []string {
	var response apiResponse
	var cat []string

	tmpurl := "/api/cis/tagging/category"
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/category"
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getCategoryDetail URL Parse Failed - %s", err))
	}
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogError(fmt.Sprintf("Get All Categories access to VCenter failed - %s", err))
	}
	//If using older API make sure to use a different object for unmarshaling response
	if deprecated {
		var cat struct {
			Value []string `json:"value"`
		}
		err = json.Unmarshal([]byte(response.RespBody), &cat)
		if err != nil {
			utils.LogError(fmt.Sprintf("JSON parsing failed for Get All Categories - %s", err))
		}
		return cat.Value
	}

	err = json.Unmarshal([]byte(response.RespBody), &cat)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for Get All Categories - %s", err))
	}
	return cat
}

// getCategoryDetail - Call API with categoryId to get the details about the category.  Specifically pull the
// name of the category which will be used to match with the PCE label types.
func getCategoryDetail(headers [][2]string, categoryid string) categoryDetail {
	var response apiResponse
	var catDetail categoryDetail

	tmpurl := "/api/cis/tagging/category/" + categoryid
	if deprecated {
		tmpurl = "/rest/com/vmware/cis/tagging/category/id:" + categoryid
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl)

	if err != nil {
		utils.LogError(fmt.Sprintf("getCategoryDetail URL Parse Failed - %s", err))
	}
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogInfo(fmt.Sprintf("getCategory API call failed - %s", err), false)
	}

	//If using older API make sure to use a different object for unmarshaling response
	if deprecated {
		var cat struct {
			Value categoryDetail `json:"value"`
		}
		err = json.Unmarshal([]byte(response.RespBody), &cat)
		if err != nil {
			utils.LogError(fmt.Sprintf("JSON parsing failed for getCategoryt - %s", err))
		}
		return cat.Value
	}

	err = json.Unmarshal([]byte(response.RespBody), &catDetail)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for getCategoryt - %s", err))
	}
	return catDetail
}

// getVMDetail - Get specifics about VMs
func getVMNetworkDetail(headers [][2]string, vm string) []Netinterfaces {

	var response apiResponse
	var obj []Netinterfaces

	tmpurl := "/api/vcenter/vm/" + vm + "/guest/networking/interfaces"
	if deprecated {
		tmpurl = "/rest/vcenter/vm/" + vm + "/guest/networking/interfaces"
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl)
	if err != nil {
		utils.LogError(fmt.Sprintf("getVMNetworkDetail URL Parse Failed - %s", err))
	}
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		if response.StatusCode == 503 {
			utils.LogInfo(fmt.Sprintf("getVMNetworkDetail Return 503 - %s - Service Unavailable - No VMTools", vm), false)
		} else {
			utils.LogError(fmt.Sprintf("getVMNetworkDetail API Call failed - %s", err))
		}
	} else {
		if deprecated {
			var obj struct {
				Value []Netinterfaces `json:"value"`
			}
			err = json.Unmarshal([]byte(response.RespBody), &obj)
			if err != nil {
				utils.LogError(fmt.Sprintf("JSON parsing failed for getVMNetworkDetail - %s", err))
			}

			return obj.Value
		}

		err = json.Unmarshal([]byte(response.RespBody), &obj)
		if err != nil {
			utils.LogError(fmt.Sprintf("JSON parsing failed for getVMNetworkDetail - %s", err))
		}
		return obj
	}
	return obj
}

// getAllVMs - VCenter API call to get all the VCenter VMs.  The call will return no more than 4000 objects.
// To make sure you can fit all the VMs into a single call you can use the 'datacenter' and 'cluster' filter.
// Currently only powered on machines are returned.
func getAllVMs(headers [][2]string) []vcenterVM {
	var response apiResponse

	var datacenterId, clusterId, folderId string

	tmpurl := "/api/vcenter/vm/"
	if deprecated {
		tmpurl = "/rest/vcenter/vm"
	}

	apiURL, err := url.Parse("https://" + vcenter + tmpurl)

	params := apiURL.Query()

	state := "POWERED_ON"
	if ignoreState {
		state = ""
	} else {
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

	if err != nil {
		utils.LogError(fmt.Sprintf("getCategoryDetail URL Parse Failed - %s", err))
	}
	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogError(fmt.Sprintf("Sessions Access to VCenter failed - %s", err))
	}
	if deprecated {
		var vms struct {
			Value []vcenterVM `json:"value"`
		}
		err = json.Unmarshal([]byte(response.RespBody), &vms)
		if err != nil {
			utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
		}
		return vms.Value
	}
	var vms []vcenterVM
	err = json.Unmarshal([]byte(response.RespBody), &vms)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
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
			if count < 500 {
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
		if err != nil {
			utils.LogError(fmt.Sprintf("GetTagsFromVMs API call failed - %s", err))
		}

		if deprecated {
			var resObject struct {
				Value []responseObject `json:"value"`
			}
			err = json.Unmarshal([]byte(response.RespBody), &resObject)
			if err != nil {
				utils.LogError(fmt.Sprintf("GetTagFromVM Unmarshall Failed - %s", err))
			}
			totalResObject = append(totalResObject, resObject.Value...)
			return totalResObject
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
	if err != nil {
		utils.LogError(fmt.Sprintf("Sessions Access to VCenter failed - %s", err))
	}
	var rawDep struct {
		Session string `json:"value"`
	}

	if deprecated {
		err = json.Unmarshal([]byte(response.RespBody), &rawDep)
		if err != nil {
			return ""
		}
		return rawDep.Session
	}

	var raw string
	err = json.Unmarshal([]byte(response.RespBody), &raw)
	if err != nil {
		return ""
	}
	return raw

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
	listvms := getAllVMs(httpHeader)

	//******WHY
	tmpWklds := make(map[string]illumioapi.Workload)
	for key, wkldStruct := range pce.Workloads {
		tmpWklds[makeLowerCase(key)] = wkldStruct
	}

	//Cycle through all VMs looking for match if labeling VMs or no match to creat umwls
	vms := make(map[string]vcenterVM)
	for _, tmpvm := range listvms {
		if wkld, ok := tmpWklds[makeLowerCase(tmpvm.Name)]; ok {
			if umwl {
				continue
			}
			if makeLowerCase(nameCheck(*wkld.Hostname)) == makeLowerCase(nameCheck(tmpvm.Name)) {
				vms[tmpvm.VMID] = vcenterVM{Name: *wkld.Hostname, VMID: tmpvm.VMID, PowerState: tmpvm.PowerState}
			}
		}
		if umwl {
			infs := getVMNetworkDetail(httpHeader, tmpvm.VMID)
			if len(infs) == 0 {
				//utils.LogInfo(fmt.Sprintf("UMWL skipped - %s No Network Object returned", tmpvm.Name), false)
			} else {
				var tmpintfs [][]string
				for _, intf := range infs {
					count := 0
					for _, ips := range intf.IP.IPAddresses {
						var tmpintfips []string
						count++
						tmpintfips = []string{fmt.Sprint("eth" + intf.Nic + "-" + fmt.Sprintf("%d", count)), ips.IPAddress}
						tmpintfs = append(tmpintfs, tmpintfips)
					}
				}
				vms[tmpvm.VMID] = vcenterVM{Name: tmpvm.Name, VMID: tmpvm.VMID, PowerState: tmpvm.PowerState, Interfaces: tmpintfs}
			}
		}

	}
	totalVMs := getTagsfromVMs(httpHeader, vms, vcTags)

	//Cycle through all the VMs and add the Tags
	newVMs := make(map[string]vcenterVM)
	for _, object := range totalVMs {
		tmpTags := make(map[string]string)
		for _, tag := range object.TagIds {

			//Check for a tag and to see if you have adont have 2 Tags with the same Category on the same VM
			if _, ok := vcTags[tag]; ok {
				if _, ok := tmpTags[vcTags[tag].LabelType]; ok {
					utils.LogInfo(fmt.Sprintf("VM has 2 or more Tags with the same Category - %s ", vms[object.ObjectId.ID].Name), true)
					continue
				}
				tmpTags[vcTags[tag].LabelType] = vcTags[tag].Tag
			}
		}

		newVMs[object.ObjectId.ID] = vcenterVM{VMID: vms[object.ObjectId.ID].VMID, Name: vms[object.ObjectId.ID].Name, PowerState: vms[object.ObjectId.ID].PowerState, Tags: tmpTags, Interfaces: vms[object.ObjectId.ID].Interfaces}
	}

	utils.LogInfo(fmt.Sprintf("Total VMs found - %d", len(newVMs)), true)
	return newVMs

}
