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
)

//Data Structure for all the options of the hostparse function in workloader.
type config struct {
	Illumio illumio `toml:"illumio"`
	Parser  parser  `toml:"parser"`
	Match   match   `toml:"match"`
	Logging logging `toml:"logging"`
}

type illumio struct {
	NoPCE bool `toml:"no_pce"`
}

type parser struct {
	Parserfile   string `toml:"parserfile"`
	HostnameFile string `toml:"hostnamefile"`
	OutputFile   string `toml:"outputfile"`
	NoPrompt     bool   `toml:"noprompt"`
	CheckCase    int    `toml:"checkcase"`
	Name         bool   `toml:"name"`
}
type match struct {
	IgnoreMatch bool   `toml:"ignorematch"`
	App         string `toml:"app"`
	Env         string `toml:"env"`
	Loc         string `toml:"loc"`
	Role        string `toml:"role"`
}
type logging struct {
	LogOnly bool `toml:"log_only"`
	debug   bool
}

//pasrseConfig - Function to load all the options into a global variable.
//
//  NoPCE - If set to true then never call PCE APIs to get or update date
//  ParseFile - File that has the Regex to match with as well as Results of parsing for each label
//  HostFile - If not using PCE workloads you can load a file with hostnames
//  OutputFile - Name of the file that will be built  - Currently static - no way to set
//  NoPrompt - If no user interaction (confirmation questions) is required this will perform all tasks as YES
//  CheckCase - Will make resulting Labels Upper=0, Lower=1 or use the match results output
//  Name - Parsing will be done on either the Hostname or Name field.  Default to Hostname
//  IgnoreMatch - Normally we need workloads to have no labels or specific labels.  This will ignore that and parse all PCE workloads
//  Role,App,Env,Loc - Labels that will be used to match workloads to have their names parsed...if left blank then looking for workloads without any labels.
//  LogOnly - Will not push any changes back to the PCE.  It will pull data from the PCE.  NoPce will stop even that.
//  Debug - This will enable debug Logging.
func parseConfig() config {

	config := config{
		Illumio: illumio{
			NoPCE: noPCE},
		Parser: parser{
			Parserfile:   parserFile,
			HostnameFile: hostFile,
			OutputFile:   "hostname-parser-output-" + time.Now().Format("20060102_150405") + ".csv",
			NoPrompt:     noPrompt,
			CheckCase:    updatecase,
			Name:         name},
		Match: match{
			IgnoreMatch: ignoreMatch,
			App:         appFlag,
			Env:         envFlag,
			Loc:         locFlag,
			Role:        roleFlag},
		Logging: logging{
			LogOnly: logonly,
			debug:   debugLogging}}

	return config
}

// Set up global variables
var configFile, parserFile, hostFile, outputFile, appFlag, roleFlag, envFlag, locFlag string
var debugLogging, noPrompt, logonly, ignoreMatch, noPCE, verbose, name bool
var updatecase int
var pce illumioapi.PCE
var err error
var conf config

// Init function will handle flags
func init() {
	HostnameCmd.Flags().StringVarP(&parserFile, "parserfile", "p", "", "Location of CSV with regex functions and labels.")
	HostnameCmd.Flags().StringVar(&hostFile, "hostfile", "", "Location of hostnames CSV to parse.")
	HostnameCmd.Flags().StringVarP(&roleFlag, "role", "e", "", "Environment label.")
	HostnameCmd.Flags().StringVarP(&appFlag, "app", "a", "", "App label.")
	HostnameCmd.Flags().StringVarP(&envFlag, "env", "r", "", "Role label.")
	HostnameCmd.Flags().StringVarP(&locFlag, "loc", "l", "", "Location label.")
	HostnameCmd.Flags().BoolVar(&noPrompt, "noprompt", false, "No Prompt or output.  Used for automatation in a script.")
	//HostnameCmd.Flags().BoolVar(&allEmpty, "allempty", false, "Parse all PCE workloads that have zero labels assigned.")
	HostnameCmd.Flags().BoolVar(&ignoreMatch, "ignorematch", false, "Parse all PCE workloads no matter what labels are assigned.")
	HostnameCmd.Flags().BoolVar(&noPCE, "nopce", false, "No PCE.")
	HostnameCmd.Flags().BoolVar(&debugLogging, "debug", false, "Debug logging.")
	HostnameCmd.Flags().BoolVar(&logonly, "logonly", false, "Set to only log changes. Don't update the PCE.")
	HostnameCmd.Flags().BoolVar(&name, "name", false, "Search Name field of workload instead of Hostname. Defaults to Hostname.")
	HostnameCmd.Flags().IntVar(&updatecase, "updatecase", 1, "Set 1 for uppercase labels(default), 2 for lowercase labels or 0 to leave capitalization alone.")
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
			utils.Logger.Fatalf("[ERROR] - getting PCE for hostparser - %s", err)
		}

		hostnameParser()
	},
}

