package hostparse

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set up global variables
var parserFile, hostFile, appFlag, roleFlag, envFlag, locFlag, outputFileName string
var debug, noPrompt, updatePCE, allWklds bool
var capitalize int
var pce illumioapi.PCE
var err error

// Init function will handle flags
func init() {
	HostnameCmd.Flags().StringVar(&hostFile, "hostfile", "", "Location of optional CSV file with target hostnames parse. Used instead of getting workloads from the PCE.")
	HostnameCmd.Flags().StringVarP(&roleFlag, "role", "r", "", "Role label to identify workloads to parse hostnames. No value will look for workloads with no role label.")
	HostnameCmd.Flags().StringVarP(&appFlag, "app", "a", "", "Application label to identify workloads to parse hostnames. No value will look for workloads with no application label.")
	HostnameCmd.Flags().StringVarP(&envFlag, "env", "e", "", "Environment label to identify workloads to parse hostnames. No value will look for workloads with no environment label.")
	HostnameCmd.Flags().StringVarP(&locFlag, "loc", "l", "", "Location label to identify workloads to parse hostnames. No value will look for workloads with no location label.")
	HostnameCmd.Flags().BoolVar(&allWklds, "all", false, "Parse all PCE workloads no matter what labels are assigned. Individual label flags are ignored if set.")
	HostnameCmd.Flags().IntVar(&capitalize, "capitalize", 1, "Set 1 for uppercase labels(default), 2 for lowercase labels or 0 to leave capitalization as is in parsed hostname.")
	HostnameCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")

	HostnameCmd.Flags().SortFlags = false

}

// HostnameCmd runs the hostname parser
var HostnameCmd = &cobra.Command{
	Use:   "hostparse [parser file csv]",
	Short: "Label workloads by parsing hostnames from provided regex functions.",
	Long: `
Label workloads by parsing hostnames.

An input CSV specifics the regex functions to use to assign labels. An example is below:

+-----------------------------------------------------+------+------+-----------+-----------+
|                        REGEX                        | ROLE | APP  |    ENV    | LOC       |
+-----------------------------------------------------+------+------+-----------+-----------+
| ([A-Za-z]{3})-([4]).*                               |      | ${1} | CERT      |           |
| ([A-Za-z]{3})-([7]).*                               |      | ${1} | DEV       |           |
| ([A-Za-z0-9]*)\.([A-Za-z0-9]*)\.([A-Za-z0-9]*)\.\w+ | ${1} | ${2} |           |           |
| (h)(3)-(\w*)-([sd])(\d+)                            | APP  | ${3} | SITE${5}  | Amazon    |
| (h)(6)-(\w*)-([sd])(\d+)                            | DB   | ${3} | SITE${5}  | Amazon    |
+-----------------------------------------------------+------+------+-----------+-----------+


`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetTargetPCE(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get persistent flags from Viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		// Get CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}
		parserFile = args[0]

		hostnameParser()
	},
}

//data structure built from the parser.csv
type regex struct {
	Regexdata []regexstruct
}

//regex structure with regex and array of replace regex to build the labels
type regexstruct struct {
	regex   string
	labelcg map[string]string
}

//ReadCSV - Open CSV for hostfile and parser file
func ReadCSV(file string) [][]string {
	csvfile, err := os.Open(file)
	if err != nil {
		utils.LogError(err.Error())
	}
	defer csvfile.Close()

	if err != nil {
		utils.LogInfo(fmt.Sprint(err), false)
		//utils.Logger.Println(err)
		os.Exit(1)
	}

	reader := csv.NewReader(csvfile)
	reader.FieldsPerRecord = -1

	rawCSVdata, err := reader.ReadAll()
	if err != nil {
		utils.LogInfo(fmt.Sprint(err), false)
		os.Exit(1)
	}

	return rawCSVdata
}

