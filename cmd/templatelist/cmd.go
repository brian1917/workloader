package templatelist

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strings"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

var directory string

func init() {
	TemplateListCmd.Flags().StringVar(&directory, "directory", "", "Directory with template files. Default is workign directory illumio-templates")
}

// TemplateListCmd lists all templates in the PCE
var TemplateListCmd = &cobra.Command{
	Use:   "template-list",
	Short: "List templates available in",
	Long: `
Create a CSV export of all workloads in the PCE. The update-pce and --no-prompt flags are ignored for this command.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Set the directory
		if directory == "" {
			directory = "illumio-templates/"
		}
		files, err := ioutil.ReadDir(directory)
		if err != nil {
			utils.LogError(err.Error())
		}
		fileNames := []string{}
		for _, f := range files {
			fileNames = append(fileNames, strings.Replace(f.Name(), "illumio-template-", "", -1))
		}

		templates := make(map[string][]string)

		for _, f := range fileNames {
			templateType := strings.Split(f, "-")[0]
			templateName := strings.Replace(strings.Replace(f, templateType+"-", "", -1), ".csv", "", -1)
			if val, ok := templates[templateName]; ok {
				templates[templateName] = append(val, templateType)
			} else {
				templates[templateName] = []string{templateType}
			}
		}

		templateNames := []string{}
		for t := range templates {
			templateNames = append(templateNames, t)
		}
		sort.SliceStable(templateNames, func(i, j int) bool {
			return i < j
		})

		for _, t := range templateNames {
			fmt.Printf("%s (%s)\r\n", t, strings.Join(templates[t], ", "))
		}

	},
}
