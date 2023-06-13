package vmsync

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var vcenter, datacenter, cluster, folder, userID, secret string

var csvFile string
var ignoreState, umwl, updatePCE, noPrompt, keepTempFile, ignoreFQDNHostname, keepAllPCEInterfaces, deprecated bool
var pce illumioapi.PCE
var err error

// Init builds the commands
func init() {

	// Disable sorting
	cobra.EnableCommandSorting = false

	//awsimport options
	VCenterSyncCmd.Flags().StringVarP(&datacenter, "datacenter", "d", "", "Sync VMs that reside in certain VCenter Datacenter object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&cluster, "cluster", "c", "", "Sync VMs that reside in certain VCenter cluster object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&folder, "folder", "f", "", "Sync VMs that reside in certain VCenter folder object. (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&vcenter, "vcenter", "v", "", "Required - FQDN or IP of VCenter instance - e.g vcenter.illumio.com")
	VCenterSyncCmd.Flags().StringVarP(&userID, "user", "u", "", "Required - username of account with access to VCenter REST API")
	VCenterSyncCmd.Flags().StringVarP(&secret, "password", "p", "", "Required - password of account with access to VCenter REST API")
	VCenterSyncCmd.Flags().BoolVarP(&ignoreState, "ignore-state", "", false, "By default only looks for workloads in a running state")
	VCenterSyncCmd.Flags().BoolVar(&umwl, "umwl", false, "Create unmanaged workloads for non-matches.")
	VCenterSyncCmd.Flags().BoolVarP(&keepTempFile, "keep-temp-file", "k", false, "Do not delete the temp CSV file downloaded from SerivceNow.")
	VCenterSyncCmd.Flags().BoolVarP(&ignoreFQDNHostname, "ignore-name-clean", "i", false, "Convert FQDN hostnames reported by Illumio VEN to short hostnames by removing everything after first period (e.g., test.domain.com becomes test). ")
	VCenterSyncCmd.Flags().BoolVarP(&keepAllPCEInterfaces, "keep-all-pce-interfaces", "s", false, "Will not delete an interface on an unmanaged workload if it's not in the import. It will only add interfaces to the workload.")
	VCenterSyncCmd.Flags().BoolVarP(&deprecated, "deprecated", "", false, "use this option if you are running an older version of the API (VCenter 6.5-7.0.u2")
	VCenterSyncCmd.MarkFlagRequired("userID")
	VCenterSyncCmd.MarkFlagRequired("secret")
	VCenterSyncCmd.MarkFlagRequired("vcenter")
	VCenterSyncCmd.Flags().SortFlags = false

}

// VCenterSyncCmd checks if the keyfilename is entered.
var VCenterSyncCmd = &cobra.Command{
	Use:   "vmsync",
	Short: "Integrate Azure VMs into PCE.",
	Long: `Sync VCenter VM Tags with PCE workload Labels.  The command requires a CSV file that maps VCenter Categories to PCE label types.
	There are options to filter the VMs from VCenter using VCenter objects(datacenter, clusters, folders, power state).  PCE hostnames and VM names
	are used to match PCE workloads to VCenter VMs.   There is an option to remove a FQDN hostname doamin to match with the VM name in VCenter
	
	There is also an UMWL option to find all VMs that are not running the Illumio VEN.  Any VCenter VM no matching a PCE workload will
	be considered as an UMWL.  To correctly configure the IP address for these UWML VMTools should be installed to pull that data from the 
	API.`,

	Run: func(cmd *cobra.Command, args []string) {

		//Get all the PCE data
		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(fmt.Sprintf("Error getting PCE - %s", err.Error()))
		}
		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		csvFile = args[0]

		// Get the debug value from viper
		// debug = viper.Get("debug").(bool)
		// updatePCE = viper.Get("update_pce").(bool)
		// noPrompt = viper.Get("no_prompt").(bool)

		utils.LogStartCommand("vcenter-sync")

		//load keymapfile, This file will have the Catagories to Label Type mapping
		keyMap := readKeyFile(csvFile)

		//Sync VMs to Workloads or create UMWL VMs for all machines in VCenter not running VEN
		callWkldImport(vcenterTagInfo(keyMap))
	},
}

