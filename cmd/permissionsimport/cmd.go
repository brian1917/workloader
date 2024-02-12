package permissionsimport

import (
	"fmt"
	"os"
	"strings"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/cmd/permissionsexport"
	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {}

type processedPermission struct {
	permissions illumioapi.Permission
	csvLine     int
}

// PermissionsImportCmd runs the PermissionsImportCmd command
var PermissionsImportCmd = &cobra.Command{
	Use:   "permissions-import",
	Short: "Create or updated scoped permissions in the PCE from a csv file.",
	Long: `
Create or updated scoped permissions in the PCE from a csv file.

Creating new permissions and updating existing permissions in the same execution is not possible.

Header order does not matter and additional headers will not cause issue.
	
To create permissions, the required headers are
 - auth_security_principal_name (group name, NOT display name)
 - role (see below for valid options)
 - scope (scope entries must be key:value semi-colon separated and label groups should have a "-lg" suffix. For example, app:erp;env:non-prod-lg.)

Only the role field can be updated. It is recommend to start with an export from permissions-export. The required headers are
- href (existing permission href)
- role (see below for valid options)
	
Valid role options include the following:
` + strings.Join(illumioapi.AvailableRolesSlice(), ", "),
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Set the CSV file
		if len(args) != 1 {
			fmt.Println("Command requires 1 argument for the csv file. See usage help.")
			os.Exit(0)
		}

		importPermissions(pce, args[0], viper.Get("update_pce").(bool), viper.Get("no_prompt").(bool))
	},
}

