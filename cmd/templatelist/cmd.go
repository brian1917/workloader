package templatelist

import (
	"fmt"
	"io/ioutil"
	"os"
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
List available Illumio templates.

Segmentation templates are a set of CSV files. By default, workloader looks for an "illumio-template" directory in the current directory. To use a different directory, use the --directory flag.

The update-pce and --no-prompt flags are ignored for this command.`,
	Run: func(cmd *cobra.Command, args []string) {

		// Get the directory
		if directory == "" {
			directory = "illumio-templates/"
		} else if directory[len(directory)-1:] != string(os.PathSeparator) {
			directory = fmt.Sprintf("%s%s", directory, string(os.PathSeparator))
		}

		// Get the files in that directory
		files, err := ioutil.ReadDir(directory)
		if err != nil {
			utils.LogError(err.Error())
		}

		// Create a template map
		templates := make(map[string][]string)

		// Iterate through each file
		for _, f := range files {
			templateType := strings.Split(f.Name(), ".")[1]
			templateName := strings.Split(f.Name(), ".")[0]
			// If the templateName is already in the map, append. Else, add it to map.
			if val, ok := templates[templateName]; ok {
				templates[templateName] = append(val, templateType)
			} else {
				templates[templateName] = []string{templateType}
			}
		}

		// Create a templateNames slice so we can sort it.
		templateNames := []string{}
		for t := range templates {
			templateNames = append(templateNames, t)
		}
		sort.SliceStable(templateNames, func(i, j int) bool {
			return i < j
		})

		// Print the sorted templates and the template types from the map
		for _, t := range templateNames {
			fmt.Printf("%s (%s)\r\n", t, strings.Join(templates[t], ", "))
		}

	},
}
