package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/brian1917/workloader/cmd"
	"github.com/brian1917/workloader/cmd/pcemgmt"
	"github.com/brian1917/workloader/utils"
)

func main() {
	// Process target-pces and all-pces
	if len(os.Args) > 1 {
		if os.Args[1] == "target-pces" && os.Args[2] != "-h" && os.Args[2] != "--help" {

			// Parse CSV data
			csvData, err := utils.ParseCSV(os.Args[2])
			if err != nil {
				utils.LogError(err.Error())
			}

			// Create PCE map
			pceMap := make(map[string]bool)
			for _, row := range csvData {
				pceMap[row[0]] = true
			}

			for _, pce := range pcemgmt.GetAllPCENames() {

				if pceMap[pce] {
					utils.LogInfo(fmt.Sprintf("running %s", strings.Join(append(os.Args[3:], "--pce", pce), " ")), true)
					command := exec.Command(os.Args[0], append(os.Args[3:], "--pce", pce)...)
					stdout, err := command.Output()
					if err != nil {
						utils.LogError(err.Error())
					}
					fmt.Println(string(stdout))
				}
			}
			return
		}

		// Process all-pces
		if os.Args[1] == "all-pces" && os.Args[2] != "-h" && os.Args[2] != "--help" {
			for _, pce := range pcemgmt.GetAllPCENames() {
				utils.LogInfo(fmt.Sprintf("running %s", strings.Join(append(os.Args[2:], "--pce", pce), " ")), true)
				command := exec.Command(os.Args[0], append(os.Args[2:], "--pce", pce)...)
				stdout, err := command.Output()
				if err != nil {
					utils.LogError(err.Error())
				}
				fmt.Println(string(stdout))
			}
			return
		}

		// Explorer renamed to traffic
		if os.Args[1] == "explorer" {
			utils.LogWarning("this command has been renamed to traffic. please use \"workloader traffic\" in the future", true)
			command := exec.Command(os.Args[0], append([]string{"traffic"}, os.Args[2:]...)...)
			utils.LogInfof(true, "executing the following: %s", command.String())
			stdout, err := command.Output()
			if err != nil {
				utils.LogError(err.Error())
			}
			fmt.Println(string(stdout))
			return
		}

		// Moved flow summary
		if os.Args[1] == "flowsummary" {
			utils.LogWarning("this command has been renamed to appgroup-flow-summary. please use \"workloader appgroup-flow-summary\" in the future", true)
			if len(os.Args) > 2 && os.Args[2] == "appgroup" {
				command := exec.Command(os.Args[0], append([]string{"appgroup-flow-summary"}, os.Args[3:]...)...)
				utils.LogInfof(true, "executing the following: %s", command.String())
				stdout, err := command.Output()
				if err != nil {
					utils.LogError(err.Error())
				}
				fmt.Println(string(stdout))
			}
			return
		}
	}

	// Run command for all other scenarios
	cmd.Execute()
}