// RelabelFromHostname function - Regex method to provide labels for the hostname provided
func (r *regex) RelabelFromHostname(failedPCE bool, wkld illumioapi.Workload, lbls map[string]string, nolabels map[string]string, outputfile *os.File) (bool, illumioapi.Workload) {

	//var templabels []string
	var match bool

	// Copy the workload struct to save to new updated workload struct if needed.
	tmpwkld := wkld

	searchname := wkld.Hostname

	if searchname == "" {
		utils.LogInfo(fmt.Sprintf("**** No Hostname string configured on the workload. Name : %s, HRef : %s", wkld.Name, wkld.Href), false)
	} else {
		utils.LogInfo(fmt.Sprintf("REGEX Match For - %s", searchname), false)
	}

	for _, tmp := range r.Regexdata {

		//Place match regex into regexp data struct
		tmpre := regexp.MustCompile(tmp.regex)

		//Is there a match using the regex?
		match = tmpre.MatchString(searchname)

		//Report  if we have a match, regex and replacement regex per label
		if debug && !match {
			utils.LogDebug(fmt.Sprintf("%s - Regex: %s - Match: %t", searchname, tmp.regex, match))
		}

		// if the Regex matches the hostname string cycle through the label types and extract the desired labels.
		// Makes sure the labels have the right capitalization. Write the old labels and new labels to the output file
		// keep all the labels that arent currently configured on the PCE to be added if NOPrompt or UpdatePCE
		if match {
			utils.LogInfo(fmt.Sprintf("%s - Regex: %s - Match: %t", searchname, tmp.regex, match), false)
			// Save the labels that are existing
			orgLabels := make(map[string]*illumioapi.Label)
			for _, l := range *wkld.Labels {
				orgLabels[l.Key] = l
			}

			var tmplabels []*illumioapi.Label
			for _, label := range []string{"loc", "env", "app", "role"} {

				//get the string returned from the replace regex.
				tmpstr := changeCase(strings.Trim(tmpre.ReplaceAllString(searchname, tmp.labelcg[label]), " "))

				var tmplabel illumioapi.Label

				//If regex produced an output add that as the label.
				if tmpstr != "" {

					//add Key, Value and if available the Href.  Without Href we can skip if user doesnt want to new labels.
					if lbls[label+"."+tmpstr] != "" {
						tmplabel = illumioapi.Label{Href: lbls[label+"."+tmpstr], Key: label, Value: tmpstr}
					} else {

						//create an entry for the label type and value into the Href map...Href is empty to start
						lbls[label+"."+tmpstr] = ""

						//create a list of labels that arent currently configured on the PCE that the replacement regex  wants.
						//only get labels for workloads that have HREFs...
						if updatePCE || !failedPCE {
							if tmpwkld.Href != "" {
								nolabels[label+"."+tmpstr] = ""
							}
						} else {
							nolabels[label+"."+tmpstr] = ""
						}
						//Build a label variable with Label type and Value but no Href due to the face its not configured on the PCE
						tmplabel = illumioapi.Label{Key: label, Value: tmpstr}

					}

					// If the regex doesnt produce a replacement or there isnt a replace regex in the CSV then copy orginial label
				} else {
					//fmt.Println(orgLabels[label])
					if orgLabels[label] != nil {
						tmplabel = *orgLabels[label]

					} else {
						continue
					}

				}
				tmplabels = append(tmplabels, &tmplabel)
				//Add Label array to the workload.
				*tmpwkld.Labels = tmplabels
			}

			//Get the original labels and new labels to show the changes.
			orgRole, orgApp, orgEnv, orgLoc := labelvalues(*wkld.Labels)
			role, app, env, loc := labelvalues(*tmpwkld.Labels)

			if debug {
				utils.LogInfo(fmt.Sprintf("%s - Replacement Regex: %+v - Labels: %s - %s - %s - %s", searchname, tmp.labelcg, role, app, env, loc), false)
			}
			utils.LogInfo(fmt.Sprintf("%s - Current Labels: %s, %s, %s, %s Replaced with: %s, %s, %s, %s", searchname, orgRole, orgApp, orgEnv, orgLoc, role, app, env, loc), false)

			// Write out ALL the hostnames with new and old labels in output file
			fmt.Fprintf(outputfile, "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\r\n", tmpwkld.Hostname, role, app, env, loc, tmpwkld.Href, orgRole, orgApp, orgEnv, orgLoc, tmp.regex, tmp.labelcg)
			return match, tmpwkld
		}

	}
	utils.LogInfo(fmt.Sprintf("**** NO REGEX MATCH FOUND **** - %s -", searchname), false)
	//return there was no match for that hostname
	orgRole, orgApp, orgEnv, orgLoc := labelvalues(*wkld.Labels)
	role, app, env, loc := "", "", "", ""
	fmt.Fprintf(outputfile, "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\r\n", tmpwkld.Hostname, role, app, env, loc, tmpwkld.Href, orgRole, orgApp, orgEnv, orgLoc, "", "")
	return match, tmpwkld
}

