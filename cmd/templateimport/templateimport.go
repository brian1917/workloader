package templateimport

import (
	"fmt"
	"os"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Global variables
var templateFile string
var pce illumioapi.PCE
var err error

// TemplateImportCmd runs the template import command
var TemplateImportCmd = &cobra.Command{
	Use:   "template-import [template file to import]",
	Short: "Import an Illumio segmentation template.",
	Long: `
Import an Illumio segmentation template.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		// Set the template file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the template file. See usage help.")
			os.Exit(0)
		}
		templateFile = args[0]

		importTemplate()
	},
}

// Process template file
func importTemplate() {

	utils.Log(0, "started running template-import command.")

	template, err := illumioapi.ParseTemplateFile(templateFile)
	if err != nil {
		utils.Log(1, err.Error())
	}

	var a illumioapi.APIResponse
	// Iterate templates
	for _, t := range template.IllumioSecurityTemplates {
		// Labels
		for _, l := range t.Labels {
			_, a, err = pce.CreateLabel(*l)
			if err != nil {
				utils.Log(0, fmt.Sprintf("error creating label: %s (%s) - API Code: %d", l.Value, l.Key, a.StatusCode))
				fmt.Println(a.RespBody)
			} else {
				utils.Log(0, fmt.Sprintf("created label: %s (%s)", l.Value, l.Key))
			}
		}
		// IPLists
		for _, i := range t.IPLists {
			_, a, err = pce.CreateIPList(*i)
			if err != nil {
				utils.Log(0, fmt.Sprintf("error creating iplist: %s - API Code: %d", i.Name, a.StatusCode))
			} else {
				utils.Log(0, fmt.Sprintf("created iplist: %s", i.Name))
			}
		}
		// Services
		for _, s := range t.Services {
			_, a, err = pce.CreateService(*s)
			if err != nil {
				utils.Log(0, fmt.Sprintf("error creating service: %s - API Code: %d", s.Name, a.StatusCode))
				fmt.Println(a.RespBody)
			} else {
				utils.Log(0, fmt.Sprintf("created service: %s", s.Name))
			}
		}
	}

	utils.Log(0, "completed running template-import command.")
}
