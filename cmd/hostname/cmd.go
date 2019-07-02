package hostname

import (
	"log"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var configFile, parserFile, hostFile, outputFile, appFlag, roleFlag, envFlag, locFlag string
var debugLogging, noPrompt, logonly, allEmpty, ignoreMatch, noPCE, verbose bool
var pce illumioapi.PCE
var err error

func init() {

	HostnameCmd.Flags().StringVarP(&parserFile, "parserfile", "p", "", "Location of CSV with regex functions and labels.")
	HostnameCmd.Flags().StringVar(&hostFile, "hostfile", "", "Location of hostnames CSV to parse.")
	HostnameCmd.Flags().StringVarP(&roleFlag, "role", "e", "", "Environment label.")
	HostnameCmd.Flags().StringVarP(&appFlag, "app", "a", "", "App label.")
	HostnameCmd.Flags().StringVarP(&envFlag, "env", "r", "", "Role label.")
	HostnameCmd.Flags().StringVarP(&locFlag, "loc", "l", "", "Location label.")
	HostnameCmd.Flags().BoolVar(&noPrompt, "noprompt", false, "No prompting.")
	HostnameCmd.Flags().BoolVar(&allEmpty, "allempty", false, "All empty.")
	HostnameCmd.Flags().BoolVar(&ignoreMatch, "ignorematch", false, "Ignore match.")
	HostnameCmd.Flags().BoolVar(&noPCE, "nopce", false, "No PCE.")
	HostnameCmd.Flags().BoolVar(&debugLogging, "verbose", false, "Verbose logging.")
	HostnameCmd.Flags().BoolVar(&logonly, "logonly", false, "Set to only log changes. Don't update the PCE.")
	HostnameCmd.Flags().SortFlags = false

	pce, err = utils.GetPCE("pce.json")
	if err != nil {
		log.Fatalf("[ERROR] - getting PCE for traffic command - %s", err)
	}

}

// HostnameCmd runs the hostname parser
var HostnameCmd = &cobra.Command{
	Use:   "hostname",
	Short: "Label workloads by parsing hostnames from provided regex functions.",
	Long: `
Label workloads by parsing hostnames.

An input CSV specifics the regex functions to use to assign labels. An example is below:

PLACEHOLDER FOR SAMPLE TABLE`,
	Run: func(cmd *cobra.Command, args []string) {

		hostnameParser()
	},
}
