package hostparse

import (
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

//Data Structure for all the options of the hostparse function in workloader.
type config struct {
	Match match `toml:"match"`
}

type match struct {
	IgnoreMatch bool   `toml:"ignorematch"`
	App         string `toml:"app"`
	Env         string `toml:"env"`
	Loc         string `toml:"loc"`
	Role        string `toml:"role"`
}

// Set up global variables
var parserFile, hostFile, appFlag, roleFlag, envFlag, locFlag, outputFile string
var debug, noPrompt, updatePCE, allWklds bool
var capitalize int
var pce illumioapi.PCE
var err error
var conf config

// Init function will handle flags
func init() {
	HostnameCmd.Flags().StringVarP(&parserFile, "parserfile", "p", "", "Location of CSV file that has the regex functions and labels.")
	HostnameCmd.Flags().StringVar(&hostFile, "hostfile", "", "Location of optional CSV file with target hostnames parse. Used instead of getting workloads from the PCE.")
	HostnameCmd.Flags().StringVarP(&roleFlag, "role", "r", "", "Role label to identify workloads to parse hostnames. No value will look for workloads with no role label.")
	HostnameCmd.Flags().StringVarP(&appFlag, "app", "a", "", "Application label to identify workloads to parse hostnames. No value will look for workloads with no application label.")
	HostnameCmd.Flags().StringVarP(&envFlag, "env", "e", "", "Environment label to identify workloads to parse hostnames. No value will look for workloads with no environment label.")
	HostnameCmd.Flags().StringVarP(&locFlag, "loc", "l", "", "Location label to identify workloads to parse hostnames. No value will look for workloads with no location label.")
	HostnameCmd.Flags().BoolVar(&allWklds, "allworkloads", false, "Parse all PCE workloads no matter what labels are assigned. Individual label flags are ignored if set.")
	HostnameCmd.Flags().IntVar(&capitalize, "capitalize", 1, "Set 1 for uppercase labels(default), 2 for lowercase labels or 0 to leave capitalization as is in parsed hostname.")
	HostnameCmd.Flags().SortFlags = false

}

// HostnameCmd runs the hostname parser
var HostnameCmd = &cobra.Command{
	Use:   "hostparse",
	Short: "Label workloads by parsing hostnames from provided regex functions.",
	Long: `
Label workloads by parsing hostnames.

An input CSV specifics the regex functions to use to assign labels. An example is below:

+-------------------------+------+-------+----------+--------+----------------------+
|          REGEX          | ROLE |  APP  |   ENV    |  LOC   |    SAMPLE    	 	|
+-------------------------+------+-------+----------+--------+----------------------+
| (dc)-(\w*)(\d+)         | DC   | INFRA | ${2}    	| POD{3} | dc-pod2      		|
| (h)(1)-(\w*)-([s])(\d+) | Web  | ${3}  | SITE${5} | AMAZON | h1-app-s1     		|
| (\w*).(\w*).(\w*).(\w*) | ${1} | ${2}  | PROD		| Boston | web.app1.it.com     	|
+-------------------------+------+-------+----------+--------+----------------------+

`,
	Run: func(cmd *cobra.Command, args []string) {

		pce, err = utils.GetPCE()
		if err != nil {
			utils.Log(1, err.Error())
		}

		// Get persistent flags from Viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

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

type lbl struct {
	href  string
	value string
}

//ReadCSV - Open CSV for hostfile and parser file
func ReadCSV(file string) [][]string {
	csvfile, err := os.Open(file)
	defer csvfile.Close()

	if err != nil {
		utils.Log(0, fmt.Sprint(err))
		//utils.Logger.Println(err)
		os.Exit(1)
	}

	reader := csv.NewReader(csvfile)
	reader.FieldsPerRecord = -1

	rawCSVdata, err := reader.ReadAll()
	if err != nil {
		utils.Log(0, fmt.Sprint(err))
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

	var searchname string
	// if conf.Parser.Name {
	// 	searchname = wkld.Name
	// } else {
	searchname = wkld.Hostname
	//}

	if searchname == "" {

		// if conf.Parser.Name {
		// 	utils.Log(0, fmt.Sprintf("**** No Name string configured on the workload.  Hostname - %s", wkld.Hostname))
		// } else {
		utils.Log(0, fmt.Sprintf("**** No Hostname string configured on the workload. Name : %s, HRef : %s", wkld.Name, wkld.Href))
		//}
	} else {
		utils.Log(0, fmt.Sprintf("REGEX Match For - %s", searchname))
	}

	for _, tmp := range r.Regexdata {

		//Place match regex into regexp data struct
		tmpre := regexp.MustCompile(tmp.regex)

		//Is there a match using the regex?
		match = tmpre.MatchString(searchname)

		//Report  if we have a match, regex and replacement regex per label
		if debug && !match {
			utils.Log(2, fmt.Sprintf("%s - Regex: %s - Match: %t", searchname, tmp.regex, match))
		}

		// if the Regex matches the hostname string cycle through the label types and extract the desired labels.
		// Makes sure the labels have the right capitalization. Write the old labels and new labels to the output file
		// keep all the labels that arent currently configured on the PCE to be added if NOPrompt or UpdatePCE
		if match {
			utils.Log(0, fmt.Sprintf("%s - Regex: %s - Match: %t", searchname, tmp.regex, match))
			// Save the labels that are existing
			orgLabels := make(map[string]*illumioapi.Label)
			for _, l := range wkld.Labels {
				orgLabels[l.Key] = l
			}

			var tmplabels []*illumioapi.Label
			for _, label := range []string{"loc", "env", "app", "role"} {

				//get the string returned from the replace regex.
				tmpstr := changecase(strings.Trim(tmpre.ReplaceAllString(searchname, tmp.labelcg[label]), " "))

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
					//tmplabels = append(tmplabels, &tmplabel)

					// If the regex doesnt produce a replacement or there isnt a replace regex in the CSV then copy orginial label
				} else {
					//fmt.Println(orgLabels[label])
					if orgLabels[label] != nil {
						tmplabel = *orgLabels[label]

						//tmplabel = illumioapi.Label{Key: label, Href: orgLabels[label], Value: lblshref[orgLabels[label]].Value}
						//tmplabels = append(tmplabels, &tmplabel)
					} else {
						continue
					}

				}
				tmplabels = append(tmplabels, &tmplabel)
				//Add Label array to the workload.
				tmpwkld.Labels = tmplabels
			}

			//Get the original labels and new labels to show the changes.
			orgRole, orgApp, orgEnv, orgLoc := labelvalues(wkld.Labels)
			role, app, env, loc := labelvalues(tmpwkld.Labels)

			if debug {
				utils.Log(0, fmt.Sprintf("%s - Replacement Regex: %+v - Labels: %s - %s - %s - %s", searchname, tmp.labelcg, role, app, env, loc))
			}
			utils.Log(0, fmt.Sprintf("%s - Current Labels: %s, %s, %s, %s Replaced with: %s, %s, %s, %s", searchname, orgRole, orgApp, orgEnv, orgLoc, role, app, env, loc))

			// Write out ALL the hostnames with new and old labels in output file
			fmt.Fprintf(outputfile, "%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s\r\n", tmpwkld.Hostname, role, app, env, loc, tmpwkld.Href, orgRole, orgApp, orgEnv, orgLoc, tmp.regex, tmp.labelcg)
			return match, tmpwkld
		}

	}
	utils.Log(0, fmt.Sprintf("**** NO REGEX MATCH FOUND **** - %s -", searchname))
	//return there was no match for that hostname
	orgRole, orgApp, orgEnv, orgLoc := labelvalues(wkld.Labels)
	//role, app, env, loc := labelvalues(tmpwkld.Labels)
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

//Method for the config.match struct that provides the number of labels filled in.
func emptylabels(labels *match) int {
	count := 0
	if labels.Role != "" {
		count++
	}
	if labels.App != "" {
		count++
	}
	if labels.Env != "" {
		count++
	}
	if labels.Loc != "" {
		count++
	}
	return count
}

//updatedLabels - Function to update  workload with new labels
func updateLabels(w *illumioapi.Workload, lblhref map[string]illumioapi.Label) {

	var tmplbls []*illumioapi.Label
	for _, lbl := range w.Labels {
		tmplbl := lblhref[lbl.Href]
		tmplbls = append(tmplbls, &tmplbl)
	}
	w.Labels = tmplbls
}

//matchworkloads - Function that checks to see that labels are matching on what is configured in the Config data.  Returns true or false
func matchworkloads(labels []*illumioapi.Label, lblhref map[string]illumioapi.Label) bool {

	//Does the workload have 0 labels or if ignorematch is set.
	if (len(labels) < 1) || allWklds {
		return true
	}

	if len(labels) < 1 { //if AllEmpty not set but workload has 0 labels return false
		return false
	}

	if len(labels) != emptylabels(&conf.Match) { //if # labels of the workload <> labels configured in ConfigFile then return false
		return false
	}
	//Check to see if all the labels match by checking all the labels in ConfigFile agaist labels on the workload
	for _, tmplbl := range labels {
		l := lblhref[tmplbl.Href]
		switch l.Key {
		case "loc":
			if l.Value != locFlag {
				return false
			}
		case "env":
			if l.Value != envFlag {
				return false
			}
		case "app":
			if l.Value != appFlag {
				return false
			}
		case "role":
			if l.Value != roleFlag {
				return false
			}
		}
	}
	return true
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

//workloadHostname - Function to build a map of all workloads using the hostname as the key.
func workloadHostname(wklds []illumioapi.Workload) map[string]illumioapi.Workload {

	m := make(map[string]illumioapi.Workload)
	for _, w := range wklds {
		if w.Hostname != "" {
			//might need to validate Hostname is not '' so w.Name could be used.
			m[w.Hostname] = w
		}
	}
	return m

}

//labelhref - Return all the Label hrefs from the labels of a workload
func labelhref(labels []*illumioapi.Label) (string, string, string, string) {

	lochref, envhref, apphref, rolehref := "", "", "", ""
	for _, l := range labels {
		switch l.Key {
		case "loc":
			lochref = l.Href
		case "env":
			envhref = l.Href
		case "app":
			apphref = l.Href
		case "role":
			rolehref = l.Href
		}
	}
	return rolehref, apphref, envhref, lochref
}

//changecase - upperorlower function check to see if user set capitalization to ignore/no change(0 default), upper (1) or lower (2)
func changecase(str string) string {

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
	utils.Log(0, "started hostparse command")

	conf = config{Match: match{
		IgnoreMatch: allWklds,
		App:         appFlag,
		Env:         envFlag,
		Loc:         locFlag,
		Role:        roleFlag}}

	// Set output file
	outputFile = "workloader-hostparse-" + time.Now().Format("20060102_150405") + ".csv"

	if debug {
		utils.Log(0, fmt.Sprintf("updatepce set to %t ", updatePCE))
		utils.Log(0, fmt.Sprintf("ignorematch set to %t", allWklds))
		utils.Log(0, fmt.Sprintf("role,app,env,loc set to %s - %s - %s - %s", roleFlag, appFlag, envFlag, locFlag))
		utils.Log(0, fmt.Sprintf("capitalize set to %d", capitalize))
		utils.Log(0, fmt.Sprintf("hostfle set to %s", hostFile))
		utils.Log(0, fmt.Sprintf("parsefile set to %s", parserFile))
		utils.Log(0, fmt.Sprintf("noprompt set to %t", noPrompt))

	}

	//Read the Regex Parsing CSV.   Format should be match Regex and replace regex per label {}
	var parserec [][]string
	if parserFile != "" {
		parserec = ReadCSV(parserFile)
		if debug {
			utils.Log(2, fmt.Sprintf("hostparser - open parser file - %s", parserFile))
		}
	} else {
		fmt.Println("No Hostname parser file provide.  Please set the parser file location via --parserfile or -p ")
		os.Exit(0)
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
		utils.Log(0, fmt.Sprintf("Error accessing PCE API - Skipping further PCE API calls"))
		if debug {
			utils.Log(2, fmt.Sprintf("Get All Labels Error: %s", err))
		}
	}
	var workloads []illumioapi.Workload

	if !failedPCE {
		workloads, apiResp, err = pce.GetAllWorkloads()
		if debug {
			utils.Log(2, fmt.Sprintf("Get All Workloads API HTTP Request: %s %v", apiResp.Request.Method, apiResp.Request.URL))
			utils.Log(2, fmt.Sprintf("Get All Workloads API HTTP Reqest Header: %v", apiResp.Request.Header))
			utils.Log(2, fmt.Sprintf("Get All Workloads API Response Status Code: %d", apiResp.StatusCode))
			utils.Log(2, fmt.Sprintf("Get All Workloads API Response Body: \r\n %s", apiResp.RespBody))
		}
		if err != nil {
			utils.Log(2, fmt.Sprintf("Get All Workloads Error: %s", err))
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
		utils.Log(2, fmt.Sprintf("Build Map of HREFs with a key that uses a label's type and value eg. 'type.value': %v", lblskv))

	}

	//Create variables for wor
	var alllabeledwrkld []illumioapi.Workload
	nolabels := make(map[string]string)

	//Create output file
	var gatBulkupdateFile *os.File
	gatBulkupdateFile, err = os.Create(outputFile)
	if err != nil {
		utils.Logger.Fatalf("ERROR - Creating file - %s\n", err)
	}
	defer gatBulkupdateFile.Close()

	fmt.Fprintf(gatBulkupdateFile, "hostname,role,app,env,loc,href,prev-role,prev-app,prev-env,prev-loc,regex\r\n")

	var wkld []illumioapi.Workload
	if hostFile != "" {
		wkldsHN := workloadHostname(workloads)
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
		if matchworkloads(w.Labels, lblshref) {

			match, labeledwrkld := data.RelabelFromHostname(failedPCE, w, lblskv, nolabels, gatBulkupdateFile)
			orgRole, orgApp, orgEnv, orgLoc := labelvalues(w.Labels)
			role, app, env, loc := labelvalues(labeledwrkld.Labels)

			if match {
				if labeledwrkld.Href != "" && !(role == orgRole && app == orgApp && env == orgEnv && loc == orgLoc) {
					matchtable.Append([]string{labeledwrkld.Hostname, role, app, env, loc, orgRole, orgApp, orgEnv, orgLoc})
					alllabeledwrkld = append(alllabeledwrkld, labeledwrkld)
				} else if labeledwrkld.Href == "" && !updatePCE {
					matchtable.Append([]string{labeledwrkld.Hostname, role, app, env, loc, orgRole, orgApp, orgEnv, orgLoc})
					utils.Log(0, fmt.Sprintf("SKIPING UPDATE - %s - No Workload on the PCE", labeledwrkld.Hostname))
				} else {
					utils.Log(0, fmt.Sprintf("SKIPING UPDATE - %s - No Label Change Required", labeledwrkld.Hostname))

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
			fmt.Printf("Do you want to update Workloads and potentially create new labels(yes/no)? ")
			fmt.Scanln(&response)
		} else {
			fmt.Println("List of ALL Regex Matched Hostnames even if no Workloada exist on the PCE. ")
		}

		//If updating is selected and the NOPCE option has not been invoked then update labels and workloads.
		if response == "yes" && !failedPCE {

			if debug {
				utils.Log(2, fmt.Sprintf("*********************************LABEL CREATION***************************************"))
			}
			for _, lbl := range tmplbls {
				newLabel, apiResp, err := pce.CreateLabel(lbl)

				if err != nil {
					utils.Log(1, fmt.Sprint(err))

				}
				if debug {
					utils.Log(2, fmt.Sprintf("Exact label does not exist for %s (%s). Creating new label... ", lbl.Value, lbl.Value))
					utils.Log(2, fmt.Sprintf("Create Label API HTTP Request: %s %v", apiResp.Request.Method, apiResp.Request.URL))
					utils.Log(2, fmt.Sprintf("Create Label API HTTP Reqest Header: %+v", apiResp.Request.Header))
					utils.Log(2, fmt.Sprintf("Create Label API HTTP Reqest Body: %+v", illumioapi.Label{Key: lbl.Value, Value: lbl.Value}))
					utils.Log(2, fmt.Sprintf("Create Label API for %s (%s) Response Status Code: %d", lbl.Value, lbl.Value, apiResp.StatusCode))
					utils.Log(2, fmt.Sprintf("Create Label API for %s (%s) Response Body: %s", lbl.Value, lbl.Key, apiResp.RespBody))
				} else {
					utils.Log(0, fmt.Sprintf("CREATED LABEL %s (%s) with following HREF: %s", newLabel.Value, newLabel.Key, newLabel.Href))
				}
				lblskv[lbl.Key+"."+lbl.Value] = newLabel.Href
			}
			if debug {
				utils.Log(2, fmt.Sprintf("*********************************WORKLOAD BULK UPDATE***************************************"))
			}
			for _, w := range alllabeledwrkld {
				for _, l := range w.Labels {
					if l.Href == "" {
						l.Href = lblskv[l.Key+"."+l.Value]
					}
				}
			}
			// Send parsed workloads and new labels to BulkUpdate
			apiResp, err := pce.BulkWorkload(alllabeledwrkld, "update")

			//get number of workloads to update
			wkldcount := len(alllabeledwrkld) - 1
			c := 1
			cprime := 0
			for _, api := range apiResp {
				//Is the number of workloads less than 1000 then set the end number to # wkdls
				if int(c/wkldcount) < 999 {
					cprime = int(wkldcount / c)
					// when workloads are greater than 999 then make numbers function of 1000
				} else {
					cprime = 999
				}
				if err != nil {
					utils.Logger.Fatal(err)
				}
				if debug {
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API HTTP Request: %s %v", api.Request.Method, api.Request.URL))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API HTTP Reqest Header: %v", api.Request.Header))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API HTTP Body: %+v", alllabeledwrkld))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads Response Status Code: %d", api.StatusCode))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API Response Body: \r\n %s", api.RespBody))
				} else {
					utils.Log(0, fmt.Sprintf("BULKWORKLOAD UPDATE %d-%d workloads:", c, c+cprime))
				}
				c += 1000
			}

		}
	} else {
		//Make sure to put NO MATCHES into output file

		utils.Log(0, fmt.Sprintf("No Workloads will me updated  -  Check the output file"))

		if !noPrompt && !failedPCE {
			fmt.Println("***** There were no hostnames that needed updating or matched an entry in the 'parsefile'****")
		} else if failedPCE {
			fmt.Println("**** PCE Error **** Cannot update Labels or Hostnames to Upload **** Check Output file ****")
		}
	}
}
