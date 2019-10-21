package dupecheck

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pce illumioapi.PCE
var debug bool
var err error

// DupeCheckCmd summarizes flows
var DupeCheckCmd = &cobra.Command{
	Use:   "dupecheck",
	Short: "Identifies duplicate hostnames and IP addresses in the PCE.",
	Long: `
Identifies duplicate hostnames and IP addresses in the PCE.

The --update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetPCE(false)
		if err != nil {
			utils.Log(1, err.Error())
		}

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		dupeCheck()
	},
}

func dupeCheck() {
	// Get all workloads
	wklds, a, err := pce.GetAllWorkloads()
	if debug {
		utils.LogAPIResp("GetAllWorkloads", a)
	}
	if err != nil {
		utils.Log(1, err.Error())
	}

	// Check for duplicate IPs
	dupeIPs, dupeIPMap := DupeIPCheck(pce, wklds)
	if dupeIPs {
		data := [][]string{[]string{"ip_addess", "hostnames"}}
		for i, h := range dupeIPMap {
			data = append(data, []string{i, strings.Join(h, ";")})
		}
		utils.WriteOutput(data, data, fmt.Sprintf(fmt.Sprintf("workloader-dupeIPs-%s.csv", time.Now().Format("20060102_150405"))))
		fmt.Printf("%d duplicate IP addresses found.\r\n", len(dupeIPMap))
	} else {
		utils.Log(0, "no duplicate IPs found")
		fmt.Println("No duplicate IPs found.")
	}

	// Check for duplicate hostnames
	dupeHostnames, dupeHostMap := DupeHostnameCheck(pce, wklds)
	if dupeHostnames {
		data := [][]string{[]string{"hostname", "occurrences"}}
		for h, o := range dupeHostMap {
			data = append(data, []string{h, strconv.Itoa(o)})
		}
		utils.WriteOutput(data, data, fmt.Sprintf(fmt.Sprintf("workloader-dupe-hostnames-%s.csv", time.Now().Format("20060102_150405"))))
		fmt.Printf("%d duplicate hostnames found.\r\n", len(dupeHostMap))
	} else {
		utils.Log(0, "no duplicate hostnames found")
		fmt.Println("No duplicate hostnames found.")
	}
}

// DupeIPCheck looks for an duplicate IP addresses in a PCE.
// If any are found it returns true with a map with they key as the ip address and the value as the slice of hostnames.
func DupeIPCheck(p illumioapi.PCE, wklds []illumioapi.Workload) (bool, map[string][]string) {
	// Create a map to hold interfaces and workloads
	interfaceMap := make(map[string][]string)

	// Iterate through the workloads to build the initial map
	for _, w := range wklds {
		for _, i := range w.Interfaces {
			if v, ok := interfaceMap[i.Address]; !ok {
				interfaceMap[i.Address] = []string{w.Hostname}
			} else {
				interfaceMap[i.Address] = append(v, w.Hostname)
			}
		}

	}

	// Create the map of just duplicates
	duplicateMap := make(map[string][]string)
	for a, b := range interfaceMap {
		if len(b) > 1 {
			duplicateMap[a] = b
		}
	}

	// Return
	if len(duplicateMap) > 0 {
		return true, duplicateMap
	}

	return false, duplicateMap
}

// DupeHostnameCheck looks for duplicate hostnames in a PCE.
// If any are found it returns true with a slice of the duplicated host names.
func DupeHostnameCheck(p illumioapi.PCE, wklds []illumioapi.Workload) (bool, map[string]int) {
	// Create a map to hold interfaces and workloads
	hostnameMap := make(map[string]int)

	// Iterate through workloads
	for _, w := range wklds {
		if v, ok := hostnameMap[strings.ToLower(w.Hostname)]; !ok {
			hostnameMap[strings.ToLower(w.Hostname)] = 1
		} else {
			hostnameMap[strings.ToLower(w.Hostname)] = v + 1
		}
	}

	// Created duplicated map
	dupeHostName := make(map[string]int)
	for h, count := range hostnameMap {
		if count > 1 {
			dupeHostName[h] = count
		}
	}

	// Return
	if len(dupeHostName) > 0 {
		return true, dupeHostName
	}
	return false, dupeHostName
}
