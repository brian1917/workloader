package pcemgmt

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var session, remove bool
var debug bool
var configFilePath string
var err error

func init() {
	AddPCECmd.Flags().BoolVarP(&session, "session", "s", false, "Authentication will be temporary session token. No API Key will be generated.")
}

// AddPCECmd generates the pce.yaml file
var AddPCECmd = &cobra.Command{
	Use:   "pce-add",
	Short: "Adds a PCE to the pce.yaml file.",
	Long: `
Adds a PCE to the pce.yaml file.

The default file name is pce.yaml stored in the current directory.
Set ILLUMIO_PCE environment variable for a custom file location, including file name.
This envrionment variable must be set foor future use so Workloader knows where to look for it. Example:

export ILLUMIO_PCE="/Users/brian/Desktop/login.yaml"

By default, the command will create an API ID and Secret. The --session (-s) flag can be used
to generate a session token that is valid for 10 minutes after inactivity.

The --update-pce and --no-prompt flags are ignored for this command.
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		configFilePath, err = filepath.Abs(viper.ConfigFileUsed())
		if err != nil {
			utils.Log(1, err.Error())
		}
	},
	Run: func(cmd *cobra.Command, args []string) {

		// Get the debug value from viper
		debug = viper.Get("debug").(bool)

		addPCE()
	},
}

//addPCE creates a YAML file for authentication
func addPCE() {

	// Log start
	utils.Log(0, "pce-add command started")

	var err error
	var pce illumioapi.PCE
	var pceName, fqdn, user, pwd, disableTLSStr string
	var port int

	// Start user prompt
	fmt.Println("\r\nDefault values will be shown in [brackets]. Press enter to accept default.")
	fmt.Println("")

	fmt.Print("Name of PCE (no spaces or periods) [default-pce]: ")
	fmt.Scanln(&pceName)
	if pceName == "" {
		pceName = "default-pce"
	}

	// If they don't have a default PCE, make it this one.
	defaultPCE := true
	if viper.IsSet("default_pce_name") {
		defaultPCE = false
	}

	fmt.Print("PCE FQDN: ")
	fmt.Scanln(&fqdn)

	fmt.Print("PCE Port: ")
	fmt.Scanln(&port)

	fmt.Print("Email: ")
	fmt.Scanln(&user)
	user = strings.ToLower(user)

	fmt.Print("Password: ")
	bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
	pwd = string(bytePassword)
	fmt.Println("")

	disableTLS := false
	fmt.Print("Disable TLS verification (true/false) [false]: ")
	fmt.Scanln(&disableTLSStr)
	if strings.ToLower(disableTLSStr) == "true" {
		disableTLS = true
	}

	// If session flag is set, create a PCE struct with session token
	var userLogin illumioapi.UserLogin
	var api []illumioapi.APIResponse
	if session {
		fmt.Println("\r\nAuthenticating ...")
		pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
		userLogin, api, err = pce.Login(user, pwd)
		if debug {
			for _, a := range api {
				utils.LogAPIResp("Login", a)
			}
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("logging into PCE - %s", err))
		}
	} else {
		// If session flag is not set, generate API credentials and create PCE struct
		fmt.Println("\r\nAuthenticating and generating API Credentials...")
		pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
		userLogin, api, err = pce.LoginAPIKey(user, pwd, "Workloader", "Created by Workloader")
		if debug {
			for _, a := range api {
				utils.LogAPIResp("LoginAPIKey", a)
			}
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("error generating API key - %s", err))
		}
	}

	// Write the login configuration
	viper.Set(pceName+".fqdn", pce.FQDN)
	viper.Set(pceName+".port", pce.Port)
	viper.Set(pceName+".org", pce.Org)
	viper.Set(pceName+".user", pce.User)
	viper.Set(pceName+".key", pce.Key)
	viper.Set(pceName+".disableTLSChecking", pce.DisableTLSChecking)
	viper.Set(pceName+".userHref", userLogin.Href)
	if !viper.IsSet("max_entries_for_stdout") {
		viper.Set("max_entries_for_stdout", 100)
	}
	if defaultPCE {
		viper.Set("default_pce_name", pceName)
	}

	if err := viper.WriteConfig(); err != nil {
		utils.Log(1, err.Error())
	}

	// Log
	fmt.Printf("\r\nAdded PCE information to %s\r\n", configFilePath)
	utils.Log(0, "completed pce-add")
}
