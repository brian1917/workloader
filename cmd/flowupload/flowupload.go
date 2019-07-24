package flowupload

import (
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// FlowCmd uploads traffic flows for view in Illumination
var FlowCmd = &cobra.Command{
	Use:   "flowupload",
	Short: "Upload flows to PCE from a CSV file.",
	Run: func(cmd *cobra.Command, args []string) {

		_, err := utils.GetPCE()
		if err != nil {
			utils.Logger.Fatalf("Error getting PCE for flow upload command - %s", err)
		}
	},
}
