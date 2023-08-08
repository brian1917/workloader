package getpairingkey

import (
	"fmt"
	"io"
	"os"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var profile, pkFile string

// Init handles flags
func init() {
	GetPairingKey.Flags().StringVarP(&profile, "profile", "p", "Default (Servers)", "Pairing profile name.")
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
		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		getPK()
	},
}

func getPK() {

	// Log command execution
	utils.LogStartCommand("get-pk")

	// Get all pairing profiles
	pps, a, err := pce.GetPairingProfiles((map[string]string{"name": profile}))
	utils.LogAPIRespV2("GetAllPairingProfiles", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	match := false
	for _, pp := range pps {
		if pp.Name == profile {
			match = true
			pk, a, err := pce.CreatePairingKey(pp)
			utils.LogAPIRespV2("CreatePairingKey", a)
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

	if !match {
		utils.LogErrorf("pairing profile %s does not exist", profile)
	}

	// Log the end of the command (don't use utils.LogEndCommand so we don't print to stdout)
	utils.LogInfo("get-pk completed", false)

}
