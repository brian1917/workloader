package login

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
)

// Set global variables for flags
var session, remove, clear bool
var debug bool

func init() {
	LoginCmd.Flags().BoolVarP(&session, "session", "s", false, "Authentication will be temporary session token. No API Key will be generated.")
	LoginCmd.Flags().BoolVarP(&remove, "remove", "r", false, "Remove existing JSON authentication file.")
	LoginCmd.Flags().BoolVarP(&clear, "clear", "x", false, "Remove existing JSON authentication file and clear all Workloader generated API credentials from the PCE.")

	LoginCmd.Flags().SortFlags = false
}

// LoginCmd generates the pce.json file
var LoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Verifies existing login or generates a pce.json file for authentication used for all other commands.",
	Long: `
Login verifies an existing login or generates a json file that is used for authentication for all other commands.
If the --remove (-r) flag or --clear (-c) flag is set, the login sequence will not run.

The default file name is pce.json stored in the current directory.
Set ILLUMIO_PCE environment variable for a custom file location, including file name.
This envrionment variable must be set foor future use so Workloader knows where to look for it. Example:

export ILLUMIO_PCE="/Users/brian/Desktop/login.json"

By default, the command will create an API ID and Secret. The --session (-s) flag can be used
to generate a session token that is valid for only the set PCE session time.

The command will prompt for PCE FQDN, port, user email address, and password.
You can avoid being prompted for any/all by setting environmental variables. Example below:

export ILLUMIO_FQDN=pce.demo.com
export ILLUMIO_PORT=8443
export ILLUMIO_USER=joe@test.com
export ILLUMIO_PWD=pwd123
`,
	Run: func(cmd *cobra.Command, args []string) {

		debug = true

		if remove && clear {
			fmt.Println("Remove flag is redundant Clear flag includes remove functionality.")
			clear = false
		}
		if remove {
			removeJSONFile()
		}
		if clear {
			clearAPIKeys()
		}
		if !remove && !clear {
			PCELogin()
		}
	},
}

//PCELogin creates a JSON file for authentication
func PCELogin() {
	var err error
	var pce illumioapi.PCE

	// Log start
	utils.Log(0, "login command started")

	// Check if already logged in
	loginCheck, pce, version := verifyLogin()
	if loginCheck {
		fmt.Printf("Login is still valid to %s. PCE Version %s\r\n", pce.FQDN, version.LongDisplay)
		utils.Log(0, fmt.Sprintf("login is still valid to %s - pce version %s", pce.FQDN, version.LongDisplay))
		return
	}

	// Get environment variables
	fqdn := os.Getenv("ILLUMIO_FQDN")
	port, _ := strconv.Atoi(os.Getenv("ILLUMIO_PORT"))
	user := os.Getenv("ILLUMIO_USER")
	pwd := os.Getenv("ILLUMIO_PWD")
	disableTLSStr := os.Getenv("ILLUMIO_DISABLE_TLS")

	// Check if there is an existing PCE
	existingPCE, _ := utils.GetPCE()

	fmt.Println("\r\nDefault values will be shown in [brackets]. Press enter to accept default.")
	fmt.Println("")

	// FQDN - if env variable isn't set, prompt for it.
	if fqdn == "" {
		// Set default value if there is an existing and no longer valid pce.json file
		defaultValue := fmt.Sprintf(" [%s]", existingPCE.FQDN)
		if existingPCE.FQDN == "" {
			defaultValue = ""
		}
		fmt.Print("PCE FQDN" + defaultValue + ": ")
		fmt.Scanln(&fqdn)
		if fqdn == "" {
			fqdn = existingPCE.FQDN
		}
	}

	// Set default port to existing PCE port
	defaultPort := existingPCE.Port
	// If there is no existing pce, set default to 8443
	if defaultPort == 0 {
		defaultPort = 8443
	}
	// If the FQDN is illum.io, set default to 443
	if len(fqdn) > 10 && fqdn[len(fqdn)-9:] == ".illum.io" {
		defaultPort = 443
	}
	// If the port environment variable isn't set, prompt for it
	if port == 0 {
		fmt.Printf("PCE Port [%d] :", defaultPort)
		fmt.Scanln(&port)
		// If user accpeted default, assign it
		if port == 0 {
			port = defaultPort
		}
	}

	// User
	if user == "" {
		fmt.Print("Email: ")
		fmt.Scanln(&user)
	}

	// Password
	if pwd == "" {
		fmt.Print("Password: ")
		bytePassword, _ := terminal.ReadPassword(int(syscall.Stdin))
		pwd = string(bytePassword)
		fmt.Println("")
	}

	// Disable TLS
	disableTLS := false
	if strings.ToLower(disableTLSStr) != "true" {
		fmt.Print("Disable TLS verification (true/false) [false]: ")
		fmt.Scanln(&disableTLSStr)
		if strings.ToLower(disableTLSStr) == "true" {
			disableTLS = true
		}
	} else {
		disableTLS = true
	}

	// If session flag is set, create a PCE struct with session token
	var userLogin illumioapi.UserLogin
	if session {
		fmt.Println("Authenticating ...")
		pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
		userLogin, _, err = pce.Login(user, pwd)
		if err != nil {
			utils.Log(1, fmt.Sprintf("logging into PCE - %s", err))
		}
	} else {
		// If session flag is not set, generate API credentials and create PCE struct
		fmt.Println("Authenticating and generating API Credentials...")
		pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
		userLogin, _, err = pce.LoginAPIKey(user, pwd, "Workloader", "Created by Workloader")
		if err != nil {
			utils.Log(1, fmt.Sprintf("error generating API key - %s", err))
		}
	}

	// Write the PCE struct to a json file
	pceFile, _ := json.MarshalIndent(utils.UserInfo{PCE: pce, User: illumioapi.UserLogin{FullName: userLogin.FullName, Orgs: userLogin.Orgs, Href: userLogin.Href}}, "", "    ")
	_ = ioutil.WriteFile(getJSONFileLoc(), pceFile, 0644)

	// Log
	fmt.Printf("Created %s\r\n", getJSONFileLoc())
	utils.Log(0, fmt.Sprintf("login successful - created %s", getJSONFileLoc()))

}

