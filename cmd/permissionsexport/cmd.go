package permissionsexport

import (
	"fmt"
	"strings"
	"time"

	"github.com/brian1917/illumioapi/v2"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// Declare local global variables
var outputFileName string
var noHref, groupsOnly bool

const (
	HeaderHref                 = "href"
	HeaderRole                 = "role"
	HeaderScope                = "scope"
	HeaderAuthSecPrincipalName = "auth_security_principal_name"
	HeaderAuthSecPrincipalHref = "auth_security_principal_href"
	HeaderAuthSecPrincipalType = "auth_security_principal_type"
)

func init() {
	PermissionsExportCmd.Flags().BoolVar(&noHref, "no-href", false, "do not export href column. use this when exporting data to import into different pce.")
	PermissionsExportCmd.Flags().BoolVar(&groupsOnly, "groups-only", false, "only export permissions attached to groups.")
	PermissionsExportCmd.Flags().StringVar(&outputFileName, "output-file", "", "optionally specify the name of the output file location. default is current location with a timestamped filename.")
	PermissionsExportCmd.Flags().SortFlags = false
}

// LabelDimensionExportCmd runs the label-dimension-export command
var PermissionsExportCmd = &cobra.Command{
	Use:   "permissions-export",
	Short: "Create a csv export of all permissions.",
	Long: `
	Create a csv export of all permissions. The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the PCE
		pce, err := utils.GetTargetPCEV2(false)
		if err != nil {
			utils.LogError(err.Error())
		}

		exportPermissions(pce)
	},
}

func exportPermissions(pce illumioapi.PCE) {

	// Start the data slice with headers
	csvData := [][]string{{HeaderAuthSecPrincipalName, HeaderAuthSecPrincipalType, HeaderAuthSecPrincipalHref, HeaderRole, HeaderScope}}
	if !noHref {
		csvData[0] = append(csvData[0], HeaderHref)
	}

	// Get permissions and auth security principals
	apiResps, err := pce.Load(illumioapi.LoadInput{Permissions: true, AuthSecurityPrincipals: true, Labels: true, LabelGroups: true, ProvisionStatus: "active"}, true)
	utils.LogMultiAPIRespV2(apiResps)
	if err != nil {
		utils.LogErrorf("loading pce - %s", err)
	}

	for _, permission := range pce.PermissionsSlice {
		if groupsOnly && pce.AuthSecurityPrincipals[permission.AuthSecurityPrincipal.Href].Type != "group" {
			continue
		}
		csvRow := make(map[string]string)
		csvRow[HeaderHref] = permission.Href
		csvRow[HeaderAuthSecPrincipalName] = pce.AuthSecurityPrincipals[permission.AuthSecurityPrincipal.Href].DisplayName
		csvRow[HeaderAuthSecPrincipalHref] = permission.AuthSecurityPrincipal.Href
		csvRow[HeaderAuthSecPrincipalType] = pce.AuthSecurityPrincipals[permission.AuthSecurityPrincipal.Href].Type

		// Process the Role
		roleStrSplit := strings.Split(permission.Role.Href, "/")
		csvRow[HeaderRole] = roleStrSplit[len(roleStrSplit)-1]

		// Process the scope
		scopeStrSlice := []string{}
		for _, scope := range illumioapi.PtrToVal(permission.Scope) {
			if scope.Label != nil {
				label := pce.Labels[scope.Label.Href]
				scopeStrSlice = append(scopeStrSlice, fmt.Sprintf("%s:%s", label.Key, label.Value))
			}
			if scope.LabelGroup != nil {
				labelGroup := pce.LabelGroups[scope.LabelGroup.Href]
				scopeStrSlice = append(scopeStrSlice, fmt.Sprintf("%s:%s-lg", labelGroup.Key, labelGroup.Name))
			}
		}
		if len(scopeStrSlice) != 0 {
			csvRow[HeaderScope] = strings.Join(scopeStrSlice, "; ")
		}

		// Append
		newRow := []string{}
		for _, header := range csvData[0] {
			newRow = append(newRow, csvRow[header])
		}
		csvData = append(csvData, newRow)

	}

	if len(csvData) > 1 {
		if outputFileName == "" {
			outputFileName = fmt.Sprintf("workloader-permissions-export-%s.csv", time.Now().Format("20060102_150405"))
		}
		utils.WriteOutput(csvData, csvData, outputFileName)
		utils.LogInfo(fmt.Sprintf("%d permissions exported", len(csvData)-1), true)
	} else {
		utils.LogInfo("no permissions in PCE.", true)
	}

}
