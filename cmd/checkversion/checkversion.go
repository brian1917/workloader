package checkversion

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/brian1917/workloader/utils"
	"github.com/spf13/cobra"
)

// CheckVersionCmd checks if running latest workloader version
var CheckVersionCmd = &cobra.Command{
	Use:   "check-version",
	Short: "Check  if running latest workloader version.",
	Run: func(cmd *cobra.Command, args []string) {
		getLatestVersion()
	},
}

// GitHubAPIResp is the response from the GitHub API
type GitHubAPIResp struct {
	URL             string    `json:"url"`
	AssetsURL       string    `json:"assets_url"`
	UploadURL       string    `json:"upload_url"`
	HTMLURL         string    `json:"html_url"`
	ID              int       `json:"id"`
	NodeID          string    `json:"node_id"`
	TagName         string    `json:"tag_name"`
	TargetCommitish string    `json:"target_commitish"`
	Name            string    `json:"name"`
	Draft           bool      `json:"draft"`
	Prerelease      bool      `json:"prerelease"`
	CreatedAt       time.Time `json:"created_at"`
	PublishedAt     time.Time `json:"published_at"`
	TarballURL      string    `json:"tarball_url"`
	ZipballURL      string    `json:"zipball_url"`
	Body            string    `json:"body"`
}

func getLatestVersion() {
	// Create HTTP client and request
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/brian1917/workloader/releases/latest", nil)
	if err != nil {
		utils.LogError(err.Error())
	}
	// Make HTTP Request
	resp, err := client.Do(req)
	if err != nil {
		utils.LogError(err.Error())
	}
	// Marshal the response
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		utils.LogError(err.Error())
	}
	var ghr GitHubAPIResp
	json.Unmarshal(body, &ghr)

	// Print output
	current := fmt.Sprintf("v%s", utils.GetVersion())
	fmt.Printf("Your version: %s\r\n", current)
	fmt.Printf("Latest version on GitHub Releases: %s\r\n", ghr.TagName)
	if current == ghr.TagName {
		fmt.Println("You are on the latest version of workloader.")
	} else {
		fmt.Println("You are not on the latest version of workloader. Go to https://github.com/brian1917/workloader/releases for the latest.")
	}
}