//Load the Regex CSV Into the parser struct -
func (r *regex) load(data [][]string) {

	//Cycle through all the parse data rows in the parse data xls
	for c, row := range data {

		var tmpr regexstruct
		//ignore header
		if c != 0 {

			//Array order 0-Role,1-App,2-Env,3-Loc
			tmpmap := make(map[string]string)
			for x, lbl := range []string{"role", "app", "env", "loc"} {
				//place CSV column in map
				tmpmap[lbl] = row[x+1]
			}
			//Put the regex string and capture groups into data structure
			tmpr.regex = row[0]
			tmpr.labelcg = tmpmap

			r.Regexdata = append(r.Regexdata, tmpr)
		}

	}
}

//updatedLabels - Function to update  workload with new labels
func updateLabels(w *illumioapi.Workload, lblhref map[string]illumioapi.Label) {

	var tmplbls []*illumioapi.Label
	for _, lbl := range *w.Labels {
		tmplbl := lblhref[lbl.Href]
		tmplbls = append(tmplbls, &tmplbl)
	}
	*w.Labels = tmplbls
}

//labelvalues - Return all the Label values from the labels of a workload
func labelvalues(labels []*illumioapi.Label) (string, string, string, string) {

	loc, env, app, role := "", "", "", ""
	for _, l := range labels {
		switch l.Key {
		case "loc":
			loc = l.Value
		case "env":
			env = l.Value
		case "app":
			app = l.Value
		case "role":
			role = l.Value
		}
	}
	return role, app, env, loc
}

// changeCase - upperorlower function check to see if user set capitalization to ignore/no change(0 default), upper (1) or lower (2)
func changeCase(str string) string {

	switch capitalize {
	case 0:
		return str
	case 1:
		return strings.ToUpper(str)
	case 2:
		return strings.ToLower(str)
	default:
		return str
	}
}

