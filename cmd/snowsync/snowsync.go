package snowsync

import (
	"fmt"
	"net/url"
	"os"

	"github.com/brian1917/workloader/cmd/wkldimport"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Global variables
var snowTable, snowUser, snowPwd, snowMatchField, snowRole, snowApp, snowEnv, snowLoc, snowIP string
var umwl, keepTempFile, keepAllPCEInterfaces, fqdnToHostname, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

func init() {

	SnowSyncCmd.Flags().StringVarP(&snowTable, "snow-table", "t", "", "The URL of the ServiceNow table (e.g., https://test.service-now.com/u_table.do")
	SnowSyncCmd.Flags().StringVarP(&snowUser, "snow-user", "u", "", "ServiceNow username")
	SnowSyncCmd.Flags().StringVarP(&snowPwd, "snow-pwd", "p", "", "ServiceNow password")
	SnowSyncCmd.Flags().StringVarP(&snowMatchField, "snow-match-field", "m", "", "ServiceNow field name to match to Illumio hostname")
	SnowSyncCmd.Flags().StringVarP(&snowRole, "role", "r", "", "ServiceNow field name for Illumio role")
	SnowSyncCmd.Flags().StringVarP(&snowApp, "app", "a", "", "ServiceNow field name for Illumio app")
	SnowSyncCmd.Flags().StringVarP(&snowEnv, "env", "e", "", "ServiceNow field name for Illumio env")
	SnowSyncCmd.Flags().StringVarP(&snowLoc, "loc", "l", "", "ServiceNow field name for Illumio loc")
	SnowSyncCmd.Flags().BoolVar(&umwl, "umwl", false, "Create unmanaged workloads for non-matches.")
	SnowSyncCmd.Flags().StringVarP(&snowIP, "ip", "i", "", "Field name for IP address. Required if --umwl is set.")
	SnowSyncCmd.Flags().BoolVarP(&keepTempFile, "keep-temp-file", "k", false, "Do not delete the temp CSV file downloaded from SerivceNow.")
	SnowSyncCmd.Flags().BoolVarP(&fqdnToHostname, "fqdn-to-hostname", "f", false, "Convert FQDN hostnames reported by Illumio VEN to short hostnames by removing everything after first period (e.g., test.domain.com becomes test). ")
	SnowSyncCmd.Flags().BoolVarP(&keepAllPCEInterfaces, "keep-all-pce-interfaces", "s", false, "Will not delete an interface on an unmanaged workload if it's not in the import. It will only add interfaces to the workload.")
	SnowSyncCmd.MarkFlagRequired("snow-table")
	SnowSyncCmd.MarkFlagRequired("snow-user")
	SnowSyncCmd.MarkFlagRequired("snow-pwd")
	SnowSyncCmd.MarkFlagRequired("snow-match-field")
	SnowSyncCmd.Flags().SortFlags = false

}

// SnowSyncCmd runs the upload command
var SnowSyncCmd = &cobra.Command{
	Use:   "snow-sync",
	Short: "Label existing workloads and (optionally) create unmanaged workloads from data stored in ServiceNow CMDB.",
	Long: `
** Note - this command uses the SericeNow CSV web service and will be limited by the CSV Format Export Row Limit set in Import Export Properties (e.g., https://test.service-now.com/nav_to.do?uri=%2F$impex_properties.do). The default value is typically 10,000 rows. **

Label existing workloads and (optionally) create unmanaged workloads from data stored in ServiceNow CMDB.

The flags are used to identify the ServiceNow table and to map fields. If a field is not mapped, it will be ignored - no changes to the PCE.

Recommended to run without --update-pce first to log of what will change. If --update-pce is used, import will create labels without prompt, but it will not create/update workloads without user confirmation, unless --no-prompt is used.`,

	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(fmt.Sprintf("Error getting PCE - %s", err.Error()))
		}

		// Get the debug value from viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		snowsync()
	},
}

func snowsync() {

	utils.LogStartCommand("snow-sync")

	// Call the ServiceNow API
	snURL := snowTable + "?CSV&sysparm_fields=" + url.QueryEscape(snowMatchField)
	if snowRole != "" {
		snURL = snURL + "," + url.QueryEscape(snowRole)
	}
	if snowApp != "" {
		snURL = snURL + "," + url.QueryEscape(snowApp)
	}
	if snowEnv != "" {
		snURL = snURL + "," + url.QueryEscape(snowEnv)
	}
	if snowLoc != "" {
		snURL = snURL + "," + url.QueryEscape(snowLoc)
	}
	if umwl {
		snURL = snURL + "," + url.QueryEscape(snowIP)
	}
	fmt.Println(snURL)
	snowCSVFile, err := snhttp(snURL)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Call the workloader import command
	f := wkldimport.Input{
		ImportFile:           snowCSVFile,
		PCE:                  pce,
		Umwl:                 umwl,
		KeepAllPCEInterfaces: keepAllPCEInterfaces,
		FQDNtoHostname:       fqdnToHostname,
		UpdatePCE:            updatePCE,
		NoPrompt:             noPrompt,
	}
	wkldimport.ImportWkldsFromCSV(f)

	// Delete the temp file
	if !keepTempFile {
		if err := os.Remove(snowCSVFile); err != nil {
			utils.LogWarning(fmt.Sprintf("Could not delete %s", snowCSVFile), true)
		} else {
			utils.LogInfo(fmt.Sprintf("Deleted %s", snowCSVFile), false)
		}
	}

	utils.LogEndCommand("snow-sync")

}
