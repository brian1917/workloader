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

func init() {
	LoginCmd.Flags().BoolP("session", "s", false, "Authentication will be temporary session token. No API Key will be generated.")

	LoginCmd.Flags().SortFlags = false
}

// LoginCmd generates the pce.json file
var LoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Generates a pce.json file for authentication used for all other commands.",
	Long: `
Login generates a json file that is used for authentication for all other commands.

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

		session, _ := cmd.Flags().GetBool("session")

		PCELogin(session)
	},
}

//PCELogin creates a JSON file for authentication
func PCELogin(session bool) {

	var pce illumioapi.PCE
	var err error

	// Get environment variables
	fqdn := os.Getenv("ILLUMIO_FQDN")
	port, _ := strconv.Atoi(os.Getenv("ILLUMIO_PORT"))
	user := os.Getenv("ILLUMIO_USER")
	pwd := os.Getenv("ILLUMIO_PWD")
	disableTLSStr := os.Getenv("ILLUMIO_DISABLE_TLS")

	// Get pce
	existingPCE, _ := utils.GetPCE()

	fmt.Println("\r\nDefault values will be shown in [brackets]. Press enter to accept default.")
	fmt.Println("")

	if fqdn == "" {
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

	if port == 0 {
		defaultValue := fmt.Sprintf(" [%d]", existingPCE.Port)
		if existingPCE.Port == 0 {
			defaultValue = fmt.Sprintf(" [%d]", 8443)
		}
		fmt.Print("PCE Port" + defaultValue + ": ")
		fmt.Scanln(&port)
		if port == 0 {
			port = existingPCE.Port
		}
	}

	if user == "" {
		fmt.Print("Email: ")
		fmt.Scanln(&user)
	}

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
		disableTLS, err = strconv.ParseBool(disableTLSStr)
		if err != nil {
			utils.Log(1, fmt.Sprintf("invalid disableTLS value - %s", err))
		}
	} else {
		disableTLS = true
	}

	if session {
		fmt.Println("Authenticating ...")
		pce, err = illumioapi.PCEbuilder(fqdn, user, pwd, port, disableTLS)
		if err != nil {
			utils.Log(1, fmt.Sprintf("building PCE - %s", err))
		}
	} else {
		fmt.Println("Authenticating and generating API Credentials...")
		pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
		pce, _, err = illumioapi.CreateAPIKey(pce, user, pwd, "Workloader", "Created by Workloader")
		if err != nil {
			utils.Log(1, fmt.Sprintf("error generating API key - %s", err))
		}
	}

	pceFile, _ := json.MarshalIndent(pce, "", "")

	file := os.Getenv("ILLUMIO_PCE")
	if file == "" {
		path, err := os.Getwd()
		if err != nil {
			utils.Log(1, fmt.Sprintf("getting current directory value - %s", err))
		}
		file = path + "/pce.json"
	}

	_ = ioutil.WriteFile(file, pceFile, 0644)

	fmt.Printf("Created %s\r\n", file)

}
