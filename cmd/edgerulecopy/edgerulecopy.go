package edgerulecopy

import (
	"fmt"
	"os"
	"time"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var debug, updatePCE, noPrompt, doNotProvision bool
var csvFile, fromGroup, toGroup string
var pce illumioapi.PCE
var err error

// RuleMap - Variable to store Rules from TO Group.
type RuleMap struct {
	Href      string    `json:"href,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

func init() {
	EdgeRuleCopyCmd.Flags().StringVarP(&fromGroup, "from-group", "f", "", "Name of Endpoint group to copy rules from. Required")
	EdgeRuleCopyCmd.MarkFlagRequired("from-group")
	EdgeRuleCopyCmd.Flags().StringVarP(&toGroup, "to-group", "t", "", "Name of Endpoint group to create rules from rules copied from other Endpoint group")
	EdgeRuleCopyCmd.MarkFlagRequired("to-group")
	EdgeRuleCopyCmd.Flags().BoolVarP(&doNotProvision, "do-not-prov", "x", false, "Do not provision created Endpoint group rules. Will provision by default.")
}

// EdgeRuleCopyCmd runs the rules copy command between 2 Illumio Edge groups
var EdgeRuleCopyCmd = &cobra.Command{
	Use:   "edgerulecopy",
	Short: "Copy rules from one group to another.",
	Long: `
Copy all the rules for one group to another.  To select the rules from one endpoint group specify the group name by placing the name after --from-ruleset or -f option.
 The group name that will receive these rules should be entered after the --to-group or -t option.  Both groups must exist on the PCE.  Exact match is required.  
 If Endpoint names have spaces in them, the entire group names needs to be encapsulated with \" (e.g. \"New Group\")

 *NOTE - All rules will be copied only.  Currently, you cannot update rules across groups.  Running this more than once will copy the rules each time.
`,

	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err = utils.GetDefaultPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		// if len(args) !=  {
		// 	fmt.Println("Command requires 1 argument for the csv file. See usage help.")
		// 	os.Exit(0)
		// }
		// csvFile = args[0]

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)
		updatePCE = viper.Get("update_pce").(bool)
		noPrompt = viper.Get("no_prompt").(bool)

		// Disable stdout
		viper.Set("output_format", "csv")
		if err := viper.WriteConfig(); err != nil {
			utils.LogError(err.Error())
		}

		edgerulescopy()
	},
}

//
func edgerulescopy() {

	// Log start of run
	utils.LogStartCommand("edgerulescopy")

	// Get all rulesets from the source PCE
	rulesets, a, err := pce.GetAllRuleSets("draft")
	utils.LogAPIResp("GetAllRuleSets", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	// fromlabel, a, err := pce.GetLabelbyKeyValue("role", fromRuleset)
	// utils.LogAPIResp("GetLabelbyKeyValue", a)
	// if err != nil {
	// 	utils.LogError(err.Error())
	// }

	//Get HRef to replace every copied rule provider with.  Necessary for Edge.
	toLabel, a, err := pce.GetLabelbyKeyValue("role", toGroup)
	utils.LogAPIResp("GetLabelbyKeyValue", a)
	if err != nil {
		utils.LogError(err.Error())
		utils.LogEndCommand("edgerulescopy")
	}

	//Use Clean Label Struct so only HREF is used.
	newLabel := illumioapi.Label{Href: toLabel.Href}

	//Create Provider Array to change Provider in Rules below - Edge requirement
	toProvider := []*illumioapi.Providers{&illumioapi.Providers{Label: &newLabel}}

	//Href of TO Ruleset
	var savedToRuleSetHref string = ""
	var foundfrom, foundto bool = false, false

	//Array of Rules copied on Match of FROM Ruleset
	var toRules, fromRules []*illumioapi.Rule

	// Iterate the list of Rulesets to match Ruleset you will copy rules from and Ruleset you will copy to.
	for _, ruleset := range rulesets {

		//Break out of FOR if both TO and FROM Rulesets found
		if !(foundfrom && foundto) {
			switch ruleset.Name {
			case fromGroup:
				fromRules = ruleset.Rules
				foundfrom = true
			case toGroup:
				toRules = ruleset.Rules
				savedToRuleSetHref = ruleset.Href
				foundto = true
			}
		} else {
			break
		}

	}

	//Load all the rules if any from the copy TO group that have have ExternalDataReference data.  When creating a rule put a referece to the rule that was used to copy.
	// By having the refence data we can then compare the existing rules to new ones and skip over existing ones.

	rulemap := make(map[string]RuleMap)
	for _, rule := range toRules {
		var tmprulemap RuleMap
		//convert CreatedAt and UpdatedAt strings to time variables
		ct, err := time.Parse(time.RFC3339, rule.CreatedAt)
		if err != nil {
			fmt.Println(err)
		}
		ut, err := time.Parse(time.RFC3339, rule.UpdatedAt)
		if err != nil {
			fmt.Println(err)
		}

		if rule.ExternalDataReference != "" {
			tmprulemap.Href = rule.Href
			tmprulemap.CreatedAt = ct
			tmprulemap.UpdatedAt = ut
			rulemap[rule.ExternalDataReference] = tmprulemap
		}
	}

	//Check to see there was a match of BOTH TO and FROM Ruleset.
	if !(foundfrom) {
		fmt.Println("Command requires from-Group Name to match a Group Name. If Endpoint group name has spaces ecapsulate the group with \" \". See usage help.")
		utils.LogInfo("from-group NAME was not matched.", false)
		os.Exit(0)
	}
	if !(foundto) {
		fmt.Println("Command requires to-Group Name to match a Group Name. If Endpoint group name has spaces ecapsulate the group with \" \". See usage help.")
		utils.LogInfo("to-group NAME was not matched.", false)
		os.Exit(0)
	}

	//Check to see if user wants to make changes or just log them.
	if !updatePCE {
		fmt.Println("No rules will be copied. To update PCE requires --updatePCE option.  See usage help.")
		utils.LogInfo(fmt.Sprint("--updatePCE not set.  Rules below WOULD BE created."), false)
	}
	counter := 0
	var provisionneeded bool = false //if create or update rules then provision needed.
	var provisionableRuleHrefs []string
	//Check to see there are rules to copy before looping rules
	if len(fromRules) > 0 {

		for _, fromrule := range fromRules {

			//list rule to copy
			counter++

			//Use Clean Rule Struct
			copyRule := illumioapi.Rule{Href: "",
				Consumers:                   fromrule.Consumers,
				ConsumingSecurityPrincipals: fromrule.ConsumingSecurityPrincipals,
				Description:                 fromrule.Description,
				Enabled:                     fromrule.Enabled,
				IngressServices:             fromrule.IngressServices,
				ResolveLabelsAs:             fromrule.ResolveLabelsAs,
				Providers:                   toProvider,
				SecConnect:                  fromrule.SecConnect,
				Stateless:                   fromrule.Stateless,
				MachineAuth:                 fromrule.MachineAuth,
				UnscopedConsumers:           fromrule.UnscopedConsumers,
				ExternalDataReference:       fromrule.Href, //Store rule that this rule comes from.
			}
			//turn updatedat string into time object
			fromupdatedtime, err := time.Parse(time.RFC3339, fromrule.UpdatedAt)
			if err != nil {
				fmt.Println(err)
			}

			//Create or update Rules
			if updatePCE {
				if rulemap[fromrule.Href].Href == "" {
					//Create Rule
					newrule, a, err := pce.CreateRuleSetRule(savedToRuleSetHref, copyRule)
					utils.LogAPIResp("CreateRuleSetRule", a)
					if err != nil {
						utils.LogError(err.Error())
					}
					utils.LogInfo(fmt.Sprintf("Rule %d - Created new Rule %s based on Rule HREF: %s ", counter, newrule.Href, fromrule.Href), false)
					provisionneeded = true

					//Check to see that the updated time of the rule is greater than existing rules updated
				} else if rulemap[fromrule.Href].UpdatedAt.Before(fromupdatedtime) {

					//Place existing rule HREF back into the copyrule struct so you can update.
					copyRule.Href = rulemap[fromrule.Href].Href
					//Update Rule
					a, err := pce.UpdateRuleSetRules(copyRule)
					utils.LogAPIResp("UpdateRuleSetRules", a)
					if err != nil {
						utils.LogError(err.Error())
					}
					utils.LogInfo(fmt.Sprintf("Rule %d - Updated Rule HREF: %s  With Rule HREF: %s", counter, copyRule.Href, fromrule.Href), false)
					provisionneeded = true
				} else {
					utils.LogInfo(fmt.Sprintf("Rule %d - NO CHANGE to Rule HREF: %s with Rule HREF: %s", counter, rulemap[fromrule.Href].Href, fromrule.Href), false)
				}
			} else {
				utils.LogInfo(fmt.Sprintf("RuleSet- HREF:%s  RULE- Consumer:%+v  Ingress-Service:%+v", savedToRuleSetHref, copyRule.Consumers, copyRule.IngressServices[0]), false)

			}

		}
		utils.LogInfo(fmt.Sprintf("%d Rules found in group %s.", len(fromRules), fromGroup), true)
		//if 1 or more copy or update task run we need to provision those changes.  Otherwise skip.
		if provisionneeded {
			provisionableRuleHrefs = append(provisionableRuleHrefs, savedToRuleSetHref)

			if updatePCE {
				//If do not provision flag set skip otherwise provision all rule hrefs created.
				if !doNotProvision {
					a, err := pce.ProvisionHref(provisionableRuleHrefs, "workloader edgerulecopy")
					utils.LogAPIResp("ProvisionHrefs", a)
					if err != nil {
						utils.LogError(err.Error())
					}
					utils.LogInfo(fmt.Sprintf("Provisioning successful - status code %d", a.StatusCode), false)
				}
			}
		}
	}
	utils.LogEndCommand("edgerulescopy")
}
