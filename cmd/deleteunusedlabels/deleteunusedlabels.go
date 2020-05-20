package labelsdeleteunused

import (
	"fmt"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var hrefFile string
var debug, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

// LabelsDeleteUnused runs the unpair
var LabelsDeleteUnused = &cobra.Command{
	Use:   "labels-delete-unused",
	Short: "Delete labels that are not used.",
	Long: `  
Delete labels that are not used.

Use the --update-pce command to run the delete with a user prompt confirmation.
Use --update-pce and --no-prompt to run the delete with no prompts.`,
	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get persistent flags from Viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		labelsDeleteUnused()
	},
}

func labelsDeleteUnused() {

	// Get all labels
	labels, a, err := pce.GetAllLabels()
	utils.LogAPIResp("GetAllLabels", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// For each label, try to delete it
	for _, l := range labels {
		a, err := pce.DeleteHref(l.Href)
		utils.LogAPIResp("DeleteHref", a)
		if err != nil {
			message := ""
			if a.StatusCode == 406 {
				message = " which often means label is currently in use."
			}
			if a.StatusCode == 401 {
				message = " which often means account does not have permission to delete labels."
			}
			utils.LogInfo(fmt.Sprintf("%s(%s) could not be deleted. Status code %d%s", l.Value, l.Key, a.StatusCode, message))
		} else {
			utils.LogInfo(fmt.Sprintf("%s(%s) deleted - Status code %d.", l.Value, l.Key, a.StatusCode))
		}
	}

	fmt.Println("completed running labels-delete-unused command.")
	utils.LogInfo("completed running labels-delete-unused command.")
}