func callWkldImport(vmMap map[string][]string) {

	var outputFileName string

	csvData := [][]string{{"hostname", "role", "app", "env", "loc", "interfaces", "name"}}

	// for _, instance := range vmMap {

	// 	ipdata := []string{}
	// 	for num, intf := range instance.Interfaces {
	// 		if intf.PublicIP != "" {
	// 			ipdata = append(ipdata, fmt.Sprintf("eth%d:%s", num, intf.PublicIP))
	// 		}
	// 		for _, ips := range intf.PrivateIP {
	// 			ipdata = append(ipdata, fmt.Sprintf("eth%d:%s", num, ips))
	// 		}
	// 	}
	// 	csvData = append(csvData, []string{instance.Name, instance.Tags["role"], instance.Tags["app"], instance.Tags["env"], instance.Tags["loc"], strings.Join(ipdata, ";"), instance.Name})
	// }

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-%s-rawdata-%s.csv", "vcenter", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d workloads exported", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no cloud instances found for export.", true)
	}

	wkldimport.ImportWkldsFromCSV(wkldimport.Input{
		PCE:             pce,
		ImportFile:      outputFileName,
		RemoveValue:     "gcp-label-delete",
		Umwl:            false,
		UpdateWorkloads: true,
		UpdatePCE:       updatePCE,
		NoPrompt:        noPrompt,
	})

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

	if deprecated {
		var value struct {
			Tags []string `json:"value"`
		}
		err = json.Unmarshal([]byte(response.RespBody), &value)
		if err != nil {
			utils.LogError(fmt.Sprintf("JSON parsing failed for Get Tags from Category - %s", err))
		}
		return value.Tags
	}
	var tags []string
	err = json.Unmarshal([]byte(response.RespBody), &tags)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for Get Tags from Category - %s", err))
	}
	return tags
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
		escapedParams = "names=" + "Jeff Schmitz"
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
		utils.LogInfo(fmt.Sprintf("getVMNetworkDetail API Call failed - %s", err), false)
	}
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

// getAllVMs - VCenter API call to get all the VCenter VMs.  The call will return no more than 4000 objects.
// To make sure you can fit all the VMs into a single call you can use the 'datacenter' and 'cluster' filter.
// Currently only powered on machines are returned.
func getAllVMs(headers [][2]string) []vmwareVM {
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
			Value []vmwareVM `json:"value"`
		}
		err = json.Unmarshal([]byte(response.RespBody), &vms)
		if err != nil {
			utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
		}
		return vms.Value
	}
	var vms []vmwareVM
	err = json.Unmarshal([]byte(response.RespBody), &vms)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
	}
	return vms
}

