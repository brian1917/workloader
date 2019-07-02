package mode

import (
	"log"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Set global variables for flags
var hostFile string
var logOnly, verbose bool
var hrefCol, desiredStateCol int
var pce illumioapi.PCE
var err error

func init() {

	ModeCmd.Flags().StringVarP(&hostFile, "input", "i", "", "Input CSV file.")
	ModeCmd.Flags().IntVar(&hrefCol, "hrefCol", 1, "Column with HREF value. First column is 1.")
	ModeCmd.Flags().IntVar(&desiredStateCol, "stateCol", 2, "Column with desired state value.")
	ModeCmd.Flags().BoolVarP(&logOnly, "logonly", "l", false, "Will not make changes in PCE. Will log potential changes.")
	ModeCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose logging.")

	ModeCmd.Flags().SortFlags = false

	pce, err = utils.GetPCE("pce.json")
	if err != nil {
		log.Fatalf("Error getting PCE for traffic command - %s", err)
	}

}

// ModeCmd runs the hostname parser
var ModeCmd = &cobra.Command{
	Use:   "mode",
	Short: "Change the state of workloads based on a CSV input.",
	Long: `
Change a workload's state based on an input CSV with at least two columns: workload HREF and desired statestate.

The state must be either idle, build, test, or enforced.

An example is below:

+--------------------------------------------------------+----------+
|                          href                          |  state   |
+--------------------------------------------------------+----------+
| /orgs/1/workloads/721d1621-31a6-40a0-a0cb-1e4b1c051210 | build    |
| /orgs/1/workloads/d1e6266c-0b07-4b6e-b68f-c3f2386bdf08 | test     |
| /orgs/1/workloads/77d72edc-8734-4a5d-a01d-d055898e6ba1 | enforced |
+--------------------------------------------------------+----------+

If the CSV does not follow the default format (href in col 1 and state in col 2), use flags to specify columns.`,
	Run: func(cmd *cobra.Command, args []string) {

		modeUpdate()
	},
}
