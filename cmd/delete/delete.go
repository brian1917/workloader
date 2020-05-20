package delete

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var hrefFile string
var debug, updatePCE, noPrompt bool
var pce illumioapi.PCE
var err error

// DeleteCmd runs the unpair
var DeleteCmd = &cobra.Command{
	Use:   "delete [csv file with hrefs to delete]",
	Short: "Delete unmanaged workloads by HREFs provided in file.",
	Long: `  
	Delete any object with an HREF (e.g., unmanaged workloads, labels, services, IPLists, etc.) from the PCE.

Default output is a CSV file with what would be deleted.
Use the --update-pce command to run the delete with a user prompt confirmation.
Use --update-pce and --no-prompt to run the delete with no prompts.`,
	Run: func(cmd *cobra.Command, args []string) {
		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		hrefFile = args[0]

		// Get persistent flags from Viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		delete()
	},
}

func delete() {

	utils.LogStartCommand("delete")

	// Get all workloads
	wkldMap, a, err := pce.GetWkldHrefMap()
	utils.LogAPIResp("GetAllWkldHrefMap", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// Get all HREFs from the CSV file
	csvFile, _ := os.Open(hrefFile)
	reader := csv.NewReader(bufio.NewReader(csvFile))
	hrefs := []string{}
	for {
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.LogError(fmt.Sprintf("Reading CSV File - %s", err))
		}
		hrefs = append(hrefs, line[0])
	}

	// Create a CSV with the unpairs
	outFile, err := os.Create("workloader-delete-" + time.Now().Format("20060102_150405") + ".csv")
	if err != nil {
		utils.LogError(fmt.Sprintf("creating CSV - %s\n", err))
	}

	// Build the data slice for writing
	deleteCounter := 0
	data := [][]string{[]string{"href", "hostname", "role", "app", "env", "loc", "status"}}
	deleteWorkloads := []illumioapi.Workload{}
	for _, h := range hrefs {
		// Check if it is a workload
		if _, ok := wkldMap[h]; !ok {
			data = append(data, []string{h, "NOT IN PCE", "NA", "NA", "NA", "NA", "workload does not exist - skipped"})
			continue
		}

		// If it is a workload, create the variable to stop using map so we can run methods
		w := wkldMap[h]

		// Check if it's unmanaged
		if w.GetMode() != "unmanaged" {
			data = append(data, []string{h, w.Hostname, w.GetRole(pce.LabelMapH).Value, w.GetApp(pce.LabelMapH).Value, w.GetEnv(pce.LabelMapH).Value, w.GetLoc(pce.LabelMapH).Value, "managed workload - skipped"})
			continue
		}

		// Add to deleted list
		deleteCounter++
		// Add to the slice to be sent to bulk delete
		deleteWorkloads = append(deleteWorkloads, w)
		data = append(data, []string{w.Href, w.Hostname, w.GetRole(pce.LabelMapH).Value, w.GetApp(pce.LabelMapH).Value, w.GetEnv(pce.LabelMapH).Value, w.GetLoc(pce.LabelMapH).Value, "to be deleted"})
	}

	// Write CSV data
	writer := csv.NewWriter(outFile)
	writer.WriteAll(data)
	if err := writer.Error(); err != nil {
		utils.LogError(fmt.Sprintf("writing CSV - %s\n", err))
	}

	// If updatePCE is disabled, we are just going to alert the user what will happen and log
	if !updatePCE {
		utils.LogInfo(fmt.Sprintf("delete identified %d workloads to be deleted - see %s for details.", deleteCounter, outFile.Name()))
		fmt.Printf("Delete identified %d workloads to be deleted. See %s for details. To do the delete, run again using --update-pce flag. The --no-prompt flag will bypass the prompt if used with --update-pce.\r\n", deleteCounter, outFile.Name())
		utils.LogInfo("completed running delete command")
		return
	}

	// If updatePCE is set, but not noPrompt, we will prompt the user.
	if updatePCE && !noPrompt {
		var prompt string
		fmt.Printf("Delete identified %d workloads to be deleted. See %s for details. Do you want to run the deletion? (yes/no)? ", deleteCounter, outFile.Name())
		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo(fmt.Sprintf("Delete identified %d workloads to be deleted - see %s for details. user denied prompt", deleteCounter, outFile.Name()))
			fmt.Println("Prompt denied.")
			utils.LogInfo("completed running delete command")
			return
		}
	}

	// We will only get here if we have need to run the delete
	apiResps, err := pce.BulkWorkload(deleteWorkloads, "delete")
	for _, a := range apiResps {
		utils.LogAPIResp("bulk delete workloads", a)
	}
	if err != nil {
		utils.LogError(err.Error())
	}
	utils.LogEndCommand("delete")
}
