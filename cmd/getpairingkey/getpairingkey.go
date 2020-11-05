package getpairingkey

import (
	"fmt"
	"io"
	"os"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug bool
var profile, pkFile string

// Init handles flags
func init() {
	GetPairingKey.Flags().StringVarP(&profile, "profile", "p", "default", "Pairing profile name.")
	GetPairingKey.Flags().StringVarP(&pkFile, "file", "f", "", "File to store pairing key")
	GetPairingKey.Flags().SortFlags = false
}

// GetPairingKey gets a pairing key
var GetPairingKey = &cobra.Command{
	Use:   "get-pk",
	Short: "Get a pairing key.",
	Long: `
Gets a pairing key. The default pairing profile is used unless a profile name is specified with --profile (-p).

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the viper values
		debug = viper.Get("debug").(bool)

		getPK()
	},
}

func getPK() {

	// Log command execution
	utils.LogStartCommand("get-pk")

	// Get all pairing profiles
	pps, a, err := pce.GetAllPairingProfiles()
	utils.LogAPIResp("GetAllPairingProfiles", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	for _, pp := range pps {
		if pp.Name == profile {
			pk, a, err := pce.CreatePairingKey(pp)
			utils.LogAPIResp("CreatePairingKey", a)
			if err != nil {
				utils.LogError(err.Error())
			}
			fmt.Println(pk.ActivationCode)

			// Write the pairing key to a file
			if pkFile != "" {
				file, err := os.Create(pkFile)
				if err != nil {
					utils.LogError(err.Error())
				}
				defer file.Close()
				_, err = io.WriteString(file, pk.ActivationCode)
				if err != nil {
					utils.LogError(err.Error())
				}
			}
		}
	}
	utils.LogEndCommand("get-pk")

}
