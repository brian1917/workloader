package unpair

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var hrefFile, hrefHeader, restore string
var updatePCE, noPrompt, includeOnline, singleAPI, singleUnpair bool
var hoursSinceLastHB int
var pce illumioapi.PCE
var err error

// Init handles flags
func init() {
	UnpairCmd.Flags().StringVar(&restore, "restore", "saved", "restore value. Must be saved, default, or disable.")
	UnpairCmd.Flags().StringVar(&hrefHeader, "header", "ven_href", "column header for hrefs in the href-file.")
	UnpairCmd.Flags().IntVar(&hoursSinceLastHB, "hours", 0, "limit unpairing to workloads that have not sent a heartbeat in set time. 0 will ignore heartbeats.")
	UnpairCmd.Flags().BoolVar(&includeOnline, "include-online", false, "include workloads that are online. by default only offline workloads that meet criteria will be unpaired.")
	UnpairCmd.Flags().BoolVar(&singleAPI, "single-api", false, "if we need to get vens and workloads from the pce query the vens on at a time vs. getting all vens. useful for very large environments that are only unpairing a small amount.")
	UnpairCmd.Flags().BoolVar(&singleUnpair, "single-unpair", false, "one API call per unpair versus one API call per 1000 workloads. this will be significantly slower but provide more details in the pce's syslog messages.")

	UnpairCmd.Flags().SortFlags = false
}

// UnpairCmd runs the unpair
var UnpairCmd = &cobra.Command{
	Use:   "unpair [csv file]",
	Short: "Unpair workloads through an input file.",

	Long: `  
Unpair workloads through an input file.

It's recommended to generate the list of workloads needed by running wkld-export with the right filters. The csv output from wkld-exporft can be passed directly into this command.

Example commands to unpair all VENs that have not sent a heartbeat in 24 hours:
    workloader wkld-export --managed-only --output-file managed_workloads.csv && workloader unpair managed_workloads.csv --hours 24

Use the --update-pce command to run the unpair with a user prompt confirmation.

Use --update-pce and --no-prompt to run unpair with no prompts.`,

	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetTargetPCEV2(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get persistent flags from Viper
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		// Get the input file
		if len(args) != 1 {
			utils.LogError("command requires 1 argument for the csv file. see usage help.")
		}
		hrefFile = args[0]

		unpair()
	},
}

func unpair() {

	// Check the restore value
	restore = strings.ToLower(restore)
	if restore != "saved" && restore != "default" && restore != "disable" {
		utils.LogError("restore value must be saved, default, or disable.")
	}

	// Process the href file
	hrefFileData, err := utils.ParseCSV(hrefFile)
	if err != nil {
		utils.LogErrorf("parsing csv - %s", err)
	}
	venHrefCol := 0
	vens := []illumioapi.VEN{}
	for rowIndex, row := range hrefFileData {
		if rowIndex == 0 {
			for colIndex, col := range row {
				if col == hrefHeader {
					venHrefCol = colIndex
					continue
				}
			}
		} else {
			vens = append(vens, illumioapi.VEN{Href: row[venHrefCol]})
		}
	}

	// If hours since last heartbeat or offline only we need the VENs from the PCE to validate
	// if hoursSinceLastHB > 0 || !includeOnline {
	if !singleAPI {
		apiResps, err := pce.Load(illumioapi.LoadInput{VENs: true, Workloads: true}, utils.UseMulti())
		utils.LogMultiAPIRespV2(apiResps)
		if err != nil {
			utils.LogErrorf("loading the pce - %s", err)
		}
	} else {
		// Get the VENs individually
		pce.VENs = make(map[string]illumioapi.VEN)
		for _, v := range vens {
			ven, api, err := pce.GetVenByHref(v.Href)
			utils.LogAPIRespV2("GetVenByHref", api)
			if err != nil {
				utils.LogErrorf("getting ven - %s", err)
			}
			pce.VENsSlice = append(pce.VENsSlice, ven)
			pce.VENs[ven.Href] = ven
			pce.VENs[illumioapi.PtrToVal(ven.Hostname)] = ven
		}
		// Get the workloads individually
		pce.Workloads = make(map[string]illumioapi.Workload)
		for _, v := range pce.VENsSlice {
			for _, w := range illumioapi.PtrToVal(v.Workloads) {
				wkld, api, err := pce.GetWkldByHref(w.Href)
				utils.LogAPIRespV2("GetWkldByHref", api)
				if err != nil {
					utils.LogErrorf("getting workload - %s", err)
				}
				pce.Workloads[wkld.Href] = wkld
				pce.Workloads[illumioapi.PtrToVal(wkld.Hostname)] = wkld
			}
		}
	}

	// Run validation
	vensToUnpair := []illumioapi.VEN{}
	for i, v := range vens {

		if val, ok := pce.VENs[v.Href]; !ok {
			utils.LogWarningf(true, "csv line %d - %s does not exist in the pce. skipping", i+1, v.Href)
		} else {

			// validate workload assignment
			if (val.Workloads != nil && len(illumioapi.PtrToVal(val.Workloads)) != 1) || val.Workloads == nil {
				utils.LogWarningf(true, "csv line %d - %s has invalid workload assignment. skipping", i+1, val.Href)
			}

			// validate heartbeat
			if hoursSinceLastHB > 0 && val.HoursSinceLastHeartBeat() < float64(hoursSinceLastHB) {
				utils.LogInfof(false, "csv line %d - %s hours since last heartbeat is %d. skipping.", i+1, val.Href, int(val.HoursSinceLastHeartBeat()))
				continue
			}

			// validate online status
			workloads := *val.Workloads
			wkld := pce.Workloads[workloads[0].Href]
			if !includeOnline && *wkld.Online {
				continue
			}

			// Add to the slice
			vensToUnpair = append(vensToUnpair, illumioapi.VEN{Href: val.Href})
		}
	}

	if len(vensToUnpair) == 0 {
		if !includeOnline {
			utils.LogInfo("zero vens identified. The --include-online option was not set so only offline workloads were evaluated.", true)
		} else {
			utils.LogInfo("zero vens identified.", true)
		}
		return
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("workloader identified %d vens requiring unpairing. To do the unpair, run again using --update-pce flag. The --no-prompt flag will bypass the prompt if used with --update-pce.", len(vensToUnpair)), true)
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("%s [PROMPT] - workloader identified %d vens requiring unpairing in %s (%s). Do you want to run the unpair? (yes/no)? ", time.Now().Format("2006-01-02 15:04:05 "), len(vensToUnpair), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("prompt denied.", true)
			return
		}
	}

	// Run the single unpair
	if singleUnpair {
		// Create a slice of slices
		singleTargetVENs := [][]illumioapi.VEN{}
		for _, v := range vensToUnpair {
			singleTargetVENs = append(singleTargetVENs, []illumioapi.VEN{v})
		}

		// Iterate through those for unpairing
		for i, v := range singleTargetVENs {
			apiResps, err := pce.VensUnpair(v, restore)
			utils.LogAPIRespV2("unpair workloads", apiResps[0])
			if err != nil {
				utils.LogError(err.Error())
			}
			// Update progress
			utils.LogInfo(fmt.Sprintf("unpaired %d of %d - %s - status code %d", i+1, len(singleTargetVENs), v[0].Href, apiResps[0].StatusCode), true)
		}
	} else {
		// Run the bulk unpair
		apiResps, err := pce.VensUnpair(vensToUnpair, restore)
		for _, a := range apiResps {
			utils.LogAPIRespV2("VensUnpair", a)
		}
		if err != nil {
			utils.LogError(err.Error())
		}

	}
}