func verifyLogin() (bool, illumioapi.PCE, illumioapi.Version) {

	// Get the PCE
	pce, err := utils.GetPCE()
	if err != nil {
		return false, pce, illumioapi.Version{}
	}

	// If the pce is the same and still works, get the version.
	version, err := pce.GetVersion()
	if err != nil {
		return false, pce, illumioapi.Version{}
	}

	return true, pce, version

}

func removeJSONFile() {

	utils.Log(0, "login remove started...")

	utils.Log(0, fmt.Sprintf("location of authentication file is %s", getJSONFileLoc()))

	if _, err := os.Stat(getJSONFileLoc()); os.IsNotExist(err) {
		utils.Log(0, "authentication file does not exist")
		return
	}

	if err := os.Remove(getJSONFileLoc()); err != nil {
		utils.Log(1, fmt.Sprintf("error deleting file - %s", err))
	}

	utils.Log(0, fmt.Sprintf("successfully deleted %s", getJSONFileLoc()))

}

func clearAPIKeys() {

	// Log start of command
	utils.Log(0, "login clear started...")

	// Get the PCE
	pce, err := utils.GetPCE()
	if err != nil {
		utils.Log(1, err.Error())
	}

	// Get all API Keys
	user, err := utils.GetUser()
	if err != nil {
		utils.Log(1, err.Error())
	}
	apiKeys, _, err := pce.GetAllAPIKeys(user.Href)
	if err != nil {
		utils.Log(1, err.Error())
	}

	// Delete the API keys that are from Workloader
	for _, a := range apiKeys {
		if a.Name == "Workloader" {
			_, err := pce.DeleteHref(a.Href)
			if err != nil {
				utils.Log(1, err.Error())
			}
			utils.Log(0, fmt.Sprintf("deleted %s", a.Href))
		}
	}

	// Remove the jsonFile
	removeJSONFile()
}

func getJSONFileLoc() string {

	// The file is either set in the ILLUMIO_PCE environment variable
	file := os.Getenv("ILLUMIO_PCE")

	// Or, we create a pce.json file in the current directory
	if file == "" {
		path, err := os.Getwd()
		if err != nil {
			utils.Log(1, fmt.Sprintf("getting current directory value - %s", err))
		}
		file = path + "/pce.json"
	}

	return file
}
