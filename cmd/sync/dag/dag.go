package dag

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// APIResponse contains the information from the response of the API
type APIResponse struct {
	RespBody   string
	StatusCode int
	Header     http.Header
	Request    *http.Request
	ReqBody    string
}

// DagRequest contains the information for the API Request
type DagRequest struct {
	XMLName xml.Name `xml:"uid-message"`
	Type    string   `xml:"type"`
	Version string   `xml:"version,omitempty"`
	Payload Payload  `xml:"payload"`
}

// Payload contains the information for the API Request
type Payload struct {
	Register   Register   `xml:"register,omitempty"`
	Unregister Unregister `xml:"unregister,omitempty"`
}

// Register contains the information for the API Request
type Register struct {
	Entry []Entry `xml:"entry,omitempty"`
}

// Unregister contains the information for the API Request
type Unregister struct {
	Entry []Entry `xml:"entry,omitempty"`
}

// DagResponse - Declare Response Struct for PAN API call
type DagResponse struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  Result   `xml:"result,omitempty"`
	MSG     string   `xml:"msg,omitempty"`
}

// Result - Declare Result container of PAN API call
type Result struct {
	Entry   []Entry `xml:"entry,omitempty"`
	Count   int     `xml:"count,omitempty"`
	Error   string  `xml:"error,omitempty"`
	Enabled string  `xml:"enabled,omitempty"`
	Group   Group   `xml:"group,omitempty"`
}

// Entry - Declare Entry container of PAN API call
type Entry struct {
	IP         string `xml:"ip,attr"`
	FromAgent  string `xml:"from_agent,attr,omitempty"`
	Persistent string `xml:"persistent,attr,omitempty"`
	Tag        Tag    `xml:"tag,omitempty"`
}

// Global - Declare Entry container of PAN API call
type Group struct {
	LocalInfo LocalInfo `xml:"local-info,omitempty"`
}

type LocalInfo struct {
	State string `xml:"state,omitempty"`
}

// Tag - Declare Entry container of PAN API call
type Tag struct {
	Members []string `xml:"member,omitempty"`
}

//PAN structure used to
type PAN struct {
	Key    string
	URL    string
	RegIPs map[string]string
}

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug, addIPv6, update, insecure, flush, removeOld bool
var outFormat, panURL, panKey, panVsys string

func init() {
	DAGSyncCmd.Flags().StringVarP(&panURL, "url", "u", "", "URL required to reach Panorama or PAN FW(requires https://).")
	DAGSyncCmd.MarkFlagRequired("url")
	DAGSyncCmd.Flags().StringVarP(&panKey, "key", "k", "", "Key used to authenticate with Panorama or PAN FW.")
	DAGSyncCmd.MarkFlagRequired("key")
	DAGSyncCmd.Flags().StringVarP(&panVsys, "vsys", "v", "", "Vsys used to progam registered IPs and tags.")
	DAGSyncCmd.MarkFlagRequired("vsys")
	DAGSyncCmd.Flags().BoolVarP(&addIPv6, "ipv6", "6", false, "Include IPv6 addresses in the syncing of PCE IP and labels/tags with PAN DAGs")
	DAGSyncCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Ignore SSL certificate validation when communicating with PAN.")
	DAGSyncCmd.Flags().BoolVarP(&update, "update-pan", "", false, "By default do not Sync Illumio PCE IP address and labels/tags to PAN DAGs but provide output and log what would have synced.")
	DAGSyncCmd.Flags().BoolVarP(&flush, "flush", "f", false, "Remove all Registered IPs from PAN")
	DAGSyncCmd.Flags().MarkHidden("flush")
	DAGSyncCmd.Flags().BoolVarP(&removeOld, "remove-old", "r", false, "Remove all Registered IPs that don't have IP on the PCE")
}

