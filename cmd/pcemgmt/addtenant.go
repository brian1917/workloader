package pcemgmt

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brian1917/illumiocloudapi"
	"github.com/brian1917/workloader/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Set global variables for flags
var tenantNameFlag, tenantIdFlag, clientIdFlag, clientSecretFlag string

func init() {
	AddTenantCmd.Flags().StringVar(&tenantNameFlag, "name", "", "name of tenant. will be prompted if left blank.")
	AddTenantCmd.Flags().StringVar(&tenantIdFlag, "tenant-id", "", "tenant id.")
	AddTenantCmd.Flags().StringVar(&clientIdFlag, "client-id", "", "client id of service account.")
	AddTenantCmd.Flags().StringVar(&clientSecretFlag, "client-secret", "", "client secret of service account.")
	AddPCECmd.Flags().SortFlags = false
}

// AddPCECmd generates the pce.yaml file
var AddTenantCmd = &cobra.Command{
	Use:   "tenant-add",
	Short: "Adds a tenant to the pce.yaml for cloud api access.",
	Long: `
Adds a tenant to the pce.yaml file.

The default file name is pce.yaml stored in the current directory. Use the --config-file flag to set a custom file and use the --config-flag on all subsequent commands. You also use ILLUMIO_CONFIG environment variable.

The command can be automated (avoid prompt) by using flags or the following following environment variables:
ILLUMIO_TENANT_NAME, ILLUMIO_TENANT_ID, ILLUMIO_TENANT_CLIENT_ID, ILLUMIO_TENANT_CLIENT_SECRET.

The --update-pce and --no-prompt flags are ignored for this command.
`,
	PreRun: func(cmd *cobra.Command, args []string) {
		configFilePath, err = filepath.Abs(viper.ConfigFileUsed())
		if err != nil {
			utils.LogError(err.Error())
		}
	},
	Run: func(cmd *cobra.Command, args []string) {

		addTenant()
	},
}

// addPCE creates a YAML file for authentication
func addTenant() {

	// Get all the tenant information - OS Env Var, Flag, Prompt
	var tenantName, tenantId, clientId, clientSecret string
	osEnvVariales := []string{"ILLUMIO_TENANT_NAME", "ILLUMIO_TENANT_ID", "ILLUMIO_TENANT_CLIENT_ID", "ILLUMIO_TENANT_CLIENT_SECRET"}
	flagvariables := []string{tenantName, tenantId, clientId, clientSecret}
	promptName := []string{"Tenant Name", "Tenant Id", "Client ID", "Client Secret"}
	finalVariables := []*string{&tenantName, &tenantId, &clientId, &clientSecret}

	for i, variable := range osEnvVariales {
		if os.Getenv(osEnvVariales[i]) != "" {
			*finalVariables[i] = os.Getenv(variable)
			fmt.Println("debug1")
			continue
		}
		if flagvariables[i] != "" {
			*finalVariables[i] = flagvariables[i]
			fmt.Println("debug2")
			continue
		}
		fmt.Print(promptName[i] + ": ")
		fmt.Scanln(finalVariables[i])
	}

	// Write the login configuration
	viper.Set(tenantName+".tenant_id", tenantId)
	viper.Set(tenantName+".client_id", clientId)
	viper.Set(tenantName+".client_secret", clientSecret)
	if err := viper.WriteConfig(); err != nil {
		utils.LogError(err.Error())
	}

	fmt.Printf("added tenant information to %s\r\n\r\n", configFilePath)
}

func GetTenantByName(name string) (tenant illumiocloudapi.Tenant, err error) {
	// Get the tenant from config file
	if !viper.IsSet(name + ".tenant_id") {
		return tenant, fmt.Errorf("could not retrieve valid tenant because tenant id is blank for %s", name)
	}
	tenant.FriendlyName = name
	tenant.TenantID = viper.Get(name + ".tenant_id").(string)
	if viper.IsSet(name + ".client_id") {
		tenant.ClientID = viper.Get(name + ".client_id").(string)
	}
	if viper.IsSet(name + ".client_secret") {
		tenant.Secret = viper.Get(name + ".client_secret").(string)
	}
	return tenant, nil
}
