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
	fmt.Printf("[ERROR] - %s - run with --debug and see workloader.log for detailed API response information.\r\n", msg)
	Logger.Fatalf("[ERROR] - %s\r\n", msg)
}

// LogWarning writes the log to workloader.log and optionally prints msg to stdout.
func LogWarning(msg string, stdout bool) {
	Logger.SetPrefix(time.Now().Format("2006-01-02 15:04:05 "))
	if stdout {
		fmt.Printf("[WARNING] - %s\r\n", msg)
	}
	Logger.Printf("[WARNING] - %s\r\n", msg)
}

// LogInfo writes the log to workloader.log and never prints to stdout.
func LogInfo(msg string) {
	Logger.SetPrefix(time.Now().Format("2006-01-02 15:04:05 "))
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

	LogDebug(fmt.Sprintf("%s HTTP Request: %s %v", callType, apiResp.Request.Method, apiResp.Request.URL))
	LogDebug(fmt.Sprintf("%s Reqest Header: %v", callType, apiResp.Request.Header))
	LogDebug(fmt.Sprintf("%s Response Status Code: %d", callType, apiResp.StatusCode))
	LogDebug(fmt.Sprintf("%s Response Body: %s", callType, apiResp.RespBody))
}

// LogStartCommand is used at the beginning of each command
func LogStartCommand(commandName string) {
	Logger.Println("-----------------------------------------------------------------------------")
	LogInfo(fmt.Sprintf("workloader version %s - started %s", commandName, GetVersion()))
}

// LogEndCommand is used at the end of each command
func LogEndCommand(commandName string) {
	LogInfo(fmt.Sprintf("completed %s", commandName))
}
