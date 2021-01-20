package dagsync

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
	"os"
	"sort"
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
	Payload Payload
}

// Payload contains the information for the API Request
type Payload struct {
	XMLName    xml.Name `xml:"payload"`
	Register   Register
	Unregister Unregister
}

// Register contains the information for the API Request
type Register struct {
	XMLName xml.Name `xml:"register,omitempty"`
	Entry   []Entry
}

// Unregister contains the information for the API Request
type Unregister struct {
	XMLName xml.Name `xml:"unregister,omitempty"`
	Entry   []Entry
}

// DagResponse - Declare Response Struct for PAN API call
type DagResponse struct {
	XMLName xml.Name `xml:"response"`
	Status  string   `xml:"status,attr"`
	Result  Result   `xml:"result"`
}

// Result - Declare Result container of PAN API call
type Result struct {
	XMLName xml.Name `xml:"result"`
	Entry   []Entry  `xml:"entry"`
	Count   int      `xml:"count"`
}

// Entry - Declare Entry container of PAN API call
type Entry struct {
	XMLName    xml.Name `xml:"entry"`
	IP         string   `xml:"ip,attr,"`
	FromAgent  string   `xml:"from_agent,attr,omitempty"`
	Persistent string   `xml:"persistent,attr,omitempty"`
	Tag        Tag      `xml:"tag,omitempty"`
}

// Tag - Declare Entry container of PAN API call
type Tag struct {
	XMLName xml.Name `xml:"tag"`
	Member  []string `xml:"member,omitempty"`
}

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug, addIPv6, change, insecure, flush bool
var outFormat string

func init() {
	DAGSyncCmd.Flags().BoolVarP(&addIPv6, "ipv6", "6", false, "Include IPv6 addresses in the syncing of PCE IP and labels/tags with PAN DAGs")
	DAGSyncCmd.Flags().BoolVarP(&insecure, "insecure", "i", false, "Ignore SSL certificate validation when communicating with PAN.")
	DAGSyncCmd.Flags().BoolVarP(&change, "change", "c", false, "Do not Sync Illumio PCE IP address and labels/tags to PAN DAGs but provide output and log what would sync.")
	DAGSyncCmd.Flags().BoolVarP(&flush, "flush", "f", false, "Remove all Registered IPs from PAN")
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
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		dagSync()
	},
}

// httpSetUp - Used to make API call to device.  Require HTTP Action, URL, body (if present), if SSL cert ignored.
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

	// Set headers for async
	// if async == true {
	// 	req.Header.Set("Prefer", "respond-async")
	// }

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
func panHTTP(apiURL string, data url.Values) DagResponse {

	var response DagResponse
	u, _ := url.ParseRequestURI(apiURL)

	resp, err := httpSetUp(http.MethodPost, u.String(), []byte(data.Encode()), insecure, [][2]string{[2]string{"Content-Type", "application/x-www-form-urlencoded"}, [2]string{"Content-Length", strconv.Itoa(len(data.Encode()))}})
	if err != nil {
		utils.LogError(err.Error())
	}

	if err := xml.Unmarshal([]byte(resp.RespBody), &response); err != nil {
		utils.LogError(err.Error())
	}
	return response
}

//ipv6Check - Function that checks IP string for ":"=IPv6 and returns true
func ipv6Check(ip string) bool {
	for i := 0; i < len(ip); i++ {
		switch ip[i] {
		case '.':
			return false
		case ':':
			return true
		}
	}
	return false
}

//workloadIPMap - Build a map of all workloads IPs and their corresponding labels.
func workloadIPMap() map[string][]string {
	var ipMap = make(map[string][]string)

	wklds, a, err := pce.GetAllWorkloads()
	utils.LogAPIResp("GetAllWorkloads", a)
	if err != nil {
		utils.LogError(fmt.Sprintf("getting all workloads - %s", err))
	}

	for _, w := range wklds {
		var labels []string
		for _, l := range w.Labels {
			labels = append(labels, pce.LabelMapH[l.Href].Value)
		}

		for _, i := range w.Interfaces {
			if net.ParseIP(i.Address) != nil {
				if ipv6Check(net.ParseIP(i.Address).String()) {
					if addIPv6 {
						ipMap[net.ParseIP(i.Address).String()] = labels
					}
				} else {
					ipMap[net.ParseIP(i.Address).String()] = labels
				}

			} else {
				utils.LogError(fmt.Sprintf("Invalid IP addres from PCE - %s", i.Address))
			}
		}
	}
	return ipMap
}

//getPanRegisteredIPs - Get all currently loaded Registered IPs from PAN.  Uses to compare against PCE workload IPs to sync.
func getPanRegisteredIPs(panKEY, panURL string) map[string][]string {

	var response DagResponse

	//apiURL := fmt.Sprintf("%s/api?key=%s&type=op&cmd=<show><object><registered-ip><all></all></registered-ip></object></show>", panURL, panKEY)

	apiURL := fmt.Sprintf("%s/api", panURL)
	data := url.Values{}
	data.Set("key", panKEY)
	data.Set("type", "op")
	data.Set("cmd", "<show><object><registered-ip><all></all></registered-ip></object></show>")

	//Add method, url, body and headers to POST
	response = panHTTP(apiURL, data)
	if response.Status != "success" {

	}
	var tmpDagEntries = make(map[string][]string)
	for _, e := range response.Result.Entry {
		if net.ParseIP(e.IP) != nil {
			tmpDagEntries[net.ParseIP(e.IP).String()] = e.Tag.Member
		} else {
			utils.LogError(fmt.Sprintf("Invalid IP addres from PAN - %s", e.IP))
		}
	}
	return tmpDagEntries

}

