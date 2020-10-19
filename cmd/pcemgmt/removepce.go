package pcemgmt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var clear bool
var pceName string

func init() {
	RemovePCECmd.Flags().BoolVarP(&clear, "clear-keys", "x", false, "Remove PCE from yaml file and clear all Workloader generated API credentials from the PCE.")
}

// RemovePCECmd removes the pce.yaml file
var RemovePCECmd = &cobra.Command{
	Use:   "pce-remove [name of pce]",
	Short: "remove a pce from pce.yaml file",
	Long:  fmt.Sprintf("\r\n%s\r\n\r\nThe --update-pce and --no-prompt flags are ignored for this command.", utils.LogOutDesc()),
	PreRun: func(cmd *cobra.Command, args []string) {
		configFilePath, err = filepath.Abs(viper.ConfigFileUsed())
		if err != nil {
			utils.LogError(err.Error())
		}
	},
	Run: func(cmd *cobra.Command, args []string) {

		// Get Name of PCE
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the name of the PCE to logout. See usage help.")
			os.Exit(0)
		}
		pceName = args[0]
		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		removePce()
	},
}

func removePce() {

	utils.LogStartCommand("pce-remove")

	// Start by clearing API keys
	if clear {

		// Log start of command
		utils.LogInfo("removing API keys...", true)

		// Get the PCE
		pce, err := utils.GetPCEbyName(pceName, false)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get all API Keys
		apiKeys, _, err := pce.GetAllAPIKeys(viper.Get(pceName + ".userhref").(string))
		if err != nil {
			utils.LogError(err.Error())
		}

		// Delete the API keys that are from Workloader
		saveHref := ""
		for _, a := range apiKeys {
			if a.Name == "Workloader" {
				if a.AuthUsername != viper.Get(pceName+".user").(string) {
					_, err := pce.DeleteHref(a.Href)
					if err != nil {
						utils.LogError(err.Error())
					}
					utils.LogInfo(fmt.Sprintf("deleted api key: %s", a.Href), true)
				} else {
					saveHref = a.Href
				}
			}
		}
		// Delete the active API Key
		_, err = pce.DeleteHref(saveHref)
		if err != nil {
			utils.LogError(err.Error())
		}
		utils.LogInfo(fmt.Sprintf("deleted api key: %s", saveHref), true)
	}

	// Remove login information from YAML
	configMap := viper.AllSettings()
	delete(configMap, pceName)
	encodedConfig, _ := json.MarshalIndent(configMap, "", " ")
	err := viper.ReadConfig(bytes.NewReader(encodedConfig))
	if err != nil {
		utils.LogError(err.Error())
	}
	viper.WriteConfig()

	utils.LogInfo("Removed pce infomration from pce.yaml.", true)

	utils.LogEndCommand("pce-remove")

}