//data structure built from the parser.csv
type regex struct {
	regexdata []regexstruct
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
func (r *regex) RelabelFromHostname(wkld illumioapi.Workload, lbls map[string]string, nolabels map[string]string) (bool, illumioapi.Workload, string) {

	//var templabels []string
	var match bool

	var tmpwkld illumioapi.Workload

	found := false

	var searchname string
	if conf.Parser.Name {
		searchname = wkld.Name
	} else {
		searchname = wkld.Hostname
	}

	if searchname == "" {
		found = true
		if conf.Parser.Name {
			utils.Log(2, fmt.Sprintf("No Name string configured on the workload.  Hostname - %s", wkld.Hostname))
		} else {
			utils.Log(2, fmt.Sprintf("No Hostname string configured on the workload. Name - %s", wkld.Name))
		}
	} else {
		utils.Log(0, fmt.Sprintf("REGEX Match For - %s", searchname))
	}

	for _, tmp := range r.regexdata {

		//If you found a match skip to the next hostname
		if !found {
			//Place match regex into regexp data struct
			tmpre := regexp.MustCompile(tmp.regex)

			//Is there a match using the regex?
			match = tmpre.MatchString(searchname)

			//Report  if we have a match, regex and replacement regex per label
			if conf.Logging.debug && !match {
				utils.Log(2, fmt.Sprintf("%s - Regex: %s - Match: %t", searchname, tmp.regex, match))
			}

			if match {
				//stop searching regex for a match if one is already found
				found = true
				utils.Log(0, fmt.Sprintf("%s - Regex: %s - Match: %t", searchname, tmp.regex, match))
				// Copy the workload struct to save to new updated workload struct if needed.
				tmpwkld = wkld

				// Save the labels that are existing
				orgLabels := make(map[string]string)
				for _, l := range wkld.Labels {
					orgLabels[l.Key] = l.Href
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
							tmplabel = illumioapi.Label{Href: lbls[label+tmpstr], Key: label, Value: tmpstr}
						} else {

							//create an entry for the label type and value into the Href map...Href is empty to start
							lbls[label+"."+tmpstr] = ""

							//create a list of labels that arent currently configured on the PCE that the replacement regex  wants.
							nolabels[label+"."+tmpstr] = ""

							//Build a label variable with Label type and Value but no Href due to the face its not configured on the PCE
							tmplabel = illumioapi.Label{Key: label, Value: tmpstr}

						}
						tmplabels = append(tmplabels, &tmplabel)

						// If the regex doesnt produce a replacement or there isnt a replace regex in the CSV then copy orginial label
					} else {
						//fmt.Println(orgLabels[label])
						if orgLabels[label] != "" {
							tmplabel = illumioapi.Label{Key: label, Href: orgLabels[label]}
							tmplabels = append(tmplabels, &tmplabel)
						}

					}

					//Add Label array to the workload.
					tmpwkld.Labels = tmplabels
				}

				//Get the original labels and new labels to show the changes.
				orgRole, orgApp, orgEnv, orgLoc := labelvalues(wkld.Labels)
				role, app, env, loc := labelvalues(tmpwkld.Labels)
				utils.Log(0, fmt.Sprintf("%s - Replacement Regex: %+v - Labels: %s - %s - %s - %s", searchname, tmp.labelcg, role, app, env, loc))
				utils.Log(0, fmt.Sprintf("%s - Current Labels: %s, %s, %s, %s Replaced with: %s, %s, %s, %s", searchname, orgRole, orgApp, orgEnv, orgLoc, role, app, env, loc))

			}
		} else {
			return match, tmpwkld, searchname
		}
	}
	utils.Log(0, fmt.Sprintf("**** NO REGEX MATCH FOUND **** - %s -", searchname))
	//return there was no match for that hostname
	return match, tmpwkld, searchname
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

			r.regexdata = append(r.regexdata, tmpr)
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

