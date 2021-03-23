package snowsync

import (
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brian1917/workloader/cmd/wkldexport"

	"github.com/brian1917/workloader/utils"
)

func snhttp(url string) (string, error) {

	// Create HTTP Client
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Set basic auth
	req.SetBasicAuth(snowUser, snowPwd)

	resp, err := client.Do(req)
	utils.LogDebug(fmt.Sprintf("DEBUG - ServiceNow API HTTP Request: %s %v \r\n", resp.Request.Method, resp.Request.URL))
	utils.LogDebug(fmt.Sprintf("DEBUG - ServiceNow API HTTP Reqest Header: %v \r\n", resp.Request.Header))
	utils.LogDebug(fmt.Sprintf("DEBUG - ServiceNow API Response Status Code: %d \r\n", resp.StatusCode))
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Process the response.
	// If the response has 5 entries, we are not doing unmanaged workloads so we need to append those fields
	// Otherwise, the response should have IP address so we just append the placeholder for name.
	bodyBytes, _ := ioutil.ReadAll(resp.Body)
	bodyString := string(bodyBytes)
	reader := csv.NewReader(strings.NewReader(bodyString))
	data, err := reader.ReadAll()
	if err != nil {
		return "", err
	}

	finalData := [][]string{{wkldexport.HeaderHostname, wkldexport.HeaderRole, wkldexport.HeaderApp, wkldexport.HeaderEnv, wkldexport.HeaderLoc, wkldexport.HeaderInterfaces}}
	for i, d := range data {
		if i == 0 {
			continue
		}
		// Start with the match
		x := []string{d[0]}

		// Adjustment is for when fields are omitted
		adjustment := 0

		// Role
		if snowRole == "" {
			x = append(x, "")
			adjustment++
		} else {
			x = append(x, d[1])
		}

		// App
		if snowApp == "" {
			x = append(x, "")
			adjustment++
		} else {
			x = append(x, d[2-adjustment])
		}

		// Env
		if snowEnv == "" {
			x = append(x, "")
			adjustment++
		} else {
			x = append(x, d[3-adjustment])
		}

		// Loc
		if snowLoc == "" {
			x = append(x, "")
			adjustment++
		} else {
			x = append(x, d[4-adjustment])
		}

		// IP
		if !umwl {
			x = append(x, "")
		} else {
			x = append(x, d[5-adjustment])
		}

		// Append to the final data
		finalData = append(finalData, x)
	}

	// Write the data to CSV
	snowDataFileName := fmt.Sprintf("workloader-snow-rawdata-%s.csv", time.Now().Format("20060102_150405"))
	outFile, err := os.Create(snowDataFileName)
	if err != nil {
		utils.LogError(fmt.Sprintf("creating CSV - %s\n", err))
	}
	writer := csv.NewWriter(outFile)
	writer.WriteAll(finalData)
	if err := writer.Error(); err != nil {
		utils.LogError(fmt.Sprintf("writing CSV - %s\n", err))
	}
	utils.LogInfo(fmt.Sprintf("Created temp SNOW file - %s.", snowDataFileName), false)

	return snowDataFileName, nil
}
