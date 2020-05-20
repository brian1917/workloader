package getpairingkey

import (
	"fmt"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug bool
var profile string

// Init handles flags
func init() {
	GetPairingKey.Flags().StringVarP(&profile, "profile", "p", "default", "Pairing profile name.")
	GetPairingKey.Flags().SortFlags = false
}

// GetPairingKey gets a pairing key
var GetPairingKey = &cobra.Command{
	Use:   "get-pk",
	Short: "Get a pairing key.",
	Long: `
Gets a pairing key. The default pairing profile is used unless a profile name is specified with --profile (-p).`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetDefaultPCE(true)
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
		}
	}
	utils.LogEndCommand("get-pk")

}
