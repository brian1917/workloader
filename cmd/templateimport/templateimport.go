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
var doNotProvision bool
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

func init() {

	TemplateImportCmd.Flags().BoolVarP(&doNotProvision, "do-not-provision", "x", false, "Provision objects after creating them.")
	TemplateImportCmd.Flags().SortFlags = false

}

// Process template file
func importTemplate() {

	utils.LogStartCommand("template-import")

	template, err := illumioapi.ParseTemplateFile(templateFile)
	if err != nil {
		utils.LogError(err.Error())
	}

	ipls := []*illumioapi.IPList{}
	services := []*illumioapi.Service{}

	// Iterate templates
	for _, t := range template.IllumioSecurityTemplates {
		// Labels
		for _, l := range t.Labels {
			_, a, err := pce.CreateLabel(*l)
			if err != nil {
				utils.LogInfo(fmt.Sprintf("error creating label: %s (%s) - API Code: %d", l.Value, l.Key, a.StatusCode), true)
			} else {
				utils.LogInfo(fmt.Sprintf("created label: %s (%s)", l.Value, l.Key), false)
			}
		}
		// IPLists
		for _, i := range t.IPLists {
			ipl, a, err := pce.CreateIPList(*i)
			if err != nil {
				utils.LogInfo(fmt.Sprintf("error creating iplist: %s - API Code: %d", i.Name, a.StatusCode), false)
			} else {
				utils.LogInfo(fmt.Sprintf("created iplist: %s", i.Name), false)
				ipls = append(ipls, &illumioapi.IPList{Href: ipl.Href})
			}
		}
		// Services
		for _, s := range t.Services {
			svc, a, err := pce.CreateService(*s)
			if err != nil {
				utils.LogInfo(fmt.Sprintf("error creating service: %s - API Code: %d", s.Name, a.StatusCode), true)
			} else {
				utils.LogInfo(fmt.Sprintf("created service: %s", s.Name), false)
				services = append(services, &illumioapi.Service{Href: svc.Href})
			}
		}
	}

	if !doNotProvision {
		a, err := pce.ProvisionCS(illumioapi.ChangeSubset{IPLists: ipls, Services: services}, "Provisioned by workloader template-import.")
		if err != nil {
			utils.LogError(fmt.Sprintf("%s\r\n[ERROR] - API Body: %s", err, a.RespBody))
		}
	}

	utils.LogEndCommand("template-import")
}
