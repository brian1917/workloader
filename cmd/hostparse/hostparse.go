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

// Set up global variables
var configFile, parserFile, hostFile, outputFile, appFlag, roleFlag, envFlag, locFlag string
var debugLogging, noPrompt, logonly, allEmpty, ignoreMatch, noPCE, verbose, name bool
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
	HostnameCmd.Flags().BoolVar(&allEmpty, "allempty", false, "Parse all PCE workloads that have zero labels assigned.")
	HostnameCmd.Flags().BoolVar(&ignoreMatch, "ignorematch", false, "Parse all PCE workloads no matter what labels are assigned.")
	HostnameCmd.Flags().BoolVar(&noPCE, "nopce", false, "No PCE.")
	HostnameCmd.Flags().BoolVar(&debugLogging, "debug", false, "Debug logging.")
	HostnameCmd.Flags().BoolVar(&logonly, "logonly", false, "Set to only log changes. Don't update the PCE.")
	HostnameCmd.Flags().BoolVar(&name, "name", false, "Search Name field of workload. Defaults to Hostname.")
	HostnameCmd.Flags().IntVar(&updatecase, "updatecase", 1, "Set 1 for uppercase labels(default), 2 for lowercase labels or 0 to ignore.")
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
| (h)(1)-(\w*)-([s])(\d+) | WEB  | ${3}  | SITE${5} | AMAZON | h1-app-s1     		|
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