// GetVMsFromTags - Function that will get all VMs that have a certain tag.
func getTagsfromVMs(headers [][2]string, vms map[string]vmwareVM, tags map[string]vcenterTags) []responseObject {

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

		tmpvm := append(tmpvm, objects{Type: "VirtualMachine", ID: vmid})

		count++
		if count != len(vms) {
			if count < 1 {
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
			utils.LogInfo(fmt.Sprintf("GetTagsFromVMs API call failed - %s", err), false)
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
}

// httpCall - Generic Function to call VCenter APIs
func httpCall(httpAction, apiURL string, body []byte, headers [][2]string, login bool) (apiResponse, error) {

	var response apiResponse
	var httpBody *bytes.Buffer
	//var asyncResults asyncResults

	// Validate the provided action
	httpAction = strings.ToUpper(httpAction)
	if httpAction != "GET" && httpAction != "POST" && httpAction != "PUT" && httpAction != "DELETE" {
		return response, errors.New("invalid http action string. action must be GET, POST, PUT, or DELETE")
	}

	// Get the base URL
	//	u, err := url.Parse(apiURL)

	// Create body
	httpBody = bytes.NewBuffer(body)

	// Create HTTP client and request
	client := &http.Client{}
	if pce.DisableTLSChecking {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	req, err := http.NewRequest(httpAction, apiURL, httpBody)
	if err != nil {
		return response, err
	}

	// Set basic authentication and headers
	if login {
		req.SetBasicAuth(userID, secret)
	}

	// Set the user provided headers
	for _, h := range headers {
		req.Header.Set(h[0], h[1])
	}

	// Make HTTP Request
	resp, err := client.Do(req)
	if err != nil {
		return response, err
	}

	// Process response
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	// Put relevant response info into struct
	response.RespBody = string(data)
	response.StatusCode = resp.StatusCode
	response.Header = resp.Header
	response.Request = resp.Request

	// Check for a 200 response code
	if strconv.Itoa(resp.StatusCode)[0:1] != "2" {
		return response, errors.New("http status code of " + strconv.Itoa(response.StatusCode))
	}

	// Return data and nil error
	return response, nil
}

// getVCenterVersion - Gets the version of VCenter running so we can make sure to correctly build the VCenter APIs
// After 7.0.u2 there is new syntax for the api.
// pre 7.0.u2 - https:<vcenter>/rest/com/vmware/cis/<tag APIs> and https:<vcenter>/rest/vcenter/vm<VM APIs>
// post 7.0.u2 - https:<vcenter>/api/vcenter/<All APIs>
func validateVCenterVersion(headers [][2]string) {

	apiURL, err := url.Parse(fmt.Sprintf("https://%s/api/appliance/system/version", vcenter))
	if err != nil {
		utils.LogError(fmt.Sprintf("validateVCenterVersio URL Parse Failed - %s", err))
	}
	response, err := httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogError(fmt.Sprintf("validateVCenterVersio API call failed - %s", err))
	}

	//vmware version json response.
	var raw struct {
		Build       string `json:"build"`
		InstallTime string `json:"install_time"`
		Product     string `json:"product"`
		Releasedate string `json:"releasedate"`
		Summary     string `json:"summary"`
		Type        string `json:"type"`
		Version     string `json:"version"`
	}

	err = json.Unmarshal([]byte(response.RespBody), &raw)
	if err != nil {
		utils.LogError(fmt.Sprintf("marshal validateVCenterVersion response failed - %s", err))
	}
	utils.LogInfo(fmt.Sprintf("The current version of VCenter is %s", raw.Version), false)
	if ver := strings.Split(raw.Version, "."); (ver[0] == "7" && ver[2] == "u2" || ver[2] == "u1") || ver[0] == "6" {
		utils.LogError("Currently this feature only support VCenter '7.0.u2' and above")
	}
}

func vcenterTagInfo(keyMap map[string]string) map[string][]string {

	utils.LogInfo("Begin VCenter API Session setup - ", false)
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
		if keyMap[catDetail.Name] != "" {
			tagIDS := getTagFromCategories(httpHeader, catDetail.ID)

			for _, tagid := range tagIDS {
				taginfo := getTagDetail(httpHeader, tagid)
				vcTags[tagid] = vcenterTags{Category: keyMap[catDetail.Name], CategoryID: catDetail.ID, Tag: taginfo.Name}
			}
			//totalTags = append(totalTags, tagIDS...)
		}
	}

	//return totaltags
	var allvms = make(map[string]vmwareDetail)
	listvms := getAllVMs(httpHeader)

	// Get the PCE version
	version, api, err := pce.GetVersion()
	utils.LogAPIRespV2("GetVersion", api)
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

	tmpWklds := make(map[string]illumioapi.Workload)
	for key, wkldStruct := range pce.Workloads {
		tmpWklds[makeLowerCase(key)] = wkldStruct
	}

	//Cycle through all VMs looking for match if labeling VMs or no match to creat umwls
	vms := make(map[string]vmwareVM)
	for _, tmpvm := range listvms {
		if wkld, ok := tmpWklds[makeLowerCase(tmpvm.Name)]; ok {
			if umwl {
				continue
			}
			if makeLowerCase(nameCheck(*wkld.Hostname)) == makeLowerCase(nameCheck(tmpvm.Name)) {
				vms[tmpvm.VMID] = vmwareVM{Name: tmpvm.Name, VMID: tmpvm.VMID, PowerState: tmpvm.PowerState}
			}
		}
		if umwl {

			tmp := getVMNetworkDetail(httpHeader, tmpvm.VMID)
			for _, inf := range tmp {
				fmt.Println(inf.IP.IPAddresses, inf.Nic)
			}

			vms[tmpvm.VMID] = vmwareVM{Name: tmpvm.Name, VMID: tmpvm.VMID, PowerState: tmpvm.PowerState}
			// allvms[tmpvm.VMID] = cloudData{VMID: tmpvm.VMID, Name: tmpvm.Name, State: tmpvm.PowerState, Location: tmplocation, Interfaces: []netInterface{tmpintf}}
		}

	}
	totalVMs := getTagsfromVMs(httpHeader, vms, vcTags)
	vmMap := make(map[string][]string)
	for _, vm := range totalVMs {
		vmMap[vm.ObjectId.ID] = vm.TagIds
	}

	//Cycle through all the reservations for all arrays in that reservation

	utils.LogInfo(fmt.Sprintf("Total EC2 instances discovered - %d", len(allvms)), true)
	return vmMap

}

// makeLowerCase - Take any string and make it all lowercase
func makeLowerCase(str string) string {
	return strings.ToLower(str)
}

// nameCheck - Match Hostname with or without domain information
func nameCheck(name string) string {

	if ignoreFQDNHostname {
		fullname := strings.Split(name, ".")
		return fullname[0]
	}
	return name
}
