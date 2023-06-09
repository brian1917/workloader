package vcenter

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

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/cmd/wkldimport"
	"github.com/brian1917/workloader/utils"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var vcenter, datacenter, cluster, credsFile, region, userID, secret, token, rg string

var csvFile string
var ignoreState, umwl, ignorePublic, updatePCE, noPrompt, keepTempFile, fqdnToHostname, keepAllPCEInterfaces, debug bool
var pce illumioapi.PCE
var err error

// Init builds the commands
func init() {

	// Disable sorting
	cobra.EnableCommandSorting = false

	//awsimport options
	VCenterSyncCmd.Flags().StringVarP(&datacenter, "datacenter", "", "", "VCenter Datacenter that will be used to sync data with the PCE (default - \"\"")
	//VCenterSyncCmd.Flags().StringVarP(&folder, "folder", "", "", "VCenter Datacenter that will be used to sync data with the PCE (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&cluster, "cluster", "", "", "VCenter Datacenter that will be used to sync data with the PCE (default - \"\"")
	VCenterSyncCmd.Flags().StringVarP(&vcenter, "vcenter", "c", "", "Required - FQDN or IP of VCenter instance - e.g vcenter.illumio.com")
	VCenterSyncCmd.Flags().StringVarP(&userID, "user", "u", "", "Required - username of account with access to VCenter REST API")
	VCenterSyncCmd.Flags().StringVarP(&secret, "password", "p", "", "Required - password of account with access to VCenter REST API")
	VCenterSyncCmd.Flags().BoolVarP(&ignoreState, "ignore-state", "", false, "By default only looks for workloads in a running state")
	VCenterSyncCmd.Flags().BoolVar(&umwl, "umwl", false, "Create unmanaged workloads for non-matches.")
	VCenterSyncCmd.Flags().BoolVarP(&keepTempFile, "keep-temp-file", "k", false, "Do not delete the temp CSV file downloaded from SerivceNow.")
	VCenterSyncCmd.Flags().BoolVarP(&fqdnToHostname, "fqdn-to-hostname", "f", false, "Convert FQDN hostnames reported by Illumio VEN to short hostnames by removing everything after first period (e.g., test.domain.com becomes test). ")
	VCenterSyncCmd.Flags().BoolVarP(&keepAllPCEInterfaces, "keep-all-pce-interfaces", "s", false, "Will not delete an interface on an unmanaged workload if it's not in the import. It will only add interfaces to the workload.")
	VCenterSyncCmd.MarkFlagRequired("userID")
	VCenterSyncCmd.MarkFlagRequired("secret")
	VCenterSyncCmd.MarkFlagRequired("vcenter")
	VCenterSyncCmd.Flags().SortFlags = false

}

// VCenterSyncCmd checks if the keyfilename is entered.
var VCenterSyncCmd = &cobra.Command{
	Use:   "vcenter",
	Short: "Integrate Azure VMs into PCE.",
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
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
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		utils.LogStartCommand("vcenter-sync")

		//load keymapfile, pull data from Azure, use that data to import into PCE.
		keyMap := readKeyFile(csvFile)
		callWkldImport("vcenter", vcenterHTTP(keyMap))
	},
}

// read-KeyFile - Reads file that maps TAG names to PCE RAEL labels.   File is added as the first argument.
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

