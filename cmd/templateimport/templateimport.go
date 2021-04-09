package templateimport

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/workloader/cmd/ruleimport"

	"github.com/brian1917/workloader/cmd/rulesetimport"

	"github.com/brian1917/workloader/cmd/iplimport"

	"github.com/brian1917/workloader/cmd/svcimport"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Global variables
var template, directory string
var pce illumioapi.PCE
var provision, updatePCE, noPrompt bool
var err error

// TemplateImportCmd runs the template import command
var TemplateImportCmd = &cobra.Command{
	Use:   "template-import [template to import]",
	Short: "Import an Illumio segmentation template.",
	Long: `
Import an Illumio segmentation template.

Segmentation templates are a set of CSV files. By default, workloader looks for an "illumio-template" directory in the current directory. To use a different directory, use the --directory flag.

Templates can be customized by editing the CSV files.

Use template-list command to see available templates.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		// Set the template file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the template name. See usage help.")
			os.Exit(0)
		}
		template = args[0]

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		importTemplate()
	},
}

func init() {

	TemplateImportCmd.Flags().BoolVar(&provision, "provision", false, "Provision objects after creating them.")
	TemplateImportCmd.Flags().StringVar(&directory, "directory", "", "Custom directory for templates.")
	TemplateImportCmd.Flags().SortFlags = false

}

// Process template file
func importTemplate() {

	// Log start of command
	utils.LogStartCommand("template-import")

	// Get the directory
	if directory == "" {
		directory = "illumio-templates/"
	} else if directory[len(directory)-1:] != string(os.PathSeparator) {
		directory = fmt.Sprintf("%s%s", directory, string(os.PathSeparator))
	}

	// Services
	fmt.Println("\r\n------------------------------------------ SERVICES -------------------------------------------")
	svcFile := fmt.Sprintf("%s%s.services.csv", directory, template)
	if _, err := os.Stat(svcFile); err == nil {
		data, err := utils.ParseCSV(svcFile)
		if err != nil {
			utils.LogError(err.Error())
		}
		svcimport.ImportServices(svcimport.Input{PCE: pce, Data: data, UpdatePCE: updatePCE, NoPrompt: noPrompt, Provision: provision})
	} else {
		utils.LogInfo(fmt.Sprintf("%s template does not include services. skipping", template), true)
	}

	// IP Lists
	fmt.Println("\r\n------------------------------------------ IP Lists -------------------------------------------")
	iplFile := fmt.Sprintf("%s%s.iplists.csv", directory, template)
	if _, err := os.Stat(iplFile); err == nil {
		iplimport.ImportIPLists(pce, iplFile, updatePCE, noPrompt, false, provision)
	} else {
		utils.LogInfo(fmt.Sprintf("%s template does not include ip lists. skipping", template), true)
	}

	// Rulesets
	fmt.Println("\r\n------------------------------------------ RULE SETS ------------------------------------------")
	rsFile := fmt.Sprintf("%s%s.rulesets.csv", directory, template)
	if _, err := os.Stat(rsFile); err == nil {
		rulesetimport.ImportRuleSetsFromCSV(rulesetimport.Input{PCE: pce, UpdatePCE: updatePCE, NoPrompt: noPrompt, Provision: provision, CreateLabels: true, ImportFile: rsFile, ProvisionComment: "workloader template-import"})
	} else {
		utils.LogInfo(fmt.Sprintf("%s template does not include rule sets. skipping", template), true)
	}

	// Rules
	fmt.Println("\r\n------------------------------------------- RULES ---------------------------------------------")
	rFile := fmt.Sprintf("%s%s.rules.csv", directory, template)
	if _, err := os.Stat(rFile); err == nil {
		ruleimport.ImportRulesFromCSV(ruleimport.Input{PCE: pce, ImportFile: rFile, ProvisionComment: "workloader template-import", Provision: provision, UpdatePCE: updatePCE, NoPrompt: noPrompt, CreateLabels: true})
	} else {
		utils.LogInfo(fmt.Sprintf("%s template does not include rules. skipping", template), true)
	}
	fmt.Println("-------------------------------------------------------------------------------------------")

	// Warn on Any IP List
	f, err := os.Open(rFile)
	if err != nil {
		utils.LogError(err.Error())
	}
	defer f.Close()

	// Splits on newlines by default.
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "Any (0.0.0.0/0 and ::/0)") {
			fmt.Println()
			utils.LogWarning("This template includes some rules that uses the Any (0.0.0.0/0 and ::/0) IP List. Review these rules and use a more refined IP List where necessary.\r\n", true)
			break
		}
	}

	utils.LogEndCommand("template-import")
}