//panRegister - Call PAN to add IPs and labels to Registered IPs
func panRegisterAPI(apiURL string, data url.Values, register bool, ip string, labels []string) {

	var request DagRequest
	if register {
		var tmprequest = DagRequest{Type: "update", Version: "2.0", Payload: Payload{Register: Register{Entry: []Entry{Entry{IP: ip, FromAgent: "0", Persistent: "1", Tag: Tag{Member: labels}}}}}}
		request = tmprequest
		utils.LogInfo(fmt.Sprintf("Register %s with the following labels %s from RegisteredIPs", ip, labels), false)
	} else {
		var tmprequest = DagRequest{Type: "update", Version: "2.0", Payload: Payload{Unregister: Unregister{Entry: []Entry{Entry{IP: ip}}}}}
		request = tmprequest
		utils.LogInfo(fmt.Sprintf("Unregister %s from RegisteredIPs", ip), false)
	}
	if change {
		xml, _ := xml.MarshalIndent(request, "", "")
		data.Set("cmd", string(xml))

		response := panHTTP(apiURL, data)
		if response.Status != "success" {
			utils.LogInfo(fmt.Sprintf("Unregistered failed with %s Status- IP - %s", response.Status, ip), true)
		}
	}
}

//IsEqual -  compare function for arrays - Order not guaranteed
func IsEqual(a1 []string, a2 []string) bool {
	sort.Strings(a1)
	sort.Strings(a2)
	if len(a1) == len(a2) {
		for i, v := range a1 {
			if v != a2[i] {
				return false
			}
		}
	} else {
		return false
	}
	return true
}

//dagSync - Compares IPs already registered on PAN with those on the PCE also compare the labels/tags currently configured.  If different labels/tags
func dagSync() {

	//Enter Start Log for PAN DAG Sync
	utils.LogStartCommand(fmt.Sprintf("PAN DAG Sync - change=%t, insecure=%t, ipv6=%t", change, insecure, addIPv6))

	panKEY := os.Getenv("PANORAMA_KEY")
	panURL := os.Getenv("PANORAMA_URL")
	if panKEY == "" || panURL == "" {
		fmt.Println("Please make sure PANORAMA_KEY and PANORAMA_KEY environment variables are set")
		utils.LogError(fmt.Sprintf("Environment variable missing"))
	}

	//Get PAN registered IPs and Workload IPs from PAN/PCE
	panEntries := getPanRegisteredIPs(panKEY, panURL)
	workloadsMap := workloadIPMap()

	//define PAN API settings
	apiURL := fmt.Sprintf("%s/api", panURL)
	data := url.Values{}
	data.Set("key", panKEY)
	data.Set("type", "user-id")

	//Cycle through workloads and see if there is a current registered IP - labels must be the same
	var register = true
	var unregister = false

	if !change {
		utils.LogInfo(fmt.Sprintf("Changes will not be made - must enter \"--change\" or \"-c\" to make changes to PAN!!!"), true)
	}

	//clear RegisterIP database.
	if flush {
		fmt.Printf("Flushing RegisterIP data \r\n")
		for e := range panEntries {
			panRegisterAPI(apiURL, data, unregister, e, []string{})
		}
		return
	}

	sync := false
	for ip, labels := range workloadsMap {
		if len(panEntries) == 0 {
			panRegisterAPI(apiURL, data, register, ip, labels)
			fmt.Printf("Register IP %s with labels %s\r\n", ip, labels)
			sync = true
		} else if len(panEntries) != 0 && len(panEntries[ip]) == 0 {
			panRegisterAPI(apiURL, data, register, ip, labels)
			fmt.Printf("Register IP %s with labels %s\r\n", ip, labels)
			sync = true
		} else if len(panEntries) != 0 && !IsEqual(panEntries[ip], labels) {
			panRegisterAPI(apiURL, data, unregister, ip, []string{})
			panRegisterAPI(apiURL, data, register, ip, labels)
			fmt.Printf("Update IP %s with labels %s\r\n", ip, labels)
			sync = true
		}
	}
	//clear RegisterIP database.
	if flush {
		fmt.Printf("Flushing RegisterIP data \r\n")
		for e := range panEntries {
			panRegisterAPI(apiURL, data, unregister, e, []string{})
		}
		return
	}

	//Check to see there are Registered IPs that are not workloads
	for e := range panEntries {
		if len(workloadsMap[e]) == 0 {
			panRegisterAPI(apiURL, data, unregister, e, []string{})
			fmt.Printf("Unregister IP %s \r\n", e)
			sync = true
		}

	}
	if !sync {
		fmt.Printf("No IPs to sync\r\n")
	}
}
