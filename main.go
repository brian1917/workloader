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

	// Setup logging
	utils.SetUpLogging()

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
				utils.LogInfof(true, "running %s", strings.Join(append(os.Args[2:], "--pce", pce), " "))
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
			// utils.LogWarning("this command has been renamed to traffic. please use \"workloader traffic\" in the future", true)
			command := exec.Command(os.Args[0], append([]string{"legacy-explorer"}, os.Args[2:]...)...)
			utils.LogInfof(false, "executing the following: %s", command.String())
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

		// EB change to deny rule
		if os.Args[1] == "eb-import" {
			utils.LogWarning("this command has been renamed to deny-rule-import. please use \"workloader deny-rule-import\" in the future", true)
			command := exec.Command(os.Args[0], append([]string{"deny-rule-import"}, os.Args[2:]...)...)
			command.Stdin = os.Stdin
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			utils.LogInfof(false, "executing the following: %s", command.String())
			if err := command.Run(); err != nil {
				utils.LogError(err.Error())
			}
			// fmt.Println(string(stdout))
			return
		}
		if os.Args[1] == "eb-export" {
			utils.LogWarning("this command has been renamed to deny-rule-export. please use \"workloader deny-rule-export\" in the future", true)
			command := exec.Command(os.Args[0], append([]string{"deny-rule-export"}, os.Args[2:]...)...)
			command.Stdin = os.Stdin
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			utils.LogInfof(false, "executing the following: %s", command.String())
			if err := command.Run(); err != nil {
				utils.LogError(err.Error())
			}
			return
		}
		// Change to deny rules
		if os.Args[1] == "deny-rule-import" || os.Args[1] == "deny-rule-export" {
			utils.LogWarning("deny rules as a separate object are deprecated in the newest PCEs (version 24+). use rule-import and rule-export to manage all allow and deny rules.", true)
		}

		// Process Mode
		if os.Args[1] == "mode" {
			utils.LogWarning("this command has been removed. use wkld-import with the --allow-enforcement-changes flag", true)
			return
		}
	}

	// Run command for all other scenarios
	cmd.Execute()
}
