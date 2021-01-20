strings.NewReader(ge d.Encode(agsync

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
	Version string   `xml:"version"`
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
	XMLName xml.Name `xml:"register"`
	Entry   []Entry
}

// Unregister contains the information for the API Request
type Unregister struct {
	XMLName xml.Name `xml:"unregister"`
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
	FromAgent  string   `xml:"from_agent,attr"`
	Persistent string   `xml:"persistent,attr"`
	Tag        Tag      `xml:"tag"`
}

// Tag - Declare Entry container of PAN API call
type Tag struct {
	XMLName xml.Name `xml:"tag"`
	Member  []string `xml:"member"`
}

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug bool
var outFormat, outputFileName string

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

func getPanRegisteredIPs(panKEY, panURL string) map[string][]string {

	var response DagResponse

	//apiURL := fmt.Sprintf("%s/api?key=%s&type=op&cmd=<show><object><registered-ip><all></all></registered-ip></object></show>", panURL, panKEY)

	testURL := fmt.Sprintf("%s/api", panURL)
	u, _ := url.ParseRequestURI(testURL)
	data := url.Values{}
	data.Set("key", panKEY)
	data.Set("type", "op")
	data.Set("cmd", "<show><object><registered-ip><all></all></registered-ip></object></show>")

	//Add header to request
	resp, _ := httpSetUp(http.MethodPost, u.String(), string.NewReader(data.Encode(), true, [][2]string{[2]string{"Content-Type", "application/xml"}})

	utils.LogAPIResp("GetPANRegisteredIPs", illumioapi.APIResponse{})
	//fmt.Print(resp.RespBody)
	if err := xml.Unmarshal([]byte(resp.RespBody), &response); err != nil {
		utils.LogError(err.Error())
	}

	var tmpDagEntries = make(map[string][]string)
	for _, e := range response.Result.Entry {
		if net.ParseIP(e.IP) != nil {
			tmpDagEntries[net.ParseIP(e.IP).String()] = e.Tag.Member
		}
	}
	return tmpDagEntries

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
				ipMap[net.ParseIP(i.Address).String()] = labels
			}
		}
	}
	return ipMap
}

//panRegister -
func panRegister(ip string, labels []string) {
	fmt.Println("reg", ip, labels)
}

//panUnregister -
func panUnregister(ip string, labels []string) {
	fmt.Println("un", ip, labels)
}

//IsEqual -  compare function - Order not guaranteed
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

func dagSync() {

	// Load PCE struct with all the active data
	// err := pce.Load("active")
	// if err != nil {
	// 	utils.LogError(err.Error())
	// }
	panKEY := os.Getenv("PANORAMA_KEY")
	panURL := os.Getenv("PANORAMA_URL")
	if panKEY == "" || panURL == "" {
		fmt.Println("Please make sure PANORAMA_KEY and PANORAMA_KEY environment variables are set")
		utils.LogError(fmt.Sprintf("Environment variable missing"))
	}
	panEntries := getPanRegisteredIPs(panKEY, panURL)
	workloadsMap := workloadIPMap()

	for ip, labels := range workloadsMap {
		if len(panEntries) == 0 {
			panRegister(ip, labels)
		} else if len(panEntries) != 0 && len(panEntries[ip]) == 0 {
			panRegister(ip, labels)
		} else if len(panEntries) != 0 && !IsEqual(panEntries[ip], labels) {
			panUnregister(ip, labels)
			panRegister(ip, labels)
		}
	}
}
