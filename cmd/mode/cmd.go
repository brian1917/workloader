package mode

import (
	"log"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

func init() {

	ModeCmd.Flags().StringP("hostfile", "i", "", "CSV file with two columns: hostname and desired mode. Desired mode can be idle, build, test, or enforce")
	ModeCmd.Flags().BoolP("logonly", "l", false, "Will not make changes in PCE. Will log potential changes.")
	ModeCmd.Flags().BoolP("verbose", "v", false, "Verbose logging.")

	ModeCmd.Flags().SortFlags = false

}

// Set global variables for flags
var hostFile string
var logOnly, verbose bool
var pce illumioapi.PCE
var err error

// ModeCmd runs the hostname parser
var ModeCmd = &cobra.Command{
	Use:   "mode",
	Short: "Change the state of workloads based on a CSV input.",
	Long: `
Change a workload's state based on an input CSV with two columns: hostname and state.

The state must be either idle, build, test, or enforced.

An example is below:

+--------------+-----------+
|   hostname   |   state   |
+--------------+-----------+
| app.test.com |  idle     |
| ntp.test.com |  build    |
| web.test.com |  test     |
| db.test.com  |  enforced |
+--------------+-----------+`,
	Run: func(cmd *cobra.Command, args []string) {

		hostFile, _ = cmd.Flags().GetString("hostfile")
		logOnly, _ = cmd.Flags().GetBool("logonly")
		verbose, _ = cmd.Flags().GetBool("verbose")

		pce, err = utils.GetPCE("pce.json")
		if err != nil {
			log.Fatalf("Error getting PCE for traffic command - %s", err)
		}

		modeUpdate()
	},
}
