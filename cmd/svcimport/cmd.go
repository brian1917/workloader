package svcimport

import (
	"fmt"
	"os"

	"github.com/brian1917/illumioapi"

	"github.com/brian1917/workloader/cmd/svcexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var input Input
var err error

func init() {
	SvcImportCmd.Flags().BoolVarP(&input.Provision, "provision", "p", false, "Provision IP Lists after creating and/or updating.")
}

// IplImportCmd runs the iplist import command
var SvcImportCmd = &cobra.Command{
	Use:   "svc-import [csv file to import]",
	Short: "Create and update services from a CSV.",
	Long: `
Create and update services from a CSV file. 

It's recommended to start with a svc-export command and edit/add to it. The input should have a header row as the first row will be skipped. Acceptable values for the headers are below:
` + "\r\n- " + svcexport.HeaderName + "\r\n" +
		"- " + svcexport.HeaderDescription + "\r\n" +
		"- " + svcexport.HeaderPort + "\r\n" +
		"- " + svcexport.HeaderProto + "\r\n" +
		"- " + svcexport.HeaderProcess + "\r\n" +
		"- " + svcexport.HeaderService + "\r\n" +
		"- " + svcexport.HeaderWinService + "\r\n" +
		"- " + svcexport.HeaderICMPCode + "\r\n" +
		"- " + svcexport.HeaderICMPType + "\r\n" + `	


Notes on input:
- The name field is required. If an HREF field is provided the service will updated. No href means a service will be created.
- Rows that share a common name are the same service. For example, a service that has muliple ports should be separate rows with the same name.
- Ports can be individual values or a range (e.g., 10-20)
	
Recommended to run without --update-pce first to log of what will change. If --update-pce is used, ipl-import will create the IP lists with a  user prompt. To disable the prompt, use --no-prompt.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		input.PCE, err = utils.GetTargetPCE(true)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the services
		input.PCE.Load(illumioapi.LoadInput{Services: true})

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}

		input.Data, err = utils.ParseCSV(args[0])
		if err != nil {
			utils.LogError(err.Error())
		}

		// Get the viper values
		input.UpdatePCE = viper.Get("update_pce").(bool)
		input.NoPrompt = viper.Get("no_prompt").(bool)

		ImportServices(input)
	},
}
