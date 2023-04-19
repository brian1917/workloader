package utils

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/spf13/viper"
)

// Logger is the global logger for Workloader
var Logger log.Logger

func init() {

	f, err := os.OpenFile("workloader.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		log.Fatal(err)
	}
	Logger.SetOutput(f)

}

// LogError writes the error the workloader.log and always prints an error to stdout.
func LogError(msg string) {

	Logger.SetPrefix(time.Now().Format("2006-01-02 15:04:05 "))
	fmt.Printf("%s [ERROR] - %s see workloader.log for detailed information if error is from an illumio api call.\r\n", time.Now().Format("2006-01-02 15:04:05 "), msg)
	if (viper.Get("continue_on_error") != nil && viper.Get("continue_on_error").(bool)) || (viper.Get("continue_on_error_default") != nil && viper.Get("continue_on_error_default").(string) == "continue") {
		Logger.Printf("[ERROR] - %s\r\n", msg)
	} else {
		Logger.Fatalf("[ERROR] - %s\r\n", msg)
	}
}

// LogWarning writes the log to workloader.log and optionally prints msg to stdout.
func LogWarning(msg string, stdout bool) {
	Logger.SetPrefix(time.Now().Format("2006-01-02 15:04:05 "))
	if stdout {
		fmt.Printf("%s [WARNING] - %s\r\n", time.Now().Format("2006-01-02 15:04:05 "), msg)
	}
	Logger.Printf("[WARNING] - %s\r\n", msg)
}

// LogInfo writes the log to workloader.log and never prints to stdout.
func LogInfo(msg string, stdout bool) {
	Logger.SetPrefix(time.Now().Format("2006-01-02 15:04:05 "))
	if stdout {
		fmt.Printf("%s [INFO] - %s\r\n", time.Now().Format("2006-01-02 15:04:05 "), msg)
	}
	Logger.Printf("[INFO] - %s\r\n", msg)
}

// LogDebug writes the log to workloader.log only if debug flag is set and never prints to stdout.
// Debug logic is not required in code.
func LogDebug(msg string) {

	// Get the debug value from viper
	debug := viper.Get("debug").(bool)

	if debug {
		Logger.SetPrefix(time.Now().Format("2006-01-02 15:04:05 "))
		Logger.Printf("[DEBUG] - %s\r\n", msg)
	}
}

// LogAPIResp will log the HTTP Requset, Request Header, Response Status Code, and Response Body
// The callType should be the name of call: GetAllLabels, GetAllWorkloads, etc. This is just for logging purposes and any string will be accepted.
// The log type will be DEBUG.
// This call will not do anything if the debug flag isn't set. A debug conditional is not required in app code.
func LogAPIResp(callType string, apiResp illumioapi.APIResponse) {

	// If we have a bad API response, set the debug to true
	if apiResp.StatusCode > 299 {
		viper.Set("debug", true)
	}

	if apiResp.Request != nil {
		LogDebug(fmt.Sprintf("%s http request: %s %v", callType, apiResp.Request.Method, apiResp.Request.URL))
		LogDebug(fmt.Sprintf("%s request body: %s", callType, apiResp.ReqBody))
	}
	LogInfo(fmt.Sprintf("%s status sode: %d", callType, apiResp.StatusCode), false)
	if viper.Get("verbose").(bool) || apiResp.StatusCode > 299 {
		LogDebug(fmt.Sprintf("%s response body: %s", callType, apiResp.RespBody))
	}

	for _, w := range apiResp.Warnings {
		LogWarning(w, true)
	}
}

func LogMultiAPIResp(APIResps map[string]illumioapi.APIResponse) {
	for k, v := range APIResps {
		LogAPIResp(k, v)
	}
}

// LogStartCommand is used at the beginning of each command
func LogStartCommand(commandName string) {
	Logger.Println("-----------------------------------------------------------------------------")
	LogInfo(fmt.Sprintf("workloader version %s - started %s", GetVersion(), commandName), false)
	if viper.IsSet("target_pce") && viper.Get("target_pce") != nil && viper.Get("target_pce").(string) != "" {
		LogInfo(fmt.Sprintf("using %s pce - %s", viper.Get("target_pce").(string), viper.Get(viper.Get("target_pce").(string)+".pce_version")), false)
	} else {
		if viper.Get("default_pce_name") != nil {
			LogInfo(fmt.Sprintf("using default pce - %s - %s", viper.Get("default_pce_name").(string), viper.Get(viper.Get("default_pce_name").(string)+".pce_version")), false)
		}
	}
}

// LogEndCommand is used at the end of each command
func LogEndCommand(commandName string) {
	LogInfo(fmt.Sprintf("%s completed", commandName), true)
}

// Replaces a blank string with <empty>
func LogBlankValue(val string) string {
	if val == "" {
		return "<empty>"
	}
	return val
}