//hostnameParser - Main function to parse hostnames either on the PCE on in a hostfile using regex file and created labels from results.
func hostnameParser() {

	// Log the start of the command
	utils.LogStartCommand("hostparse")

	// Set output file
	if outputFileName == "" {
		outputFileName = "workloader-hostparse-" + time.Now().Format("20060102_150405") + ".csv"
	}

	// Log configuration
	if debug {
		name := []string{"update-pce", "no-prompt", "all", "role", "app", "env", "loc", "capitalize", "hostfile", "parsefile"}
		value := []string{strconv.FormatBool(updatePCE), strconv.FormatBool(noPrompt), strconv.FormatBool(allWklds), roleFlag, appFlag, envFlag, locFlag, strconv.Itoa(capitalize), hostFile, parserFile}
		for i, n := range name {
			utils.LogInfo(fmt.Sprintf("%s set to %s ", n, value[i]), false)
		}
	}

	//Read the Regex Parsing CSV.   Format should be match Regex and replace regex per label {}
	var parserec [][]string

	// If no parserFile, log fatal the error. Otherwise, open it.
	if parserFile == "" {
		utils.LogError("no Hostname parser file provide - set the parser file location via --parserfile (-p) ")
	}
	parserec = ReadCSV(parserFile)
	if debug {
		utils.LogDebug(fmt.Sprintf("hostparser - open parser file - %s", parserFile))
	}

	var data regex
	// Load the regex data into the regex struct
	data.load(parserec)

	//Make the Workload Output table object for the console
	matchtable := tablewriter.NewWriter(os.Stdout)
	matchtable.SetAlignment(tablewriter.ALIGN_LEFT)
	matchtable.SetHeader([]string{"Hostname", "New-Role", "New-App", "New-Env", "New-Loc", "Org-Role", "Org-App", "Org-Env", "org-Loc"})

	//Make the Label Output table object for the console
	labeltable := tablewriter.NewWriter(os.Stdout)
	labeltable.SetAlignment(tablewriter.ALIGN_LEFT)
	labeltable.SetHeader([]string{"Type", "Value"})

	failedPCE := false
	//Access PCE to get all Labels only if no_pce is not set to true in config file
	labels, apiResp, err := pce.GetAllLabels()
	if debug {
		utils.LogAPIResp("GetAllLabels", apiResp)
	}
	if err != nil {
		debug = true
		updatePCE = false
		failedPCE = true
		utils.LogInfo("error accessing PCE API - Skipping further PCE API calls", false)
		if debug {
			utils.LogDebug(fmt.Sprintf("Get All Labels Error: %s", err))
		}
	}
	var workloads []illumioapi.Workload

	if !failedPCE {
		workloads, apiResp, err = pce.GetAllWorkloads()
		if debug {
			utils.LogAPIResp("GetAllWorkloads", apiResp)
		}
		if err != nil {
			utils.LogDebug(fmt.Sprintf("Get All Workloads Error: %s", err))
			failedPCE = true
		}
	}
	//Map struct for labels using 'key'.'value' as the map key.
	lblskv := make(map[string]string)
	//Map struct for labels using labe 'href' as the map key.
	lblshref := make(map[string]illumioapi.Label)
	for _, l := range labels {
		lblskv[l.Key+"."+l.Value] = l.Href
		lblshref[l.Href] = l
	}

	//create Label array with all the HRefs as value with label type and label key combined as the key "key.value"
	if debug && !failedPCE {
		utils.LogDebug(fmt.Sprintf("Build Map of HREFs with a key that uses a label's type and value eg. 'type.value': %v", lblskv))

	}

	//Create variables for wor
	var alllabeledwrkld []illumioapi.Workload
	nolabels := make(map[string]string)

	//Create output file
	var outputFile *os.File
	outputFile, err = os.Create(outputFileName)
	if err != nil {
		utils.Logger.Fatalf("ERROR - Creating file - %s\n", err)
	}
	defer outputFile.Close()

	fmt.Fprintf(outputFile, "hostname,role,app,env,loc,href,prev-role,prev-app,prev-env,prev-loc,regex\r\n")

	var wkld []illumioapi.Workload
	if hostFile != "" {
		wkldsHN, a, err := pce.GetWkldHostMap()
		if debug {
			utils.LogAPIResp("GetWkldHostMap", a)
		}
		if err != nil {
			utils.LogError(err.Error())
		}
		hostrec := ReadCSV(hostFile)
		var tmpwkld illumioapi.Workload
		for c, row := range hostrec {
			if c != 0 {
				w, ok := wkldsHN[row[0]]
				if ok {
					tmpwkld = w
				} else {
					tmpwkld = illumioapi.Workload{Hostname: row[0]}
				}
				wkld = append(wkld, tmpwkld)
			}
		}
	} else {
		wkld = workloads
	}

	//Cycle through all the workloads
	for _, w := range wkld {

		//Check to see

		updateLabels(&w, lblshref)
		if w.LabelsMatch(roleFlag, appFlag, envFlag, locFlag, lblshref) || allWklds {

			match, labeledwrkld := data.RelabelFromHostname(failedPCE, w, lblskv, nolabels, outputFile)
			orgRole, orgApp, orgEnv, orgLoc := labelvalues(*w.Labels)
			role, app, env, loc := labelvalues(*labeledwrkld.Labels)

			if match {
				if labeledwrkld.Href != "" && !(role == orgRole && app == orgApp && env == orgEnv && loc == orgLoc) {
					matchtable.Append([]string{labeledwrkld.Hostname, role, app, env, loc, orgRole, orgApp, orgEnv, orgLoc})
					alllabeledwrkld = append(alllabeledwrkld, labeledwrkld)
				} else if labeledwrkld.Href == "" && !updatePCE {
					matchtable.Append([]string{labeledwrkld.Hostname, role, app, env, loc, orgRole, orgApp, orgEnv, orgLoc})
					utils.LogInfo(fmt.Sprintf("SKIPING UPDATE - %s - No Workload on the PCE", labeledwrkld.Hostname), false)
				} else {
					utils.LogInfo(fmt.Sprintf("SKIPING UPDATE - %s - No Label Change Required", labeledwrkld.Hostname), false)

				}
			}

		}

	}

	//Capture all the labels that need to be created and make them ready for display.
	var tmplbls []illumioapi.Label
	if len(nolabels) > 0 && len(alllabeledwrkld) > 0 {

		for keylabel := range nolabels {
			key, value := strings.Split(keylabel, ".")[0], strings.Split(keylabel, ".")[1]
			tmplbls = append(tmplbls, illumioapi.Label{Value: value, Key: key})
			labeltable.Append([]string{key, value})
			//Make sure we arent only looking for an output file and we have the ability to access the PCE.

		}
		if !noPrompt {
			labeltable.Render()
			fmt.Print("***** Above Labels Not Configured on the PCE ***** \r\n")
		}
	}

	var response string
	// Print table with all the workloads and the new labels.
	if len(alllabeledwrkld) > 0 {
		if !noPrompt {
			matchtable.Render()

		}
		response = "no"
		//check if noprompt is set to true or you want to update....Skip bulk upload of workload labels.
		if noPrompt {
			response = "yes"
		} else if updatePCE {
			fmt.Printf("Do you want to update Workloads and potentially create new labels in %s (%s) (yes/no)? ", pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))
			fmt.Scanln(&response)
		} else {
			fmt.Println("List of ALL Regex Matched Hostnames even if no Workloada exist on the PCE. ")
		}

		//If updating is selected and the NOPCE option has not been invoked then update labels and workloads.
		if response == "yes" && !failedPCE {

			if debug {
				utils.LogDebug("*********************************LABEL CREATION***************************************")
			}
			for _, lbl := range tmplbls {
				newLabel, apiResp, err := pce.CreateLabel(lbl)

				if err != nil {
					utils.LogError(err.Error())
				}
				if debug {
					utils.LogDebug(fmt.Sprintf("Exact label does not exist for %s (%s). Creating new label... ", lbl.Value, lbl.Value))
					utils.LogAPIResp("CreateLabel", apiResp)
				} else {
					utils.LogInfo(fmt.Sprintf("CREATED LABEL %s (%s) with following HREF: %s", newLabel.Value, newLabel.Key, newLabel.Href), false)
				}
				lblskv[lbl.Key+"."+lbl.Value] = newLabel.Href
			}
			if debug {
				utils.LogDebug("*********************************WORKLOAD BULK UPDATE***************************************")
			}
			for _, w := range alllabeledwrkld {
				for _, l := range *w.Labels {
					if l.Href == "" {
						l.Href = lblskv[l.Key+"."+l.Value]
					}
				}
			}
			// Send parsed workloads and new labels to BulkUpdate
			apiResp, err := pce.BulkWorkload(alllabeledwrkld, "update", true)

			//get number of workloads to update
			utils.LogInfo(fmt.Sprintf("running bulk update on %d workloads. batches run in 1,0000 workload chunks", len(alllabeledwrkld)), false)
			for i, api := range apiResp {
				if debug {
					utils.LogAPIResp("BulkWorkloadUpdate", api)
				}
				// Log our error if there is an error
				if err != nil {
					utils.LogInfo(err.Error(), false)
				}
				// If not doing debug level logging, log each complete api
				if !debug {
					utils.LogInfo(fmt.Sprintf("bulkworkload update batch %d completed", i), false)
				}
			}

		}
	} else {
		//Make sure to put NO MATCHES into output file
		utils.LogInfo("No Workloads will me updated  -  Check the output file", false)

		if !noPrompt && !failedPCE {
			fmt.Println("***** There were no hostnames that needed updating or matched an entry in the 'parsefile'****")
		} else if failedPCE {
			fmt.Println("**** PCE Error **** Cannot update Labels or Hostnames to Upload **** Check Output file ****")
		}
	}
	utils.LogEndCommand("hostparse")
}