// createLabels
func createLabels(pce illumioapi.PCE, tmplabel illumioapi.Label) illumioapi.Label {

	newLabel, apiResp, err := illumioapi.CreateLabel(pce, tmplabel)

	if conf.Logging.debug {
		utils.Log(2, fmt.Sprintf("exact label does not exist for %s (%s). Creating new label... \r\n", tmplabel.Value, tmplabel.Key))
		utils.Log(2, fmt.Sprintf("create Label API HTTP Request: %s %v \r\n", apiResp.Request.Method, apiResp.Request.URL))
		utils.Log(2, fmt.Sprintf("create Label API HTTP Reqest Header: %+v \r\n", apiResp.Request.Header))
		utils.Log(2, fmt.Sprintf("create Label API HTTP Reqest Body: %+v \r\n", tmplabel))
		utils.Log(2, fmt.Sprintf("create Label API for %s (%s) Response Status Code: %d \r\n", tmplabel.Value, tmplabel.Key, apiResp.StatusCode))
		utils.Log(2, fmt.Sprintf("create Label API for %s (%s) Response Body: %s \r\n", tmplabel.Value, tmplabel.Key, apiResp.RespBody))
	}
	if err != nil {
		utils.Log(1, err.Error())
	}

	utils.Log(0, fmt.Sprintf("created label %s (%s) - %s", newLabel.Value, newLabel.Key, newLabel.Href))

	return newLabel
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
			utils.Log(2, fmt.Sprintf("No Name string configured on the workload.  Hostname - %s\r\n", wkld.Hostname))
		} else {
			utils.Log(2, fmt.Sprintf("No Hostname string configured on the workload. Name - %s\r\n", wkld.Name))
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

	//Does the workload have 0 labels and if AllEmpty is set in Config file or if ignorematch is set.
	if ((len(labels) < 1) && conf.Match.AllEmpty) || conf.Match.IgnoreMatch {
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

	conf = parseConfig()

	//Set timestamp for file usage.
	timestamp := time.Now().Format("20060102_150405")

	// LOG THE MODE
	utils.Log(0, fmt.Sprintf("hostparser - log only mode set to %t \r\n", conf.Logging.LogOnly))
	utils.Log(0, fmt.Sprintf("hostparser - Illumio and Log Settings NoPCE:%+v %+v\r\n", conf.Illumio.NoPCE, conf.Logging))

	//Read the Regex Parsing CSV.   Format should be match Regex and replace regex per label {}
	var parserec [][]string
	if conf.Parser.Parserfile != "" {
		parserec = ReadCSV(conf.Parser.Parserfile)
		if conf.Logging.debug == true {
			utils.Log(2, fmt.Sprintf("hostparser - open parser file - %s\r\n", conf.Parser.Parserfile))
		}
	} else {
		fmt.Println("No Hostname parser file provide.  Please set the parser file location via --parserfile or -p ")
		os.Exit(0)
	}

	var data regex
	data.load(parserec)

	matchtable := tablewriter.NewWriter(os.Stdout)
	matchtable.SetAlignment(tablewriter.ALIGN_LEFT)
	matchtable.SetHeader([]string{"Hostname", "Role", "App", "Env", "Loc"})

	labeltable := tablewriter.NewWriter(os.Stdout)
	labeltable.SetAlignment(tablewriter.ALIGN_LEFT)
	labeltable.SetHeader([]string{"Type", "Value"})

	lblskv := make(map[string]string)
	lblshref := make(map[string]illumioapi.Label)
	//Access PCE to get all Labels only if no_pce is not set to true in config file
	if !conf.Illumio.NoPCE {
		labels, apiResp, err := illumioapi.GetAllLabels(pce)
		//fmt.Println(labelsAPI, apiResp, err)
		if err != nil {
			utils.Logger.Fatal(err)
		}
		if conf.Logging.debug == true {
			utils.Log(2, fmt.Sprintf("Get All Labels API HTTP Request: %s %v \r\n", apiResp.Request.Method, apiResp.Request.URL))
			utils.Log(2, fmt.Sprintf("Get All Labels API HTTP Reqest Header: %v \r\n", apiResp.Request.Header))
			utils.Log(2, fmt.Sprintf("Get All Labels API Response Status Code: %d \r\n", apiResp.StatusCode))
			utils.Log(0, fmt.Sprintf("Get All Labels API Response Body: \r\n %s \r\n", apiResp.RespBody))
		}
		//create Label array with all the HRefs as value with label type and label key combined as the key "key.value"
		for _, l := range labels {
			lblskv[l.Key+"."+l.Value] = l.Href
			lblshref[l.Href] = l

		}
		if conf.Logging.debug == true {
			utils.Log(2, fmt.Sprintf("Build Map of HREFs with a key that uses a label's type and value eg. 'type.value': %v \r\n", lblskv))

		}

	}
	// var createlabels string
	// fmt.Print("Do you want to create labels that are not already configured on the PCE?")
	// fmt.Scanln(&createlabels)

	//fmt.Println(lbls)
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
		if conf.Logging.debug == true {
			utils.Log(2, fmt.Sprintf("Skipping calls to PCE for workloads hostname and using CSV hostname file \r\n"))
		}

		for _, x := range hostrec {

			match, labeledwrkld, searchname := data.RelabelFromHostname(illumioapi.Workload{Hostname: x[0]}, lblskv, nolabels)
			if match {
				role, app, env, loc := labelvalues(labeledwrkld.Labels)
				fmt.Fprintf(gatBulkupdateFile, "%s,%s,%s,%s,%s,%s\r\n", searchname, role, app, env, loc, labeledwrkld.Href)
				matchtable.Append([]string{searchname, role, app, env, loc})
			}
			alllabeledwrkld = append(alllabeledwrkld, labeledwrkld)
		}

	} else {
		//Access PCE to get all Workloads.  Check rally should never get to this if no hostfile is configured...just extra error checking
		if !conf.Illumio.NoPCE {

			workloads, apiResp, err := illumioapi.GetAllWorkloads(pce)
			if conf.Logging.debug {
				utils.Log(2, fmt.Sprintf("Get All Workloads API HTTP Request: %s %v \r\n", apiResp.Request.Method, apiResp.Request.URL))
				utils.Log(2, fmt.Sprintf("Get All Workloads API HTTP Reqest Header: %v \r\n", apiResp.Request.Header))
				utils.Log(2, fmt.Sprintf("Get All Workloads API Response Status Code: %d \r\n", apiResp.StatusCode))
				utils.Log(2, fmt.Sprintf("Get All Workloads API Response Body: \r\n %s \r\n", apiResp.RespBody))
			}
			if err != nil {
				utils.Log(1, fmt.Sprint(err))

			}

			//fmt.Printf("%+v\r\n", len(workloads))
			fmt.Fprintf(gatBulkupdateFile, "hostname,role.app,env,loc,href\r\n")
			for _, w := range workloads {

				//fmt.Println(w.Hostname)
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
		// for _, w := range alllabeledwrkld {
		// 	for k, l := range w.Labels {
		// 		fmt.Printf("%+v %+v %d\r\n", l, l.Value, k)
		// 	}
		// }

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
			// if conf.Logging.verbose {
			// 	fmt.Printf("**** Parsing the hostname provided these updated labels.\r\n ")
			// }
			fmt.Printf("Do you want to update Workloads and potentially create new labels(yes/no)? ")
			fmt.Scanln(&response)
		} else {
			response = "yes"
		}

		if response == "yes" && (!conf.Logging.LogOnly && !conf.Illumio.NoPCE) {

			if conf.Logging.debug {
				utils.Log(2, fmt.Sprintf("*********************************LABEL CREATION***************************************\r\n"))
				utils.Log(2, fmt.Sprintf("Both LogOnly is set to false and NoPCE is set to false - Creating Labels\r\n"))
			}
			for _, lbl := range tmplbls {
				newLabel, apiResp, err := illumioapi.CreateLabel(pce, lbl)

				if err != nil {
					utils.Log(1, fmt.Sprint(err))
					//utils.Logger.Printf("ERROR - %s", err)
				}
				if conf.Logging.debug {
					utils.Log(2, fmt.Sprintf("Exact label does not exist for %s (%s). Creating new label... \r\n", lbl.Value, lbl.Value))
					utils.Log(2, fmt.Sprintf("Create Label API HTTP Request: %s %v \r\n", apiResp.Request.Method, apiResp.Request.URL))
					utils.Log(2, fmt.Sprintf("Create Label API HTTP Reqest Header: %+v \r\n", apiResp.Request.Header))
					utils.Log(2, fmt.Sprintf("Create Label API HTTP Reqest Body: %+v \r\n", illumioapi.Label{Key: lbl.Value, Value: lbl.Value}))
					utils.Log(2, fmt.Sprintf("Create Label API for %s (%s) Response Status Code: %d \r\n", lbl.Value, lbl.Value, apiResp.StatusCode))
					utils.Log(2, fmt.Sprintf("Create Label API for %s (%s) Response Body: %s \r\n", lbl.Value, lbl.Key, apiResp.RespBody))
				}

				utils.Log(0, fmt.Sprintf("CREATED LABEL %s (%s) with following HREF: %s", newLabel.Value, newLabel.Key, newLabel.Href))

				lblskv[lbl.Key+"."+lbl.Value] = newLabel.Href
			}
			if conf.Logging.debug {
				utils.Log(2, fmt.Sprintf("*********************************WORKLOAD BULK UPDATE***************************************\r\n"))
				utils.Log(2, fmt.Sprintf("Both LogOnly is set to false and NoPCE is set to false - Updating Workload Labels\r\n"))
			}
			for _, w := range alllabeledwrkld {
				for _, l := range w.Labels {
					if l.Href == "" {
						l.Href = lblskv[l.Key+"."+l.Value]
					}
				}
			}
			apiResp, err := illumioapi.BulkWorkload(pce, alllabeledwrkld, "update")

			for _, api := range apiResp {
				if err != nil {
					utils.Logger.Fatal(err)
				}
				if conf.Logging.debug {
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API HTTP Request: %s %v \r\n", api.Request.Method, api.Request.URL))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API HTTP Reqest Header: %v \r\n", api.Request.Header))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API HTTP Body: %+v \r\n", alllabeledwrkld))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads Response Status Code: %d \r\n", api.StatusCode))
					utils.Log(2, fmt.Sprintf("BulkUpdate Workloads API Response Body: \r\n %s \r\n", api.RespBody))
				}
			}

		}
	} else {
		if conf.Logging.debug {
			utils.Log(2, fmt.Sprintf("NO WORKLOAD WERE EITHER FOUND OR MATCHED REGEX\r\n"))

		}
		fmt.Println("***** There were no hostnames that match in the 'parsefile'****")
	}
}
