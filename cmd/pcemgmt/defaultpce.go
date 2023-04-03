package pcemgmt

import (
	"fmt"
	"path/filepath"

	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// PCEListCmd gets all PCEs
var PCEListCmd = &cobra.Command{
	Use:   "pce-list",
	Short: "List all PCEs in pce.yaml.",
	PreRun: func(cmd *cobra.Command, args []string) {
		configFilePath, err = filepath.Abs(viper.ConfigFileUsed())
		if err != nil {
			utils.LogError(err.Error())
		}
	},
	Run: func(cmd *cobra.Command, args []string) {

		allSettings := viper.AllSettings()

		defaultPCEName := ""
		if viper.Get("default_pce_name") != nil {
			defaultPCEName = viper.Get("default_pce_name").(string)
		}

		count := 0
		for k := range allSettings {
			if viper.Get(k+".fqdn") != nil {
				if k == defaultPCEName {
					fmt.Printf("* %s (%s)\r\n", k, viper.Get(k+".fqdn").(string))
					count++
				} else {
					fmt.Printf("  %s (%s)\r\n", k, viper.Get(k+".fqdn").(string))
					count++
				}
			}
		}
		if count == 0 {
			utils.LogInfo("no pce configured. run pce-add to add a pce to pce.yaml file.", true)
		}

	},
}

// GetAllPCEnames returns PCE names in the pce.yaml file
func GetAllPCENames() (pceNames []string) {
	allSettings := viper.AllSettings()
	for k := range allSettings {
		if viper.Get(k+".fqdn") != nil {
			pceNames = append(pceNames, k)
		}
	}
	return pceNames
}
