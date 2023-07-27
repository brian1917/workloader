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

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/pkg/errors"
)

// httpCall - Generic Function to call VCenter APIs
func httpCall(httpAction, apiURL string, body []byte, login bool) (illumioapi.APIResponse, error) {

	var response illumioapi.APIResponse
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
	if insecure {
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	}

	req, err := http.NewRequest(httpAction, apiURL, httpBody)
	if err != nil {
		return response, err
	}

	// Set basic authentication and headers
	if login {
		req.SetBasicAuth(vc.User, vc.Secret)
	}

	// Set basic authentication and headers
	for k, v := range vc.Header {
		req.Header.Set(k, v)
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

	//for any deprecated VCenter API call there is a "value:" as the first entry in the data returned.
	//This command removes the "value:" so that all the responses now have the same structure.
	if strings.Contains(response.RespBody, "\"value\":") {
		response.RespBody = response.RespBody[:len(response.RespBody)-1]
		response.RespBody = response.RespBody[9:]
	}

	// Check for a 200 response code
	if strconv.Itoa(resp.StatusCode)[0:1] != "2" {
		return response, errors.New("http status code of " + strconv.Itoa(response.StatusCode))
	}

	// Return data and nil error
	return response, nil
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

// cleanFQDN cleans up the provided PCE FQDN in case of common errors
func (v *VCenter) cleanFQDN() string {
	// Remove trailing slash if included
	v.VCenterURL = strings.TrimSuffix(v.VCenterURL, "/")
	// Remove HTTPS if included
	v.VCenterURL = strings.TrimPrefix(v.VCenterURL, "https://")
	return v.VCenterURL
}

// GetCollectionHeaders returns a collection of Illumio objects and allows for customizing headers of HTTP request
// func (v *VCenter) Get(endpoint string, queryParameters, headers map[string]string, login bool, response interface{}, calledAPI string) (api illumioapi.APIResponse, err error) {
func (v *VCenter) Get(endpoint string, queryParameters map[string]string, login bool, response interface{}, calledAPI string) {
	// Build the API URL
	tmpurl, err := url.Parse("https://" + v.cleanFQDN() + endpoint)
	if err != nil {
		utils.LogError(fmt.Sprintf("%s Unable to Parse URL - %s", calledAPI, err))
		//return illumioapi.APIResponse{}, err
	}

	// Set the query parameters
	for key, value := range queryParameters {
		//Necessary check because golang net/url Encodes a space character as "+".  VCenter needs that to be %20
		if calledAPI == "getObjectID" {
			tmp := key + "=" + value
			tmpurl.RawQuery = url.PathEscape(tmp)
		} else {
			q := tmpurl.Query()
			q.Set(key, value)
			tmpurl.RawQuery = q.Encode()

		}
	}

	// Call the API
	api, err := httpCall("GET", tmpurl.String(), []byte{}, login)
	utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{calledAPI: api})
	//Check for ServiceNot available for getVMIdentity or getNetInterfaces because lack of VMTools
	if (err != nil && api.StatusCode != 503) && (calledAPI == "getVMIdentity" || calledAPI == "getVMNetworkDetail") {
		utils.LogError(fmt.Sprintf("%s access to VCenter failed - %s", calledAPI, err))
		//return api, err
	} else if err != nil && api.StatusCode == 503 {
		return
	}

	// Unmarshal response to struct and return
	err = json.Unmarshal([]byte(api.RespBody), &response)
	if err != nil {
		utils.LogError(fmt.Sprintf("Unmarshal of %s object failed - %s", calledAPI, err))
		//return api, err
	}
	//return api, nil

}

// Post sends a POST request to the VCenter
func (v *VCenter) Post(endpoint string, object, createdObject interface{}, login bool, calledAPI string) (api illumioapi.APIResponse, err error) {

	// Build the API URL
	apiURL, err := url.Parse("https://" + v.cleanFQDN() + endpoint)
	if err != nil {
		utils.LogError(fmt.Sprintf("%s Unable to Parse URL - %s", calledAPI, err))
		//return api, err
	}

	// Create payload
	jsonBytes, err := json.Marshal(object)
	if err != nil {
		utils.LogError(fmt.Sprintf("Unmarshal of %s object failed - %s", calledAPI, err))
		//return api, err
	}

	// Call the API
	api, err = httpCall("POST", apiURL.String(), jsonBytes, login)
	//api, err = httpCall("POST", apiURL.String(), jsonBytes, map[string]string{"Content-Type": "application/json"}, true)
	api.ReqBody = string(jsonBytes)
	utils.LogMultiAPIRespV2(map[string]illumioapi.APIResponse{calledAPI: api})
	if err != nil {
		utils.LogError(fmt.Sprintf("%s access to VCenter failed - %s", calledAPI, err))
		//return api, err
	}

	// Unmarshal new label
	err = json.Unmarshal([]byte(api.RespBody), &createdObject)
	if err != nil {
		utils.LogError(fmt.Sprintf("Unmarshal of %s object failed - %s", calledAPI, err))
		//return api, err
	}

	return api, nil
}
