package flowsummary

import (
	"fmt"

	"github.com/spf13/cobra"
)

// FlowSummaryCmd calls the CLI
var FlowSummaryCmd = &cobra.Command{
	Use:   "flowsummary",
	Short: "Summarize flows from explorer. Two subcommands: appgroup (available now) and env (coming soon).",
	Long: `
Summarize flows between app groups or into envrionments.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Command requires a sub command: appgroup or envrionment")
	},
}

// Init builds the commands
func init() {

	// Disable sorting
	cobra.EnableCommandSorting = false

	// Add all commands
	FlowSummaryCmd.AddCommand(AppGroupFlowSummaryCmd)

}