func callWkldImport(cloudName string, allVMs map[string]cloudData) {
	var outputFileName string

	csvData := [][]string{{"hostname", "role", "app", "env", "loc", "interfaces", "name"}}

	for _, instance := range allVMs {

		ipdata := []string{}
		for num, intf := range instance.Interfaces {
			if intf.PublicIP != "" {
				ipdata = append(ipdata, fmt.Sprintf("eth%d:%s", num, intf.PublicIP))
			}
			for _, ips := range intf.PrivateIP {
				ipdata = append(ipdata, fmt.Sprintf("eth%d:%s", num, ips))
			}
		}
		csvData = append(csvData, []string{instance.Name, instance.Tags["role"], instance.Tags["app"], instance.Tags["env"], instance.Tags["loc"], strings.Join(ipdata, ";"), instance.Name})
	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-%s-rawdata-%s.csv", cloudName, time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d workloads exported", len(csvData)-1), true)
	} else {
		// Log command execution for 0 results
		utils.LogInfo("no cloud instances found for export.", true)
	}

	wkldimport.ImportWkldsFromCSV(wkldimport.Input{
		PCE:             *pce,
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
	utils.LogEndCommand(fmt.Sprintf("%s-sync", cloudName))
}

func getCategoryDetail(headers [][2]string, categoryid string) categoryDetail {
	var response apiResponse
	var cat struct {
		Value categoryDetail `json:"value"`
	}
	apiURL, err := url.Parse("https://" + vcenter + "/rest/com/vmware/cis/tagging/category/id:" + categoryid)

	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogInfo(fmt.Sprintf("VM Detail access to VCenter failed - %s", err), false)
	}
	err = json.Unmarshal([]byte(response.RespBody), &cat)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
	}
	return cat.Value
}

func getTagDetail(headers [][2]string, tagID string) tagDetail {
	var response apiResponse
	var tmptag struct {
		Value tagDetail `json:"value"`
	}
	apiURL, err := url.Parse("https://" + vcenter + "/rest/com/vmware/cis/tagging/tag/id:" + tagID)

	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogInfo(fmt.Sprintf("VM Detail access to VCenter failed - %s", err), false)
	}

	err = json.Unmarshal([]byte(response.RespBody), &tmptag)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
	}
	return tmptag.Value
}

func getTagFromCategories(headers [][2]string, categoryID string) []string {
	var response apiResponse
	var cat struct {
		Value []string `json:"value"`
	}
	apiURL, err := url.Parse("https://" + vcenter + "/rest/com/vmware/cis/tagging/tag/id:" + categoryID + "?~action=list-tags-for-category")

	response, err = httpCall("POST", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogInfo(fmt.Sprintf("VM Detail access to VCenter failed - %s", err), false)
	}

	err = json.Unmarshal([]byte(response.RespBody), &cat)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
	}
	return cat.Value
}

func getObjectID(headers [][2]string, object, filter string) vcenterObjects {
	var response apiResponse
	var obj struct {
		Value []vcenterObjects `json:"value"`
	}
	if object != "datacenter" && object != "cluster" {
		utils.LogError(fmt.Sprintf("GetObjectID getting invalid object type - %s", object))
	}
	apiURL, err := url.Parse("https://" + vcenter + "/rest/vcenter/" + object + "?filter.names=" + filter)

	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogInfo(fmt.Sprintf("VM Detail access to VCenter failed - %s", err), false)
	}

	err = json.Unmarshal([]byte(response.RespBody), &obj)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
	}

	if len(obj.Value) > 1 {
		utils.LogError(fmt.Sprintf("Get Datacenter ID return more than one answer - %d", len(obj.Value)))
	}
	return obj.Value[0]
}

func getCategories(headers [][2]string) []string {
	var response apiResponse
	var cat struct {
		Value []string `json:"value"`
	}
	apiURL, err := url.Parse("https://" + vcenter + "/rest/com/vmware/cis/tagging/category")

	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogInfo(fmt.Sprintf("VM Detail access to VCenter failed - %s", err), false)
	}

	err = json.Unmarshal([]byte(response.RespBody), &cat)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
	}
	return cat.Value
}

func getVMDetail(headers [][2]string, vm string) vmwareDetail {
	var response apiResponse
	var vms struct {
		Value vmwareDetail `json:"value"`
	}

	vms.Value.Found = false
	apiURL, err := url.Parse("https://" + vcenter + "/rest/vcenter/vm/" + vm + "/guest/identity")

	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogInfo(fmt.Sprintf("VM Detail access to VCenter failed - %s", err), false)
		return vms.Value
	}

	err = json.Unmarshal([]byte(response.RespBody), &vms)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Detail - %s", err))
	}
	vms.Value.Found = true
	return vms.Value

}

