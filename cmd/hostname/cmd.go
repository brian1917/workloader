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

	HostnameCmd.Flags().StringP("parserfile", "p", "", "Location of CSV with regex functions and labels.")
	HostnameCmd.Flags().String("hostfile", "", "Location of hostnames CSV to parse.")
	HostnameCmd.Flags().StringP("role", "e", "", "Environment label.")
	HostnameCmd.Flags().StringP("app", "a", "", "App label.")
	HostnameCmd.Flags().StringP("env", "r", "", "Role label.")
	HostnameCmd.Flags().StringP("loc", "l", "", "Location label.")
	HostnameCmd.Flags().Bool("noprompt", false, "No prompting.")
	HostnameCmd.Flags().Bool("allempty", false, "All empty.")
	HostnameCmd.Flags().Bool("ignorematch", false, "Ignore match.")
	HostnameCmd.Flags().Bool("nopce", false, "No PCE.")
	HostnameCmd.Flags().Bool("verbose", false, "Verbose logging.")
	HostnameCmd.Flags().Bool("logonly", false, "Set to only log changes. Don't update the PCE.")
	HostnameCmd.Flags().SortFlags = false

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
		configFile, _ = cmd.Flags().GetString("config")
		parserFile, _ = cmd.Flags().GetString("parserfile")
		noPrompt, _ = cmd.Flags().GetBool("noprompt")
		hostFile, _ = cmd.Flags().GetString("hostfile")
		allEmpty, _ = cmd.Flags().GetBool("allempty")
		noPCE, _ = cmd.Flags().GetBool("nopce")
		ignoreMatch, _ = cmd.Flags().GetBool("ignorematch")
		appFlag, _ = cmd.Flags().GetString("app")
		roleFlag, _ = cmd.Flags().GetString("role")
		envFlag, _ = cmd.Flags().GetString("env")
		locFlag, _ = cmd.Flags().GetString("loc")
		logonly, _ = cmd.Flags().GetBool("logonly")
		debugLogging, _ = cmd.Flags().GetBool("verbose")

		pce, err = utils.GetPCE("pce.json")
		if err != nil {
			log.Fatalf("Error getting PCE for traffic command - %s", err)
		}

		hostnameParser()
	},
}