func importPermissions(pce illumioapi.PCE, csvFile string, updatePCE, noPrompt bool) {

	// Get permissions and auth security principals
	apiResps, err := pce.Load(illumioapi.LoadInput{Permissions: true, AuthSecurityPrincipals: true, Labels: true, LabelGroups: true, ProvisionStatus: "active"}, true)
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogErrorf("loading pce - %s", err)
	}

	// Parse the CSV
	csvData, err := utils.ParseCSV(csvFile)
	if err != nil {
		utils.LogErrorf("parsing csv - %s", err)
	}

	// Create the slice for new groups
	newPermissions := []processedPermission{}
	updatedPermissions := []processedPermission{}

	// Iterate over the CSV
	headersMap := make(map[string]int)
	for rowIndex, row := range csvData {
		// parse the headers
		if rowIndex == 0 {
			for colIndex, header := range row {
				headersMap[header] = colIndex
			}
			continue
		}

		// Create the new sec principal
		permission := illumioapi.Permission{}
		existingPermission := illumioapi.Permission{}

		// Href - determins update or create
		if colIndex, exists := headersMap[permissionsexport.HeaderHref]; exists {
			utils.LogInfof(true, "href column provided - only updating permissions")
			// Get the existing permission
			if permission, permissionExists := pce.Permissions[row[colIndex]]; !permissionExists {
				utils.LogWarningf(true, "csv row %d - %s doesn't exist - skipping", rowIndex+1, row[colIndex])
				continue
			} else {
				existingPermission = permission
			}
		}

		// Role
		if colIndex, exists := headersMap[permissionsexport.HeaderRole]; exists {
			if !illumioapi.AvailableRoles[row[colIndex]] {
				utils.LogWarningf(true, "csv row %d - invalid role - skipping", rowIndex+1)
				continue
			}
			permission.Role = &illumioapi.Role{Href: fmt.Sprintf("/orgs/%d/roles/%s", pce.Org, row[colIndex])}

			if existingPermission.Href != "" {
				if permission.Role.Href != existingPermission.Role.Href {
					utils.LogInfof(true, "csv line %d - permission to be updated from %s to %s", rowIndex+1, existingPermission.Role.Href, permission.Role.Href)
					existingPermission.Role = permission.Role
					updatedPermissions = append(updatedPermissions, processedPermission{csvLine: rowIndex + 1, permissions: existingPermission})
					continue
				}
			}
		}

		// Auth Principal
		if colIndex, exists := headersMap[permissionsexport.HeaderAuthSecPrincipalName]; exists {
			if _, exists := pce.AuthSecurityPrincipals[row[colIndex]]; !exists {
				utils.LogWarningf(true, "csv row %d - %s does not exist as a authorized security principal - skipping", rowIndex+1, row[colIndex])
				continue
			}
			permission.AuthSecurityPrincipal = &illumioapi.AuthSecurityPrincipal{Href: pce.AuthSecurityPrincipals[row[colIndex]].Href}
		}

		// Scope
		if colIndex, exists := headersMap[permissionsexport.HeaderScope]; exists {
			scope := []illumioapi.Scopes{}
			scopeValue := strings.Replace(row[colIndex], "; ", ";", -1)
			scopeSlice := strings.Split(scopeValue, ";")

			for _, scopeEntry := range scopeSlice {
				scopeEntrySlice := strings.Split(scopeEntry, ":")
				if len(scopeEntrySlice) == 1 {
					utils.LogWarningf(true, "csv row %d - %s is an invalid scope entry - skipping", rowIndex+1, row[colIndex])
					continue
				}
				key := scopeEntrySlice[0]
				value := strings.Join(scopeEntrySlice[1:], ":")
				if strings.HasSuffix(value, "-lg") {
					if lg, lgExists := pce.LabelGroups[key+strings.TrimSuffix(value, "-lg")]; !lgExists {
						utils.LogWarningf(true, "csv row %d - %s:%s does not exist as a label group - skipping", rowIndex+1, key, strings.TrimSuffix(value, "-lg"))
						continue
					} else {
						scope = append(scope, illumioapi.Scopes{LabelGroup: &illumioapi.LabelGroup{Href: lg.Href}})
					}
				} else {
					if label, labelExists := pce.Labels[key+value]; !labelExists {
						utils.LogWarningf(true, "csv row %d - %s:%s does not exist as a label - skipping", rowIndex+1, key, value)
						continue
					} else {
						scope = append(scope, illumioapi.Scopes{Label: &illumioapi.Label{Href: label.Href}})
					}
				}
			}
			permission.Scope = &scope
		}

		newPermissions = append(newPermissions, processedPermission{csvLine: rowIndex + 1, permissions: permission})
	}

	// End run of nothing to do
	if len(newPermissions) == 0 && len(updatedPermissions) == 0 {
		utils.LogInfo("nothing to be done.", true)
		return
	}

	if !updatePCE {
		utils.LogInfof(true, "workloader identified %d permissions to create and %d permissions to update. See workloader.log for all identified changes. To do the import, run again using --update-pce flag", len(newPermissions), len(updatedPermissions))
		return
	}

	if !noPrompt {
		var prompt string
		fmt.Printf("[PROMPT] - workloader will create %d permissions and update %d permissions in %s (%s). Do you want to run the import (yes/no)? ", len(newPermissions), len(updatedPermissions), pce.FriendlyName, viper.Get(pce.FriendlyName+".fqdn").(string))

		fmt.Scanln(&prompt)
		if strings.ToLower(prompt) != "yes" {
			utils.LogInfo("Prompt denied.", true)
			return
		}
	}

	// Create the permissions
	for _, permission := range newPermissions {
		createdPermission, api, err := pce.CreatePermission(permission.permissions)
		utils.LogAPIRespV2("CreatePermission", api)
		if err != nil {
			utils.LogErrorf("csv line %d - error - api status code: %d, api resp: %s", permission.csvLine, api.StatusCode, api.RespBody)
		}
		utils.LogInfof(true, "csv line %d - created %s - %d", permission.csvLine, createdPermission.Href, api.StatusCode)
	}

	// Update permissions
	for _, permission := range updatedPermissions {
		api, err := pce.UpdatePermission(permission.permissions)
		utils.LogAPIRespV2("UpdatePermission", api)
		if err != nil {
			utils.LogErrorf("csv line %d - error - api status code: %d, api resp: %s", permission.csvLine, api.StatusCode, api.RespBody)
		}
		utils.LogInfof(true, "csv line %d - updated %s - %d", permission.csvLine, permission.permissions.Href, api.StatusCode)
	}
}