func getAllVMs(headers [][2]string) vmwareVms {
	var response apiResponse
	var vms vmwareVms
	var datacenterFilter, folderFilter, clusterFilter string

	state := "POWERED_ON"

	if ignoreState {
		state = ""
	}
	stateFilter := "?filter.power_states=" + state

	if datacenter != "" {
		object := getObjectID(headers, "datacenter", datacenter)
		datacenterFilter = "&filter.datacenters=" + object.Datacenter
	}

	if cluster != "" {
		object := getObjectID(headers, "cluster", cluster)
		clusterFilter = "&filter.clusters=" + object.Cluster
	}

	//DO WE NEED FILTER ON FOLDER
	// if folder != "" {
	// 	folderFilter = "&filter.folders=" + folder
	// }
	defaultParams := stateFilter + datacenterFilter + folderFilter + clusterFilter

	apiURL, err := url.Parse("https://" + vcenter + "/rest/vcenter/vm" + defaultParams)

	response, err = httpCall("GET", apiURL.String(), []byte{}, headers, false)
	if err != nil {
		utils.LogError(fmt.Sprintf("Sessions Access to VCenter failed - %s", err))
	}

	err = json.Unmarshal([]byte(response.RespBody), &vms)
	if err != nil {
		utils.LogError(fmt.Sprintf("JSON parsing failed for VM Get - %s", err))
	}
	return vms
}

func getVMsFromTags(headers [][2]string, labels map[string]vcenterLabels, vms map[string]cloudData) {

	type objectids struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	}

	type value struct {
		ObjectIDS []objectids `json:"object_ids"`
		TagID     string      `json:"tag_id"`
	}

	var raw struct {
		Value []value `json:"value"`
	}
	tags := []string{}
	for l := range labels {
		tags = append(tags, l)
	}
	tmpbody := map[string][]string{"tag_ids": tags}

	body, err := json.Marshal(tmpbody)
	//body := []byte(`{"object_id":{"id":"vm-110677","type":"VirtualMachine"}}`)
	var response apiResponse
	apiURL, err := url.Parse(fmt.Sprintf("https://%s/rest/com/vmware/cis/tagging/tag-association?~action=list-attached-objects-on-tags", vcenter))

	response, err = httpCall("POST", apiURL.String(), body, headers, false)
	if strconv.Itoa(response.StatusCode)[0:1] != "2" {
		utils.LogInfo(fmt.Sprintf("TAGs do not have any vms - %s", tags), true)
		fmt.Println(response, err, response)
	}

	err = json.Unmarshal([]byte(response.RespBody), &raw)
	if err != nil {
		utils.LogError(fmt.Sprintf("GetTagFromVM Unmarshall Failed - %s", err))
	}

	for _, data := range raw.Value {
		for _, vm := range data.ObjectIDS {
			if vm.ID == "vm-110677" {
				fmt.Print("found")
			}
			tmpvm := vms[vm.ID]
			label := tmpvm.Tags
			if label == nil {
				label = make(map[string]string)
				label[labels[data.TagID].Key] = labels[data.TagID].Value
			} else {
				label[labels[data.TagID].Key] = labels[data.TagID].Value
			}
			tmpvm.Tags = label

			vms[vm.ID] = tmpvm

		}
	}

}

// func getTagsFromVM(headers [][2]string, vm string, label map[string]vcenterLabels) map[string]string {

// 	type vminfo struct {
// 		Type string `json:"type"`
// 		ID   string `json:"id"`
// 	}

// 	var raw struct {
// 		Value []string `json:"value"`
// 	}

// 	tags := make(map[string]string)

// 	tmpbody := map[string]vminfo{"object_id": {ID: vm, Type: "VirtualMachine"}}

// 	body, err := json.Marshal(tmpbody)
// 	//body := []byte(`{"object_id":{"id":"vm-110677","type":"VirtualMachine"}}`)
// 	var response apiResponse
// 	apiURL, err := url.Parse(fmt.Sprintf("https://%s/rest/com/vmware/cis/tagging/tag-association?~action=list-attached-tags", vcenter))

