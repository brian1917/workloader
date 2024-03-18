package pcemgmt

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var session, useAPIKey, noAuth, proxy bool
var configFilePath, pceNameFlag, pceFQDNFlag, pcePortFlag, pceUserFlag, pcePasswordFlag, pceApiKeyFlag, pceApiUserFlag, pceDisableTLSFlag, pceLoginServer, pceOrg string
var err error

func init() {
	AddPCECmd.Flags().StringVar(&pceNameFlag, "name", "", "name of pce. will be prompted if left blank.")
	AddPCECmd.Flags().StringVar(&pceFQDNFlag, "fqdn", "", "fqdn of pce. will be prompted if left blank.")
	AddPCECmd.Flags().StringVar(&pcePortFlag, "port", "", "port of pce. will be prompted if left blank.")
	AddPCECmd.Flags().StringVar(&pceUserFlag, "email", "", "email to login to pce. will be prompted if left blank.")
	AddPCECmd.Flags().StringVar(&pceApiUserFlag, "api-user", "", "api user. will be prompted if left blank and using api-key flag.")
	AddPCECmd.Flags().StringVar(&pceApiKeyFlag, "api-secret", "", "api secret. will be prompted if left blank and using api-key flag.")
	AddPCECmd.Flags().StringVar(&pceOrg, "org", "", "org. will be prompted if left blank and using api-key flag.")
	AddPCECmd.Flags().StringVar(&pcePasswordFlag, "pwd", "", "password to login to pce. will be prompted if left blank.")
	AddPCECmd.Flags().StringVar(&pceDisableTLSFlag, "disable-tls-verification", "", "disable tls verification to pce. must be blank, true, or false")
	AddPCECmd.Flags().StringVar(&pceLoginServer, "login-server", "", "login server. almost always blank")
	AddPCECmd.Flags().BoolVarP(&session, "session", "s", false, "authentication will be temporary session token. No API Key will be generated.")
	AddPCECmd.Flags().BoolVarP(&proxy, "proxy", "p", false, "set a proxy. can be changed later with clear-proxy and set-proxy commands.")
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

The default file name is pce.yaml stored in the current directory. Use the --config-file flag to set a custom file and use the --config-flag on all subsequent commands. You also use ILLUMIO_CONFIG environment variable.

The command can be automated (avoid prompt) by using flags or the following following environment variables:
PCE_NAME, PCE_FQDN, PCE_PORT, PCE_USER, PCE_PWD, PCE_DISABLE_TLS, PCE_PROXY.

The --update-pce and --no-prompt flags are ignored for this command.
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		configFilePath, err = filepath.Abs(viper.ConfigFileUsed())
		if err != nil {
			utils.LogError(err.Error())
		}
	},
	Run: func(cmd *cobra.Command, args []string) {

		addPCE()
	},
}

