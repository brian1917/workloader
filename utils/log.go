package utils

import (
	"log"
	"os"
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
