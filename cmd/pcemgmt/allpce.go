package pcemgmt

import (
	"github.com/spf13/cobra"
)

// AllPceCmd runs a command on all PCEs
var AllPceCmd = &cobra.Command{
	Use:   "all-pces",
	Short: "Run a workloadaer command on all PCEs in your pce.yaml file.",
	Long: `
Run a workloadaer command on all pces in your pce.yaml file.

Prepend the all-pces command to any workloader command to run it on all PCEs in the pce.yaml file.

# Example to run a wkld-import to label and/or create unmanaged workloads in all PCEs:
workloader all-pces wkld-import file.csv --update-pce --no-prompt --umwl

# Example to import ip lists to all PCEs
workloader all-pces ipl-import iplists.csv --update-pce --no-prompt --provision
`,
	Run: func(cmd *cobra.Command, args []string) {
		// Just a place holder function for help menu
		// Logic is processed in main.go
	},
}

// AllPceCmd runs a command on all PCEs
var TargetPcesCmd = &cobra.Command{
	Use:   "target-pces [pce-file] '[workloader command]'",
	Short: "Run a workloadaer command on target PCEs in your pce.yaml file.",
	Long: `
Run a workloadaer command on target pces in your pce.yaml file.

Prepend the target-pces command and the location of a file listing the PCEs to any workloader command to run it on the target PCEs.

# Example to run a wkld-import to label and/or create unmanaged workloads in all PCEs:
workloader target-pces pces.csv wkld-import file.csv --update-pce --no-prompt --umwl

# Example to import ip lists to all PCEs
workloader target-pces pces.csv ipl-import iplists.csv --update-pce --no-prompt --provision
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Just a place holder function for help menu
		// Logic is processed in main.go
	},
}
