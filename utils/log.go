package utils

import (
	"fmt"
	"log"
	"os"
	"time"
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
// Errors (t=1) will also print a message to std out
func Log(t int, msg string) {
	var logType string

	// Set the time prefix for the logger
	Logger.SetPrefix(time.Now().Format("2006-01-02 15:04:05 "))

	switch t {
	case 0:
		logType = "[INFO]"
	case 1:
		logType = "[ERROR]"
	case 2:
		logType = "[DEBUG]"
	}
	if t == 1 {
		fmt.Printf("Error - %s - see workloader.log\r\n", msg)
		Logger.Fatalf("%s - %s\r\n", logType, msg)
	} else {
		Logger.Printf("%s - %s\r\n", logType, msg)
	}

}
