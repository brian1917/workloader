package edgeruleexport

import (
	"github.com/brian1917/workloader/cmd/ruleexport"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Declare local global variables
var pce illumioapi.PCE
var err error
var debug, useActive, expandSVCs bool
var outFormat, group string

// Init handles flags
func init() {

	EdgeRuleExportCmd.Flags().BoolVar(&useActive, "active", false, "Use active policy versus draft. Draft is default.")
	EdgeRuleExportCmd.Flags().BoolVarP(&expandSVCs, "expand-services", "e", false, "Expand all services. Do not use if re-importing to edit/add rules.")
	EdgeRuleExportCmd.Flags().SortFlags = false

}

// EdgeRuleExportCmd runs the workload identifier
var EdgeRuleExportCmd = &cobra.Command{
	Use:   "edge-rule-export",
	Short: "Create a CSV export of all rules in the Edge PCE.",
	Long: `
Create a CSV export of all rules in the Edge PCE.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the viper values
		debug = viper.Get("debug").(bool)
		outFormat = viper.Get("output_format").(string)

		exportEdgeRules()
	},
}

func exportEdgeRules() {
	ruleexport.ExportRules(pce, useActive, "", "", "", true, expandSVCs, debug)
}
