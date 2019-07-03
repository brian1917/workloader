package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/brian1917/illumioapi"
	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
)

func init() {
	loginCmd.Flags().BoolP("disableTLS", "x", false, "Disable TLS checking to PCE.")
	loginCmd.Flags().BoolP("session", "s", false, "Authentication will be temporary session token. No API Key will be generated.")

	loginCmd.Flags().SortFlags = false
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Generates a pce.json file for authentication used for all other commands.",
	Long: `
Login generates a pce.json file that is used for authentication for all other commands.
By default, the command will create an API ID and Secret. Optionally, the --session (-s) flag can be used
to generate a session token that is valid for only the set session time.

The command will prompt for PCE FQDN, port, user email address, and password.
You can avoid being prompted for any/all by setting environmental variables. Example below:

export ILLUMIO_FQDN=pce.demo.com
export ILLUMIO_PORT=8443
export ILLUMIO_USER=joe@test.com
export ILLUMIO_PWD=pwd123
`,
	Run: func(cmd *cobra.Command, args []string) {

		disableTLS, _ := cmd.Flags().GetBool("disableTLS")
		session, _ := cmd.Flags().GetBool("session")

		pceLogin(disableTLS, session)
	},
}

func pceLogin(disableTLS, session bool) {

	fqdn := os.Getenv("ILLUMIO_FQDN")
	port, _ := strconv.Atoi(os.Getenv("ILLUMIO_PORT"))
	user := os.Getenv("ILLUMIO_USER")
	pwd := os.Getenv("ILLUMIO_PWD")

	if fqdn == "" {
		fmt.Print("PCE FQDN: ")
		fmt.Scanln(&fqdn)
	}

	if port == 0 {
		fmt.Print("PCE Port: ")
		fmt.Scanln(&port)
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

	var pce illumioapi.PCE
	var err error

	if session {
		fmt.Println("Authenticating ...")
		pce, err = illumioapi.PCEbuilder(fqdn, user, pwd, port, disableTLS)
		if err != nil {
			utils.Logger.Fatalf("ERROR - building PCE - %s", err)
		}
	} else {
		fmt.Println("Authenticating and generating API Credentials...")
		pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
		pce, _, err = illumioapi.CreateAPIKey(pce, user, pwd, "Workloader", "Created by Workloader")
		if err != nil {
			utils.Logger.Fatal(err)
		}
	}

	pceFile, _ := json.MarshalIndent(pce, "", "")

	_ = ioutil.WriteFile("pce.json", pceFile, 0644)

	fmt.Println("pce.json created")

}