// 	response, err = httpCall("POST", apiURL.String(), body, headers, false)
// 	if strconv.Itoa(response.StatusCode)[0:1] != "2" {
// 		utils.LogInfo(fmt.Sprintf("VM doesnt have any tags - %s", vm), true)
// 		fmt.Println(response, err, response)
// 		return tags
// 	}

// 	err = json.Unmarshal([]byte(response.RespBody), &raw)
// 	if err != nil {
// 		utils.LogError(fmt.Sprintf("GetTagFromVM Unmarshall Failed - %s", err))
// 	}

// 	for _, tag := range raw.Value {
// 		if label[tag].Key != "" {
// 			tags[label[tag].Key] = label[tag].Value
// 		}
// 	}
// 	return tags
// }

func getSessionToken() string {

	var response apiResponse
	apiURL, err := url.Parse(fmt.Sprintf("https://%s/rest/com/vmware/cis/session", vcenter))

	response, err = httpCall("POST", apiURL.String(), []byte{}, nil, true)
	if err != nil {
		utils.LogError(fmt.Sprintf("Sessions Access to VCenter failed - %s", err))
	}
	var raw struct {
		Session string `json:"value"`
	}
	err = json.Unmarshal([]byte(response.RespBody), &raw)
	if err != nil {
		return ""
	}
	return raw.Session
}

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
	if pce.DisableTLSChecking == true {
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

func vcenterHTTP(keyMap map[string]string) map[string]cloudData {

	utils.LogInfo("VCenter API Session setup - ", false)

	if userID == "" || secret == "" {
		utils.LogError(fmt.Sprintf("Both user - %s", err))
	}

	//Call the EC2 API to get the instance info
	fmt.Println("Start")
	httpHeader := [][2]string{{"Content-Type", "application/json"}, {"vmware-api-session-id", getSessionToken()}}

	if err != nil {
		utils.LogError(fmt.Sprintf("DescribeInstances error - %s", err))
	}
	utils.LogInfo("AWS DescribeInstance API call - ", false)
	fmt.Println("Get Tags and Categories")
	categories := getCategories(httpHeader)
	var label = make(map[string]vcenterLabels)

	var totalTags []string

	for _, category := range categories {
		tmpcat := getCategoryDetail(httpHeader, category)
		if keyMap[tmpcat.Name] != "" {
			tagIDS := getTagFromCategories(httpHeader, tmpcat.ID)

			for _, tagid := range tagIDS {
				taginfo := getTagDetail(httpHeader, tagid)
				label[tagid] = vcenterLabels{Key: keyMap[tmpcat.Name], KeyID: tmpcat.ID, Value: taginfo.Name}
			}
			totalTags = append(totalTags, tagIDS...)
		}
	}
	fmt.Println("Get VM Info")
	var allvms = make(map[string]cloudData)
	listvms := getAllVMs(httpHeader).Value

	for _, tmpvm := range listvms {

		tmp := getVMDetail(httpHeader, tmpvm.VM)
		//tags := getTagsFromVM(httpHeader, tmpvm.VM, label)
		tmpintf := netInterface{PrivateDNS: tmp.HostName, PrivateIP: []string{tmp.IPAddress}, Primary: true}
		// if !tmp.Found {
		// 	tmpintf = netInterface{}
		// }

		tmplocation := ""
		if datacenter != "" {
			tmplocation = datacenter
		}
		// os := compute.Linux
		// if tmp.Family != "LINUX" {
		// 	os = compute.Windows
		// }

		allvms[tmpvm.VM] = cloudData{VMID: tmpvm.VM, Name: tmpvm.Name, State: tmpvm.PowerState, Location: tmplocation, Interfaces: []netInterface{tmpintf}}
	}
	getVMsFromTags(httpHeader, label, allvms)

	//Cycle through all the reservations for all arrays in that reservation

	utils.LogInfo(fmt.Sprintf("Total EC2 instances discovered - %d", len(allvms)), true)
	return allvms

}