// addPCE creates a YAML file for authentication
func addPCE() {

	var err error
	var pce illumioapi.PCE
	var pceName, fqdn, user, pwd, disableTLSStr, proxyServer string
	var port int

	// Check if all our env variables are set
	envVars := []string{"PCE_NAME", "PCE_FQDN", "PCE_PORT", "PCE_USER", "PCE_PWD", "PCE_DISABLE_TLS", "PCE_PROXY"}
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

	pceName = pceNameFlag
	if pceName == "" {
		pceName = os.Getenv("PCE_NAME")
	}
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

	fqdn = pceFQDNFlag
	if fqdn == "" {
		fqdn = os.Getenv("PCE_FQDN")
	}
	if fqdn == "" {
		fmt.Print("PCE FQDN: ")
		fmt.Scanln(&fqdn)
	}

	portStr := pcePortFlag
	if portStr == "" {
		portStr = os.Getenv("PCE_PORT")
	}
	if portStr == "" {
		fmt.Print("PCE Port: ")
		fmt.Scanln(&port)
	} else {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			utils.LogError(err.Error())
		}
	}

	if proxy {
		proxyServer = os.Getenv("PCE_PROXY")
		if proxyServer == "" {
			fmt.Print("Proxy Server (http://server:port): ")
			fmt.Scanln(&proxyServer)
		}
	}

	var apiUser, apiKey string
	var org int

	// Get api key information if flag is set
	if useAPIKey {
		apiUser = pceApiUserFlag
		if apiUser == "" {
			apiUser = os.Getenv("API_USER")
		}
		if apiUser == "" {
			fmt.Print("API Authentication Username: ")
			fmt.Scanln(&apiUser)
		}

		apiKey = pceApiKeyFlag
		if apiKey == "" {
			apiKey = os.Getenv("API_SECRET")
		}
		if apiKey == "" {
			fmt.Print("API Secret: ")
			fmt.Scanln(&apiKey)
		}

		if pceOrg == "" {
			pceOrg = os.Getenv("PCE_ORG")
		}
		if pceOrg == "" {
			fmt.Print("Org: ")
			fmt.Scanln(&pceOrg)
		}

	}

	// If not using an API key or skipping auth, get the email and password
	if !noAuth && !useAPIKey {
		user = pceUserFlag
		if user == "" {
			user = os.Getenv("PCE_USER")
		}
		if user == "" {
			fmt.Print("Email: ")
			fmt.Scanln(&user)
		}
		user = strings.ToLower(user)

		pwd = pcePasswordFlag
		if pwd == "" {
			pwd = os.Getenv("PCE_PWD")
		}
		if pwd == "" {
			fmt.Print("Password: ")
			bytePassword, _ := term.ReadPassword(int(syscall.Stdin))
			pwd = string(bytePassword)
			fmt.Println("")
		}
	}

	// Get the disable tls
	disableTLS := false
	if strings.ToLower(pceDisableTLSFlag) == "true" || strings.ToLower(pceDisableTLSFlag) == "false" {
		disableTLS, _ = strconv.ParseBool(pceDisableTLSFlag)
	} else {
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
	}

	var userLogin illumioapi.UserLogin

	// If using an API key, build the PCE and check authentication
	if useAPIKey {
		pce.FQDN = fqdn
		pce.Port = port
		pce.Proxy = proxyServer
		pce.Org = org
		pce.User = apiUser
		pce.Key = apiKey
		pce.DisableTLSChecking = disableTLS
		_, api, _ := pce.GetVersion()
		if api.StatusCode != 200 {
			utils.LogError(fmt.Sprintf("checking credentials by getting PCE version returned a status code of %d.", api.StatusCode))
		}
	}

	// Process session if set
	var apiResponses []illumioapi.APIResponse
	if !noAuth && !useAPIKey {
		// Generate session credentials if session flag set
		if session {
			fmt.Println("\r\nAuthenticating ...")
			pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
			userLogin, apiResponses, err = pce.Login(user, pwd, pceLoginServer)
			for _, a := range apiResponses {
				utils.LogAPIRespV2("Login", a)
			}
			if err != nil {
				utils.LogError(fmt.Sprintf("logging into PCE - %s", err))
			}
		} else {
			// Generate API keys
			if auto {
				fmt.Println("Authenticating and generating API Credentials...")
			} else {
				fmt.Println("\r\nAuthenticating and generating API Credentials...")
			}
			pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS}
			userLogin, apiResponses, err = pce.LoginAPIKey(user, pwd, "workloader", "created by workloader", pceLoginServer)
			for _, a := range apiResponses {
				utils.LogAPIRespV2("LoginAPIKey", a)
			}
			if err != nil {
				utils.LogError(fmt.Sprintf("error generating API key - %s", err))
			}
		}
	} else if noAuth {
		// Generate PCE if no auth set
		pce = illumioapi.PCE{FQDN: fqdn, Port: port, DisableTLSChecking: disableTLS, Org: org}
	}

	// Write the login configuration
	viper.Set(pceName+".fqdn", pce.FQDN)
	viper.Set(pceName+".port", pce.Port)
	viper.Set(pceName+".org", pce.Org)
	viper.Set(pceName+".user", pce.User)
	viper.Set(pceName+".key", pce.Key)
	viper.Set(pceName+".disableTLSChecking", pce.DisableTLSChecking)
	viper.Set(pceName+".userHref", userLogin.Href)
	viper.Set(pceName+".proxy", pce.Proxy)
	if !viper.IsSet("max_entries_for_stdout") {
		viper.Set("max_entries_for_stdout", 100)
	}
	if defaultPCE {
		viper.Set("default_pce_name", pceName)
	}

	if err := viper.WriteConfig(); err != nil {
		utils.LogError(err.Error())
	}

	_, api, err := pce.GetVersion()
	utils.LogAPIRespV2("GetVersion", api)
	if err != nil {
		utils.LogErrorf("getting pce version - %s - %s - %d", err, api.RespBody, api.StatusCode)
	}
	viper.Set(pceName+".pce_version", fmt.Sprintf("%d.%d.%d-%d", pce.Version.Major, pce.Version.Minor, pce.Version.Patch, pce.Version.Build))
	if err := viper.WriteConfig(); err != nil {
		utils.LogError(err.Error())
	}

	// Log
	if auto {
		fmt.Printf("Added PCE information to %s\r\n\r\n", configFilePath)
	} else {
		fmt.Printf("\r\nAdded PCE information to %s\r\n\r\n", configFilePath)
	}

}
