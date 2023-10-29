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
var create bool
var profile, pkFile, venType string

// Init handles flags
func init() {
	GetPairingKey.Flags().StringVarP(&profile, "profile", "p", "Default (Servers)", "pairing profile name.")
	GetPairingKey.Flags().StringVarP(&pkFile, "file", "f", "", "file to store pairing key")
	GetPairingKey.Flags().BoolVarP(&create, "create", "c", false, "create pairing profile if it does not exist.")
	GetPairingKey.Flags().StringVarP(&venType, "ven-type", "v", "", "ven type (endpoint or server) used in conjunction with --create option")

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
		pce, err = utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		if create && (venType != "server" && venType != "endpoint") {
			utils.LogError("ven type must be server or endpoint with create flag.")
		}

		getPK()
	},
}

func getPK() {

	// Get all pairing profiles
	pps, a, err := pce.GetPairingProfiles((map[string]string{"name": profile}))
	utils.LogAPIRespV2("GetAllPairingProfiles", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	match := false
	var targetPairingProfile illumioapi.PairingProfile
	for _, pp := range pps {
		if pp.Name == profile {
			match = true
			targetPairingProfile = pp
			break
		}
	}

	if !match && create {
		utils.LogInfof(false, "%s doesn't exist - creating", profile)
		createdPP, api, err := pce.CreatePairingProfile(illumioapi.PairingProfile{Name: profile, Enabled: illumioapi.Ptr(true), VenType: venType})
		utils.LogAPIRespV2("CreatePairingProfile", api)
		if err != nil {
			utils.LogErrorf("creating pairing profile - %s", err)
		}
		targetPairingProfile = createdPP
	}

	if !match && !create {
		utils.LogErrorf("%s does not exist. rerun with --create (-c) flag to create it.", profile)
	}

	// Get pairing key
	pk, a, err := pce.CreatePairingKey(targetPairingProfile)
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
