package utils

import "runtime"

//LogOutDesc returns the text of the logout command based on runtime
func LogOutDesc() string {
	if runtime.GOOS == "windows" {
		return "Removes login information from pce.yaml and optionally removes all workloader generated API keys from PCE."
	}
	return "Removes pce.yaml file and optionally removes all workloader generated API keys from PCE."
}
