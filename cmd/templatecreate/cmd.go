package templatecreate

import (
	"fmt"
	"os"

	"github.com/brian1917/workloader/cmd/svcexport"

	"github.com/brian1917/workloader/cmd/ruleexport"

	"github.com/brian1917/workloader/cmd/rulesetexport"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Global variables
var directory string
var ruleSetNames []string
var templateName string
var pce illumioapi.PCE
var err error

// TemplateCreateCmd runs the template export command
var TemplateCreateCmd = &cobra.Command{
	Use:   "template-create [space separated list of rulesets]",
	Short: "Export an Illumio segmentation template.",
	Long: `
Create an Illumio segmentation template.

Segmentation templates are a set of CSV files. The template-create command will create a template for the rulesets, rules, and services. These templates can be imported using workloader template-import. All IP lists in the rules are converted to Any with the expectation the user will refine on import.

Example commands:

Create a template named Active-Directory based on the ruleset named "ACTIVE-DIRECTORY | PROD":
    workloader template-create "ACTIVE-DIRECTORY | PROD" -n Active-Directory

Create a template based on mutliple rulesets:
    workloader template-create "RULESET1" "RULESET2" -n template_name`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for csv command - %s", err)
		}

		// Set the template file
		if len(args) == 0 {
			fmt.Println("Command requires at least 1 argument for the ruleset name(s) to templatize. See usage help.")
			os.Exit(0)
		}
		ruleSetNames = args

		importTemplate()
	},
}

func init() {

	TemplateCreateCmd.Flags().StringVarP(&directory, "directory", "d", "", "directory to export template files to. by default the files are created in the working directory.")
	TemplateCreateCmd.Flags().StringVarP(&templateName, "name", "n", "", "name for the template")
	TemplateCreateCmd.MarkFlagRequired("name")
	TemplateCreateCmd.Flags().SortFlags = false

}

// Process template file
func importTemplate() {

	// Log start of command
	utils.LogStartCommand("template-create")

	// Load the PCE with RuleSets
	if err := pce.Load(illumioapi.LoadInput{RuleSets: true}); err != nil {
		utils.LogError(err.Error())
	}

	// Create the slice of target Hrefs
	var targetRuleSetsHrefs []string
	for _, rsName := range ruleSetNames {
		if val, ok := pce.RuleSets[rsName]; ok {
			targetRuleSetsHrefs = append(targetRuleSetsHrefs, val.Href)
		} else {
			utils.LogError(fmt.Sprintf("%s does not exist as a ruleset in the PCE", rsName))
		}
	}

	// Get the services we need.
	services := make(map[string]bool)
	for _, rsHef := range targetRuleSetsHrefs {
		if rs, ok := pce.RuleSets[rsHef]; ok {
			for _, rule := range rs.Rules {
				for _, svc := range *rule.IngressServices {
					if svc.Href != nil && *svc.Href != "" {
						services[*svc.Href] = true
					}
				}
			}
		}
	}
	serviceHrefs := []string{}
	for svc := range services {
		serviceHrefs = append(serviceHrefs, svc)
	}

	// Export the RuleSets
	fmt.Println("\r\n------------------------------------------ RULE SETS ------------------------------------------")
	rulesetexport.ExportRuleSets(pce, fmt.Sprintf("%s%s.rulesets.csv", directory, templateName), true, targetRuleSetsHrefs)

	// Export the Rules
	fmt.Println("\r\n------------------------------------------- RULES ---------------------------------------------")
	ruleexport.ExportRules(ruleexport.Input{PCE: pce, SkipWkldDetailCheck: true, OutputFileName: fmt.Sprintf("%s%s.rules.csv", directory, templateName), PolicyVersion: "draft", TemplateFormat: true, RulesetHrefs: targetRuleSetsHrefs})

	// Export the services
	fmt.Println("\r\n------------------------------------------ SERVICES -------------------------------------------")
	svcexport.ExportServices(pce, true, fmt.Sprintf("%s%s.services.csv", directory, templateName), serviceHrefs)

	// Get the directory
	if directory == "" {
		directory = "illumio-templates/"
	} else if directory[len(directory)-1:] != string(os.PathSeparator) {
		directory = fmt.Sprintf("%s%s", directory, string(os.PathSeparator))
	}

	utils.LogEndCommand("template-create")
}
