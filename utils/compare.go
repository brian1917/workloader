package utils

import (
	"fmt"
	"strings"
)

func SliceComare(slice1 []string, slice2 []string, slice1Name, slice2Name string) (equal bool, logMsg string) {
	logs := []string{}

	// Build Maps
	map1 := make(map[string]bool)
	map2 := make(map[string]bool)
	for _, elem := range slice1 {
		map1[elem] = true
	}
	for _, elem := range slice2 {
		map2[elem] = true
	}

	// Set equal to true and flip it when necessary
	equal = true

	// Check slice 1
	for _, elem := range slice1 {
		if _, ok := map2[elem]; !ok {
			equal = false
			logs = append(logs, fmt.Sprintf("%s is in %s but not in %s", elem, slice1Name, slice2Name))
		}
	}

	// Check slice 2
	for _, elem := range slice2 {
		if _, ok := map1[elem]; !ok {
			equal = false
			logs = append(logs, fmt.Sprintf("%s is in %s but not in %s", elem, slice2Name, slice1Name))
		}
	}

	return equal, strings.Join(logs, ";")
}
