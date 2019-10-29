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

// Log writes to the Workloader logger. t must be 0, 1, or 2.
//
// 0 = Info
//
// 1 = Error
//
// 2 = Debug
//
// Errors (t=1) will also print a message to std out.
// Debug (t=2) will only log if the debug flag has been set as pulled from Viper.
func Log(t int, msg string) {
	var logType string

	// Get the debug value from viper
	debug := viper.Get("debug").(bool)

	// Set the time prefix for the logger
	Logger.SetPrefix(time.Now().Format("2006-01-02 15:04:05 "))

	switch t {
	case 0:
		logType = "[INFO]"
		Logger.Printf("%s - %s\r\n", logType, msg)
	case 1:
		logType = "[ERROR]"
		fmt.Printf("Error - %s - run with --debug flag and see workloader.log for more details.\r\n", msg)
		Logger.Fatalf("%s - %s\r\n", logType, msg)
	case 2:
		if debug {
			logType = "[DEBUG]"
			Logger.Printf("%s - %s\r\n", logType, msg)
		}
	}

}

// NewLog writes to the Workloader logger. t must be 0, 1, or 2.
//
// 0 = Info
//
// 1 = Error
//
// 2 = Debug
//
// Info (t=0) will only print a message to stdout if stdout=true
// Errors (t=1) will always print a message to stdout.
// Debug (t=2) will only log if the debug flag has been set as pulled from Viper and will never print to stdout.
func NewLog(t int, stdout bool, msg string) {

	// Get the debug value from viper
	debug := viper.Get("debug").(bool)

	// Set the time prefix for the logger
	Logger.SetPrefix(time.Now().Format("2006-01-02 15:04:05 "))

	switch t {
	case 0:
		Logger.Printf("[INFO] - %s\r\n", msg)
		if stdout {
			fmt.Printf("[INFO] - %s\r\n", msg)
		}
	case 1:
		fmt.Printf("[ERROR] - %s - run with --debug flag and see workloader.log for more details.\r\n", msg)
		Logger.Fatalf("[ERROR] - %s\r\n", msg)
	case 2:
		if debug {
			Logger.Printf("[DEBUG] - %s\r\n", msg)
		}
	}

}

// LogAPIResp will log the HTTP Requset, Request Header, Response Status Code, and Response Body
// The callType should be the name of call: GetAllLabels, GetAllWorkloads, etc. This is just for logging purposes and any string will be accepted.
// The log type will be DEBUG.
// This call will not do anything if the debug flag isn't set. A debug conditional is not required in app code.
func LogAPIResp(callType string, apiResp illumioapi.APIResponse) {
	Log(2, fmt.Sprintf("%s HTTP Request: %s %v", callType, apiResp.Request.Method, apiResp.Request.URL))
	Log(2, fmt.Sprintf("%s Reqest Header: %v", callType, apiResp.Request.Header))
	Log(2, fmt.Sprintf("%s Response Status Code: %d", callType, apiResp.StatusCode))
	Log(2, fmt.Sprintf("%s Response Body: %s", callType, apiResp.RespBody))
}