//Function that checks to see that labels are matching on what is configured in the Config data.  Returns true or false
func matchworkloads(labels []*illumioapi.Label, lblhref map[string]illumioapi.Label) bool {

	//Does the workload have 0 labels or if ignorematch is set.
	if (len(labels) < 1) || conf.Match.IgnoreMatch {
		return true

	} else if len(labels) < 1 { //if AllEmpty not set but workload has 0 labels return false
		return false
	} else if len(labels) != emptylabels(&conf.Match) { //if # labels of the workload <> labels configured in ConfigFile then return false
		return false
		//Check to see if all the labels match by checking all the labels in ConfigFile agaist labels on the workload
	} else {
		for _, tmplbl := range labels {
			l := lblhref[tmplbl.Href]
			switch l.Key {
			case "loc":
				if l.Value != conf.Match.Loc {
					return false
				}
			case "env":
				if l.Value != conf.Match.Env {
					return false
				}
			case "app":
				if l.Value != conf.Match.App {
					return false
				}
			case "role":
				if l.Value != conf.Match.Role {
					return false
				}
			}
		}
		return true
	}
	//return if match = 0 or 1 as boolean

}

//Return all the Label values from the labels of a workload
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

//Return all the Label hrefs from the labels of a workload
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

//upperorlower function check to see if user set capitalization to ignore/no change(0 default), upper (1) or lower (2)
func changecase(str string) string {

	switch conf.Parser.CheckCase {
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

func hostnameParser() {

	//Load All the options into the conf variable.
	conf = parseConfig()

	if conf.Illumio.NoPCE && conf.Parser.HostnameFile == "" {
		fmt.Println("You must use the hostfile option when using the 'no_pce' option")
		utils.Logger.Fatalf("[ERROR] - hostparser - no-pce=%t. and hostfile=%s.  Must use the hostfile option when no_pce=true", conf.Illumio.NoPCE, conf.Parser.HostnameFile)
	}

	//Set timestamp for file usage.
	timestamp := time.Now().Format("20060102_150405")

	// LOG THE MODE
	if conf.Logging.debug {
		utils.Log(0, fmt.Sprintf("***************************************************************************************************************************"))
		utils.Log(0, fmt.Sprintf("                                                      HOSTPARSE"))
		utils.Log(0, fmt.Sprintf("***************************************************************************************************************************"))
		utils.Log(0, fmt.Sprintf("hostparser - 'nopce' set to %t", conf.Illumio.NoPCE))
		utils.Log(0, fmt.Sprintf("hostparser - 'logonly' set to %t ", conf.Logging.LogOnly))
		utils.Log(0, fmt.Sprintf("hostparser - 'ignorematch' set to %t", conf.Match.IgnoreMatch))
		utils.Log(0, fmt.Sprintf("hostparser - 'role,app,env,loc' set to %s - %s - %s - %s", conf.Match.Role, conf.Match.App, conf.Match.Env, conf.Match.Loc))
		utils.Log(0, fmt.Sprintf("hostparser - 'updatecase' set to %d", conf.Parser.CheckCase))
		utils.Log(0, fmt.Sprintf("hostparser - 'hostfle' set to %s", conf.Parser.HostnameFile))
		utils.Log(0, fmt.Sprintf("hostparser - 'parsefile' set to %s", conf.Parser.Parserfile))
		utils.Log(0, fmt.Sprintf("hostparser - 'noprompt' set to %t", conf.Parser.NoPrompt))
		utils.Log(0, fmt.Sprintf("hostparser - 'name' set to %t", conf.Parser.Name))

	}

	//Read the Regex Parsing CSV.   Format should be match Regex and replace regex per label {}
	var parserec [][]string
	if conf.Parser.Parserfile != "" {
		parserec = ReadCSV(conf.Parser.Parserfile)
		if conf.Logging.debug {
			utils.Log(2, fmt.Sprintf("hostparser - open parser file - %s", conf.Parser.Parserfile))
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
	matchtable.SetHeader([]string{"Hostname", "Role", "App", "Env", "Loc"})

	//Make the Label Output table object for the console
	labeltable := tablewriter.NewWriter(os.Stdout)
	labeltable.SetAlignment(tablewriter.ALIGN_LEFT)
	labeltable.SetHeader([]string{"Type", "Value"})

	//Map struct for labels using 'key'.'value' as the map key.
	lblskv := make(map[string]string)
	//Map struct for labels using labe 'href' as the map key.
	lblshref := make(map[string]illumioapi.Label)

	//Access PCE to get all Labels only if no_pce is not set to true in config file
	if !conf.Illumio.NoPCE {
		labels, apiResp, err := illumioapi.GetAllLabels(pce)
		//fmt.Println(labelsAPI, apiResp, err)
		if err != nil {
			utils.Logger.Fatal(err)
		}
		if conf.Logging.debug == true {
			utils.Log(2, fmt.Sprintf("Get All Labels API HTTP Request: %s %v", apiResp.Request.Method, apiResp.Request.URL))
			utils.Log(2, fmt.Sprintf("Get All Labels API HTTP Reqest Header: %v", apiResp.Request.Header))
			utils.Log(2, fmt.Sprintf("Get All Labels API Response Status Code: %d", apiResp.StatusCode))
			utils.Log(0, fmt.Sprintf("Get All Labels API Response Body: \r\n %s", apiResp.RespBody))
		}
		//create Label array with all the HRefs as value with label type and label key combined as the key "key.value"
		for _, l := range labels {
			lblskv[l.Key+"."+l.Value] = l.Href
			lblshref[l.Href] = l

		}
		if conf.Logging.debug == true {
			utils.Log(2, fmt.Sprintf("Build Map of HREFs with a key that uses a label's type and value eg. 'type.value': %v", lblskv))

		}

	}

	//Create variables for wor
	var alllabeledwrkld []illumioapi.Workload
	nolabels := make(map[string]string)
	var gatBulkupdateFile *os.File
	if conf.Parser.OutputFile != "" {
		gatBulkupdateFile, err = os.Create(conf.Parser.OutputFile)
	} else {
		gatBulkupdateFile, err = os.Create("gat-bulk-umwls_" + timestamp + ".csv")
	}
	if err != nil {
		utils.Logger.Fatalf("ERROR - Creating file - %s\n", err)
	}
	defer gatBulkupdateFile.Close()

	if conf.Parser.HostnameFile != "" {
		hostrec := ReadCSV(conf.Parser.HostnameFile)
		if conf.Logging.debug {
			utils.Log(2, fmt.Sprintf("Skipping calls to PCE for workloads hostname and using CSV hostname file"))
		}

		fmt.Fprintf(gatBulkupdateFile, "hostname,role.app,env,loc,href,other\r\n")
		for _, row := range hostrec {
			//get all the extra data from the hostfile and place it at the end of the output file.
			var tmpstr string
			l := len(row)
			for c := 1; c < l; c++ {
				if c != 1 {
					tmpstr = ","
				}
				tmpstr = tmpstr + fmt.Sprintf("%s", row[c])
			}

			//Parse hostname and update nolabel map with labels that arent on the pce.
			match, labeledwrkld, searchname := data.RelabelFromHostname(illumioapi.Workload{Hostname: row[0], Name: row[0]}, lblskv, nolabels)
			var role, app, env, loc string
			if match {
				role, app, env, loc = labelvalues(labeledwrkld.Labels)
				matchtable.Append([]string{searchname, role, app, env, loc})
			}
			fmt.Fprintf(gatBulkupdateFile, "%s,%s,%s,%s,%s,%s,%s\r\n", searchname, role, app, env, loc, labeledwrkld.Href, tmpstr)
			alllabeledwrkld = append(alllabeledwrkld, labeledwrkld)
		}

	} else {
		//Access PCE to get all Workloads.  Check rally should never get to this if no hostfile is configured...just extra error checking
		if !conf.Illumio.NoPCE {

			workloads, apiResp, err := illumioapi.GetAllWorkloads(pce)
			if conf.Logging.debug {
				utils.Log(2, fmt.Sprintf("Get All Workloads API HTTP Request: %s %v", apiResp.Request.Method, apiResp.Request.URL))
				utils.Log(2, fmt.Sprintf("Get All Workloads API HTTP Reqest Header: %v", apiResp.Request.Header))
				utils.Log(2, fmt.Sprintf("Get All Workloads API Response Status Code: %d", apiResp.StatusCode))
				utils.Log(2, fmt.Sprintf("Get All Workloads API Response Body: \r\n %s", apiResp.RespBody))
			}
			if err != nil {
				utils.Log(1, fmt.Sprint(err))

			}

			fmt.Fprintf(gatBulkupdateFile, "hostname,role.app,env,loc,href\r\n")

			//Cycle through all the workloads
			for _, w := range workloads {

				//Check to see
				if matchworkloads(w.Labels, lblshref) {
					match, labeledwrkld, searchname := data.RelabelFromHostname(w, lblskv, nolabels)
					if match {
						role, app, env, loc := labelvalues(labeledwrkld.Labels)
						fmt.Fprintf(gatBulkupdateFile, "%s,%s,%s,%s,%s,%s\r\n", searchname, role, app, env, loc, labeledwrkld.Href)
						matchtable.Append([]string{searchname, role, app, env, loc})
						alllabeledwrkld = append(alllabeledwrkld, labeledwrkld)

					}
				}

			}
		}

	}

	//Capture all the labels that need to be created and make them ready for display.
	var tmplbls []illumioapi.Label
	if len(nolabels) > 0 {

		for keylabel := range nolabels {
			key, value := strings.Split(keylabel, ".")[0], strings.Split(keylabel, ".")[1]
			tmplbls = append(tmplbls, illumioapi.Label{Value: value, Key: key})
			labeltable.Append([]string{key, value})
			//Make sure we arent only looking for an output file and we have the ability to access the PCE.

		}
		if !conf.Parser.NoPrompt {
			labeltable.Render()
			fmt.Print("***** Above Labels Not Configured on the PCE ***** \r\n")

		}
	}

	var response string
	// Print table with all the workloads and the new labels.
	if len(alllabeledwrkld) > 0 {
		if !conf.Parser.NoPrompt {
			matchtable.Render()
		}

		//check if noprompt is set to true or logging changes only....Skip bulk upload of workload labels.
		if !conf.Parser.NoPrompt && !conf.Logging.LogOnly {
			fmt.Printf("Do you want to update Workloads and potentially create new labels(yes/no)? ")
			fmt.Scanln(&response)
		} else {
			response = "yes"
		}

		if response == "yes" && (!conf.Logging.LogOnly && !conf.Illumio.NoPCE) {

			if conf.Logging.debug {
				utils.Log(2, fmt.Sprintf("*********************************LABEL CREATION***************************************"))
				utils.Log(2, fmt.Sprintf("Both LogOnly is set to false and NoPCE is set to false - Creating Labels"))
			}
			for _, lbl := range tmplbls {
				newLabel, apiResp, err := illumioapi.CreateLabel(pce, lbl)

				if err != nil {
					utils.Log(1, fmt.Sprint(err))
					//utils.Logger.Printf("ERROR - %s", err)
				}
				if conf.Logging.debug {
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
			if conf.Logging.debug {
				utils.Log(2, fmt.Sprintf("*********************************WORKLOAD BULK UPDATE***************************************"))
				utils.Log(2, fmt.Sprintf("Both LogOnly is set to false and NoPCE is set to false - Updating Workload Labels"))
			}
			for _, w := range alllabeledwrkld {
				for _, l := range w.Labels {
					if l.Href == "" {
						l.Href = lblskv[l.Key+"."+l.Value]
					}
				}
			}
			apiResp, err := illumioapi.BulkWorkload(pce, alllabeledwrkld, "update")

			c := 1
			for _, api := range apiResp {
				if err != nil {
					utils.Logger.Fatal(err)
				}
				if conf.Logging.debug {
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API HTTP Request: %s %v", api.Request.Method, api.Request.URL))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API HTTP Reqest Header: %v", api.Request.Header))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API HTTP Body: %+v", alllabeledwrkld))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads Response Status Code: %d", api.StatusCode))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API Response Body: \r\n %s", api.RespBody))
				} else {
					utils.Log(0, fmt.Sprintf("BULKWORKLOAD UPDATE %d-%d workloads:", c, c+999))
				}
				c += 1000
			}

		}
	} else {
		//Make sure to put NO MATCHES into output file
		utils.Log(2, fmt.Sprintf("NO WORKLOAD WERE EITHER FOUND OR MATCHED REGEX"))

		if !conf.Parser.NoPrompt {
			fmt.Println("***** There were no hostnames that match in the 'parsefile'****")
		}
	}
}