// DAGSyncCmd runs the DAG register/unregister PAN API Sync
var DAGSyncCmd = &cobra.Command{
	Use:   "dagsync",
	Short: "Syncs IPs and Labels for Workloads between PCE and Dynamic Access Group on Palo Alto FW",
	Long: `
Syncs PCE Workloads IPs and Labels and Palo Alto FW Dynamic Access Groups.  Must either pass ("-u") PAN URL and ("-k") KEY.  
You can also configure environmental variables PANORAMA_URL and PANORAMA_KEY.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the viper values
		outFormat = viper.Get("output_format").(string)

		dagSync()
	},
}

// httpSetUp - Used to make API call to PAN.  Require HTTP Action, URL, body (if present), if SSL cert ignored and headers (if present).
func httpSetUp(httpAction, apiURL string, body []byte, disableTLSChecking bool, headers [][2]string) (APIResponse, error) {

	var response APIResponse
	var httpBody *bytes.Buffer

	// Validate the provided action
	httpAction = strings.ToUpper(httpAction)
	if httpAction != "GET" && httpAction != "POST" && httpAction != "PUT" && httpAction != "DELETE" {
		return response, errors.New("invalid http action string. action must be GET, POST, PUT, or DELETE")
	}

	// Create body
	httpBody = bytes.NewBuffer(body)

	// Create HTTP client and request
	client := &http.Client{}
	if disableTLSChecking == true {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	req, err := http.NewRequest(httpAction, apiURL, httpBody)
	if err != nil {
		return response, err
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
	response.RespBody = string(data[:])
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

//panHTTP - Function to setup HTTP POST with necessary headers and other requirements
func (pan *PAN) callHTTP(cmdType string, cmd string) DagResponse {

	var dagResp DagResponse
	apiURL := fmt.Sprintf("%s/api", pan.URL)
	urlInfo := url.Values{}
	urlInfo.Set("key", pan.Key)
	urlInfo.Set("type", cmdType)
	urlInfo.Set("cmd", cmd)

	url, err := url.ParseRequestURI(apiURL)
	if err != nil {
		utils.LogError(fmt.Sprintf("Get Registered IP URL Parse failed - %s", err))
	}

	resp, err := httpSetUp(http.MethodPost, url.String(), []byte(urlInfo.Encode()), insecure, [][2]string{{"Content-Type", "application/x-www-form-urlencoded"}, {"Content-Length", strconv.Itoa(len(urlInfo.Encode()))}})
	if err != nil {
		utils.LogError(fmt.Sprintf("PanHTTP Call failed - %s", err))
	}

	//Unmarshal the HTTP call and place in DagResponse.
	if err := xml.Unmarshal([]byte(resp.RespBody), &dagResp); err != nil {
		utils.LogError(fmt.Sprintf("Unmarshall HTTPSetUp response - %s - Body - %s", err, resp.ReqBody))
	}
	//check to see that the results do not have an error.
	if dagResp.Result.Error != "" {
		utils.LogError(fmt.Sprintf("API request has Error - %s", dagResp.Result.Error))
	}

	return dagResp
}

//ipv6Check - Function that checks IP string for valid IP.  Also checks to see if Ipv6 and if IPv6 should be included
func ipCheck(ip string) string {

	//make sure ip string is a valid IP.
	if net.ParseIP(ip) == nil {
		utils.LogError(fmt.Sprintf("Invalid IP addres from PCE - %s", ip))
	}

	//skip all link local addresses
	_, ipv4LL, _ := net.ParseCIDR("169.254.0.0/16")
	_, ipv6LL, _ := net.ParseCIDR("fe80::/10")

	//Check if the IP is v4 or v6.  For v6 only add if command option enabled.
	if strings.Contains(ip, ".") && !ipv4LL.Contains(net.ParseIP(ip)) {
		return ip
	}
	if strings.Contains(ip, ":") && addIPv6 && !ipv6LL.Contains(net.ParseIP(ip)) {
		return ip
	}

	return ""
}

//workloadIPMap - Build a map of all workloads IPs and their corresponding labels.
func workloadIPMap() map[string][]string {
	var pceIpMap = make(map[string][]string)

	wklds, a, err := pce.GetAllWorkloads()
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(fmt.Sprintf("getting all workloads - %s", err))
	}

	for _, w := range wklds {
		var labels []string

		//Make sure there is a Tag to add.
		if len(*w.Labels) == 0 {
			continue
		}

		//Cycle through labels getting the Value from the Href
		for _, l := range *w.Labels {
			labels = append(labels, pce.Labels[l.Href].Value)
		}

		// Check ip address to make sure valid and not link local.
		for _, ip := range w.Interfaces {
			if ipCheck(ip.Address) != "" {
				pceIpMap[ip.Address] = labels
			}
		}
	}
	return pceIpMap
}

//getPanRegisteredIPs - Get all currently loaded Registered IPs from PAN.  Uses to compare against PCE workload IPs to sync.
func (pan *PAN) getRegisteredIPs() map[string][]string {

	var dagResp DagResponse

	var tmpDagEntries = make(map[string][]string)

	//Send Set VSYS API request.  panHttp check for success within the response message.  Fails if not successful.
	setVsysCMD := fmt.Sprintf("<set><system><setting><target-vsys>%s</target-vsys></setting></system></set>", panVsys)
	dagResp = pan.callHTTP("op", setVsysCMD)

	//remove parameter so we can readd
	entryLimit := 500
	startPoint := 0
	//limit calls to 500.  and Cycle through if you find more.
	getRegIPCMD := "<show><object><registered-ip><all></all></registered-ip></object></show>"

	for {
		//Send GET Registered IP API request.  panHttp check for success within the response message.  Fails if not successful.
		dagResp = pan.callHTTP("op", getRegIPCMD)
		//Add the registered IPs and Tags to global variable used for syncing
		for _, e := range dagResp.Result.Entry {
			if net.ParseIP(e.IP) != nil {
				tmpDagEntries[net.ParseIP(e.IP).String()] = e.Tag.Members
			} else {
				utils.LogError(fmt.Sprintf("Invalid IP addres from PAN - %s", e.IP))
			}
		}
		//If number of entries less than per call limit no more request to call. Otherwise move start point + entryLimits and request again.
		if dagResp.Result.Count < entryLimit {
			break

		} else {
			startPoint += entryLimit
			getRegIPCMD = fmt.Sprintf("<show><object><registered-ip><limit>%d</limit><start-point>%d</start-point></registered-ip></object></show>", entryLimit, startPoint)
		}
	}

	//Send Set VSYS back to "none" API request.  panHttp check for success within the response message.  Fails if not successful.
	setVsysCMD = "<set><system><setting><target-vsys>none</target-vsys></setting></system></set>"
	dagResp = pan.callHTTP("op", setVsysCMD)

	return tmpDagEntries

}

//panRegister - Call PAN to add IPs and labels to Registered IPs
func (pan *PAN) RegisterAPI(register bool, ipLabelList map[string][]string) {
	var request DagRequest
	var entries []Entry
	if register {
		for ip, labels := range ipLabelList {
			entries = append(entries, Entry{IP: ip, FromAgent: "0", Persistent: "1", Tag: Tag{Members: labels}})
			utils.LogInfo(fmt.Sprintf("Register %s with the following labels %s", ip, labels), false)
		}
		request = DagRequest{Type: "update", Version: "2.0", Payload: Payload{Register: Register{Entry: entries}}}
	} else {
		for ip, labels := range ipLabelList {
			if len(labels) == 0 {
				entries = append(entries, Entry{IP: ip}) //, Tag: Tag{Members: labels}
				utils.LogInfo(fmt.Sprintf("Unregister %s", ip), false)
			} else {
				entries = append(entries, Entry{IP: ip, Tag: Tag{Members: labels}})
				utils.LogInfo(fmt.Sprintf("Unregistering Labels %s - labels %s", ip, labels), false)
			}
		}
		request = DagRequest{Type: "update", Version: "2.0", Payload: Payload{Unregister: Unregister{Entry: entries}}}
	}

	//If update set send api to PAN
	if update {
		xmlData, _ := xml.MarshalIndent(request, "", "")
		dagResp := pan.callHTTP("user-id", string(xmlData))
		if dagResp.Status != "success" {
			utils.LogInfo(fmt.Sprintf("Register/Unregister API call received error - %s", dagResp.MSG), true)
		}
	}
}

//checkHAPrimary - make sure we are adding Registered IPs to primary PAN in a HA
func (pan *PAN) checkHA() bool {

	//Send show HA API request.  panHttp check for success within the response message.  Fails if not successful.
	setVsysCMD := "<show><high-availability><state></state></high-availability></show>"
	dagResp := pan.callHTTP("op", setVsysCMD)

	if strings.ToLower(dagResp.Result.Enabled) == "no" {
		return true
	}
	if strings.ToLower(dagResp.Result.Group.LocalInfo.State) == "active" || strings.ToLower(dagResp.Result.Group.LocalInfo.State) == "active-primary" {
		return true
	}
	return false

}

//Remove element in string
func removeElement(slice []string, i int) []string {
	copy(slice[i:], slice[i+1:])
	return slice[:len(slice)-1]
}

//isEqual -  compare function for arrays - Order not guaranteed
func isEqual(a1 []string, a2 []string) (bool, []string, []string) {

	var remove []string
	var equal bool = true

	//create a map of all elements in first array
	add := make(map[string]bool)
	for _, item := range a2 {
		add[item] = true
	}

	for _, v := range a1 {
		//if _, ok := add[v]; !ok && v != "%Illumio_Added%" {
		if _, ok := add[v]; !ok {
			equal = false
			remove = append(remove, v)
		} else if ok {
			delete(add, v)
		}
	}
	if len(a1) < len(a2) {
		equal = false
	}

	var addLabels []string
	for k := range add {
		addLabels = append(addLabels, k)
	}
	return equal, remove, addLabels
}

//dagSync - Compares IPs already registered on PAN with those on the PCE also compare the labels/tags currently configured.  If different labels/tags
func dagSync() {

	//Enter Start Log for PAN DAG Sync
	utils.LogStartCommand(fmt.Sprintf("PAN DAG Sync - change=%t, insecure=%t, ipv6=%t, flush=%t, rmeoveOld=%t", update, insecure, addIPv6, flush, removeOld))

	//Create PAN struct with empty map of registered IPs
	pan := PAN{Key: panKey, URL: panURL, RegIPs: make(map[string]string)}

	//Check to see if URL is for non-HA or active/active-primary PAN.  Need to only push IPs to active.
	if !pan.checkHA() {
		utils.LogError(fmt.Sprintf("URL entered is trying to use backup HA device. URL - %s", panURL))
	}

	//Get PAN registered IPs and Workload IPs from PAN/PCE
	panEntries := pan.getRegisteredIPs()

	//Get all Workloads from PCE.
	workloadsMap := workloadIPMap()

	if !update {
		utils.LogInfo(fmt.Sprintf("Changes will not be made - must enter \"--change\" or \"-c\" to make changes to PAN!!!"), true)
	}

	//clear RegisterIP database and quit.
	if flush && update {
		if len(panEntries) != 0 {
			utils.LogInfo("Flushing Register-IP data ", true)
			pan.RegisterAPI(false, panEntries)
			utils.LogEndCommand("dag-sync")
			return
		} else {
			utils.LogInfo("Nothing to Flush", true)
			utils.LogEndCommand("dag-sync")
			return
		}
	}

	//If there are no entries from PAN just all all the workloads.
	if len(panEntries) == 0 && len(workloadsMap) != 0 && update {
		pan.RegisterAPI(true, workloadsMap)
		utils.LogInfo(fmt.Sprintf("All PCE IPs and Labels added. PAN had no Registered-IPs"), true)
		utils.LogEndCommand("dag-sync")
		return
	}

	regEntries := make(map[string][]string)
	unregEntries := make(map[string][]string)
	//Cycle through Workload list as long as there are labels/tags continue
	for ip, labels := range workloadsMap {
		if len(labels) == 0 {
			continue
		}
		//If there isnt an entry for that IP on the PAN add the workload and labels/tags
		if _, ok := panEntries[ip]; !ok {
			regEntries[ip] = labels
			continue
		}
		//Check if both label sets are equal.  If not return the labels to add or remove or both
		if ok, removeLabels, addLabels := isEqual(panEntries[ip], labels); !ok {
			//skip adding these entries if list of labels is empty
			if len(addLabels) != 0 {
				regEntries[ip] = addLabels
			}
			if len(removeLabels) != 0 {
				unregEntries[ip] = removeLabels
			}
		}
	}

	for ip := range panEntries {
		if _, ok := workloadsMap[ip]; !ok && removeOld {
			unregEntries[ip] = []string{}
		}
	}

	if len(regEntries) == 0 && len(unregEntries) == 0 {
		utils.LogInfo(fmt.Sprintf("Nothing to do. No Add/Update/Removals needed on PAN."), true)
		utils.LogEndCommand("dag-sync")
		return
	}
	if len(regEntries) != 0 {
		pan.RegisterAPI(true, regEntries)
		utils.LogInfo(fmt.Sprintf("Updating PAN with PCE IPs and Labels"), true)
	}
	//make sure there is some unregister updates need
	if len(unregEntries) != 0 {
		pan.RegisterAPI(false, unregEntries)
		utils.LogInfo(fmt.Sprintf("Removing IPs or Labels from PAN."), true)
	}
	utils.LogEndCommand("dag-sync")
}
