package utils

import (
	"fmt"

	"github.com/brian1917/illumioapi/v2"
	"github.com/spf13/viper"
)

// LogAPIResp will log the HTTP Requset, Request Header, Response Status Code, and Response Body
// The callType should be the name of call: GetAllLabels, GetAllWorkloads, etc. This is just for logging purposes and any string will be accepted.
// The log type will be DEBUG.
// This call will not do anything if the debug flag isn't set. A debug conditional is not required in app code.
func LogAPIRespV2(callType string, apiResp illumioapi.APIResponse) {

	// Get the original logging status in case it's flipped for a non-200 status code
	// orginalDebug := viper.Get("debug").(bool)

	// If we have a bad API response, set the debug to true
	// if apiResp.StatusCode > 299 {
	// 	viper.Set("debug", true)
	// }

	if apiResp.Request != nil {
		LogDebug(fmt.Sprintf("%s http request: %s %v", callType, apiResp.Request.Method, apiResp.Request.URL))
		LogDebug(fmt.Sprintf("%s request body: %s", callType, apiResp.ReqBody))
	}
	LogInfo(fmt.Sprintf("%s response status code: %d", callType, apiResp.StatusCode), false)
	if viper.Get("verbose").(bool) || apiResp.StatusCode > 299 {
		LogDebug(fmt.Sprintf("%s response body: %s", callType, apiResp.RespBody))
	}

	for _, w := range apiResp.Warnings {
		LogWarning(w, true)
	}

	// Put the logging back to the original status in case it was flipped for a non-200 status code
	// viper.Set("debug", orginalDebug)
}

func LogMultiAPIRespV2(APIResps map[string]illumioapi.APIResponse) {
	for k, v := range APIResps {
		LogAPIRespV2(k, v)
	}
}
