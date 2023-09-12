package pcemgmt

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AddPCECmd generates the pce.yaml file
var SetProxyCmd = &cobra.Command{
	Use:   "set-proxy [fqdn:port]",
	Short: "Set workloader-specific proxy.",
	Long: `
Set workloader-specific proxy.

Workloader uses HTTP_PROXY and HTTPS_PROXY environment variables natively. This command is only if the proxy needs to be explicitly set for workloader outside those variables.

The command requires two arguments: pce name and proxy in format of http://fqdn:port (or http://ip:port).

For example, the following command sets the proxy for default-pce to http://proxy.com:8080

workloader set-proxy default-pce http://proxy.com:8080
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		configFilePath, err = filepath.Abs(viper.ConfigFileUsed())
		if err != nil {
			utils.LogError(err.Error())
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		utils.LogStartCommand("set-proxy")
		if len(args) != 2 {
			utils.LogError("command requires 2 arguments for the pce name and the proxy string as fqdn:port. See usage help.")
		}
		pce, err := utils.GetPCEbyName(args[0], false)
		if err != nil {
			utils.LogError(err.Error())
		}
		// Make sure has "http"
		if !strings.Contains(args[1], "http") {
			utils.LogError(fmt.Sprintf("%s is not a valid proxy - it must be in format of http://fqdn:port", args[1]))
		}
		// Make sure valid port
		s := strings.Split(args[1], ":")
		_, err = strconv.Atoi(s[len(s)-1])
		if err != nil {
			utils.LogError(fmt.Sprintf("%s is not a valid proxy - it must be in format of http://fqdn:port", args[1]))
		}
		viper.Set(pce.FriendlyName+".proxy", args[1])
		if err := viper.WriteConfig(); err != nil {
			utils.LogError(err.Error())
		}
		utils.LogEndCommand("set-proxy")

	},
}

// ClearProxyCmd clears any set proxy
var ClearProxyCmd = &cobra.Command{
	Use:   "clear-proxy [pce name]",
	Short: "Clear workloader-specific proxy.",
	Long: `
Clear workloader-specific proxy.
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		configFilePath, err = filepath.Abs(viper.ConfigFileUsed())
		if err != nil {
			utils.LogError(err.Error())
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		utils.LogStartCommand("clear-proxy")
		if len(args) != 1 {
			utils.LogError("command requires 1 argument for the pce name. See usage help.")
		}
		pce, err := utils.GetPCEbyName(args[0], false)
		if err != nil {
			utils.LogError(err.Error())
		}
		viper.Set(pce.FriendlyName+".proxy", "")
		if err := viper.WriteConfig(); err != nil {
			utils.LogError(err.Error())
		}
		utils.LogEndCommand("clear-proxy")

	},
}
