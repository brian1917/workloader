package vmsync

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/brian1917/workloader/utils"
	"github.com/pkg/errors"
)

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

// makeLowerCase - Take any string and make it all lowercase
// input string
// output lowercase string
func makeLowerCase(str string) string {
	return strings.ToLower(str)
}

// nameCheck - Match Hostname with or without domain information
// input string with or without a domain.
// output sting without domain
func nameCheck(name string) string {

	if !keepFQDNHostname {
		fullname := strings.Split(name, ".")
		return fullname[0]
	}
	return name
}
