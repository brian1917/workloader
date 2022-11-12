package pcemgmt

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var session, useAPIKey, noAuth bool
var debug bool
var configFilePath string
var err error

func init() {
	AddPCECmd.Flags().BoolVarP(&session, "session", "s", false, "authentication will be temporary session token. No API Key will be generated.")
	AddPCECmd.Flags().BoolVarP(&useAPIKey, "api-key", "a", false, "use pre-generated api credentials from an api key or a service account.")
	AddPCECmd.Flags().BoolVarP(&noAuth, "no-auth", "n", false, "do not authenticate to the pce. subsequent commands will require WORKLOADER_API_USER, WORKLOADER_API_KEY, WORKLOADER_ORG environment variables to be set.")
	AddPCECmd.Flags().SortFlags = false
}

// AddPCECmd generates the pce.yaml file
var AddPCECmd = &cobra.Command{
	Use:   "pce-add",
	Short: "Adds a PCE to the pce.yaml file.",
	Long: `
Adds a PCE to the pce.yaml file.

The default file name is pce.yaml stored in the current directory.
Set ILLUMIO_CONFIG environment variable for a custom file location, including file name.
This envrionment variable must be set for future use so Workloader knows where to look for it. Example:

export ILLUMIO_CONFIG="/Users/brian/Desktop/login.yaml"

By default, the command will create an API ID and Secret. The --session (-s) flag can be used
to generate a session token that is valid for 10 minutes after inactivity.

The command can be automated (avoid prompt) by setting the following environment variables:
PCE_NAME, PCE_FQDN, PCE_PORT, PCE_USER, PCE_PWD, PCE_DISABLE_TLS.

The ILLUMIO_LOGIN_SERVER environment variable can be used to specify a login server (note - rarely needed).

The --update-pce and --no-prompt flags are ignored for this command.
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		configFilePath, err = filepath.Abs(viper.ConfigFileUsed())
		if err != nil {
			utils.LogError(err.Error())
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
	utils.LogStartCommand("pce-add")

	var err error
	var pce illumioapi.PCE
	var pceName, fqdn, user, pwd, disableTLSStr string
	var port int

	// Check if all our env variables are set
	envVars := []string{"PCE_NAME", "PCE_FQDN", "PCE_PORT", "PCE_USER", "PCE_PWD", "PCE_DISABLE_TLS"}
	auto := true
	for _, e := range envVars {
		if os.Getenv(e) == "" {
			auto = false
		}
	}

	// Start user prompt
	if !auto {
		fmt.Println("\r\nDefault values will be shown in [brackets]. Press enter to accept default.")
		fmt.Println("")
	}

	pceName = os.Getenv("PCE_NAME")
	if pceName == "" {
		fmt.Print("Name of PCE (no spaces or periods) [default-pce]: ")
		fmt.Scanln(&pceName)
		for strings.Contains(pceName, ".") {
			fmt.Println("\r\n[WARNING] - The name of the PCE cannot contain periods. Please re-enter.")
			fmt.Print("Name of PCE (no spaces or periods) [default-pce]: ")
			fmt.Scanln(&pceName)
		}
		if pceName == "" {
			pceName = "default-pce"
		}
	}

	// If they don't have a default PCE, make it this one.
	defaultPCE := true
	if viper.IsSet("default_pce_name") {
		defaultPCE = false
	}

	fqdn = os.Getenv("PCE_FQDN")
	if fqdn == "" {
		fmt.Print("PCE FQDN: ")
		fmt.Scanln(&fqdn)
	}

	portStr := os.Getenv("PCE_PORT")
	if portStr == "" {
		fmt.Print("PCE Port: ")
		fmt.Scanln(&port)
	} else {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	var apiUser, apiKey, orgStr string
	var org int
	if !noAuth {
		if useAPIKey {
			fmt.Print("API Authentication Username: ")
			fmt.Scanln(&apiUser)

			fmt.Print("API Secret: ")
			fmt.Scanln(&apiKey)

			fmt.Print("Org: ")
			fmt.Scanln(&orgStr)
			org, err = strconv.Atoi(orgStr)
			if err != nil {
				utils.LogError(err.Error())
			}

		} else {
			user = os.Getenv("PCE_USER")
			if user == "" {
				fmt.Print("Email: ")
				fmt.Scanln(&user)
			}
			user = strings.ToLower(user)

			pwd = os.Getenv("PCE_PWD")
			if pwd == "" {
				fmt.Print("Password: ")
				bytePassword, _ := term.ReadPassword(int(syscall.Stdin))
				pwd = string(bytePassword)
				fmt.Println("")
			}
		}
	}

	disableTLS := false
	disableTLSEnv := os.Getenv("PCE_DISABLE_TLS")
	if strings.ToLower(disableTLSEnv) == "true" {
		disableTLS = true
	} else if disableTLSEnv == "" {
		fmt.Print("Disable TLS verification (true/false) [false]: ")
		fmt.Scanln(&disableTLSStr)
		if strings.ToLower(disableTLSStr) == "true" {
			disableTLS = true
		}
	}

	var userLogin illumioapi.UserLogin

	if useAPIKey {
		// Build the PCE
		pce.FQDN = fqdn
		pce.Port = port
		pce.Org = org
		pce.User = apiUser
		pce.Key = apiKey
		pce.DisableTLSChecking = disableTLS
		_, api, _ := pce.GetVersion()
		if api.StatusCode != 200 {
			utils.LogError(fmt.Sprintf("checking credentials by getting PCE version returned a status code of %d.", api.StatusCode))
		}

	} else {

		// If session flag is set, create a PCE struct with session token
		var apiResponses []illumioapi.APIResponse
		if session {
			fmt.Println("\r\nAuthenticating ...")
			pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
			userLogin, apiResponses, err = pce.Login(user, pwd)
			for _, a := range apiResponses {
				utils.LogAPIResp("Login", a)
			}
			if err != nil {
				utils.LogError(fmt.Sprintf("logging into PCE - %s", err))
			}
		} else if !noAuth {
			// If session flag is not set, generate API credentials and create PCE struct
			if auto {
				fmt.Println("Authenticating and generating API Credentials...")
			} else {
				fmt.Println("\r\nAuthenticating and generating API Credentials...")
			}
			pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
			userLogin, apiResponses, err = pce.LoginAPIKey(user, pwd, "Workloader", "Created by Workloader")
			if debug {
				for _, a := range apiResponses {
					utils.LogAPIResp("LoginAPIKey", a)
				}
			}
			if err != nil {
				utils.LogError(fmt.Sprintf("error generating API key - %s", err))
			}
		} else {
			pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
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
		utils.LogError(err.Error())
	}

	// Log
	if auto {
		fmt.Printf("Added PCE information to %s\r\n\r\n", configFilePath)
	} else {
		fmt.Printf("\r\nAdded PCE information to %s\r\n\r\n", configFilePath)
	}
	utils.LogEndCommand("pce-add")
}
