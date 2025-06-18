package cspiplist

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	ia "github.com/brian1917/illumioapi/v2"
	"github.com/brian1917/workloader/utils"
)

// confirmationURL is the URL of the Azure IP ranges confirmation page
const AZUREURL = "https://www.microsoft.com/en-us/download/details.aspx?id=56519"
const AWSURL = "https://ip-ranges.amazonaws.com/ip-ranges.json"

var originalIPRanges []string

// ipv6check checks if the ip is ipv6
func ipv6check(cidr string) bool {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	if includev6 {
		return false
	}
	return ipnet.IP.To4() == nil
}

// getLastIP calculates the last IP address in a given CIDR range
func getLastIP(ipNet *net.IPNet) net.IP {
	ip := ipNet.IP
	mask := ipNet.Mask

	// Make a copy of IP to avoid modifying original
	lastIP := make(net.IP, len(ip))
	copy(lastIP, ip)

	for i := range lastIP {
		lastIP[i] |= ^mask[i]
	}
	return lastIP
}

// removeSubsetIPs removes any IP ranges that are a subset of another IP range
func removeSubsetIPs(uniqueIPs map[string]bool) []string {

	ipNets := []*net.IPNet{}
	tmpIP := []string{}
	for ip := range uniqueIPs {
		_, ipNet, err := net.ParseCIDR(ip)
		if testIPs {
			tmpIP = append(tmpIP, ip)
		}
		//only save ipv4 addresses
		if err != nil {
			utils.LogWarningf(false, "Invalid CIDR: %s", ip)
			continue
		}
		ipNets = append(ipNets, ipNet)
	}
	if testIPs {
		buildCSV(tmpIP, "original")
	}

	// Filter out subset IP ranges
	filteredIPs := []string{}
	for i, ipNet1 := range ipNets {
		isSubset := false

		for j, ipNet2 := range ipNets {
			if i == j {
				continue
			}

			// Check if ipNet2 fully contains ipNet1 (start AND end IPs)
			if ipNet2.Contains(ipNet1.IP) && ipNet2.Contains(getLastIP(ipNet1)) {
				isSubset = true
				break
			}
		}

		if !isSubset {
			filteredIPs = append(filteredIPs, ipNet1.String())
		}
	}
	if testIPs {
		buildCSV(filteredIPs, "removed-subset")
	}
	return filteredIPs
}

// mergeConsecutiveRanges merges consecutive IP ranges
// loop thorugh the list of ips and merge the consecutive ranges until no more available consecutive IP ranges
func mergeConsecutiveRanges(ips []string) []string {

	filteredIPs := ips
	for {
		tmpIPs := []string{}
		fmt.Println("starting consolidation loop", len(tmpIPs))
		ipNets := []*net.IPNet{}
		for _, ip := range filteredIPs {
			_, ipNet, err := net.ParseCIDR(ip)
			if err != nil {
				utils.LogWarningf(false, "Invalid CIDR: %s", ip)
				continue
			}
			ipNets = append(ipNets, ipNet)
		}

		// Sort IP networks by IP address
		sort.Slice(ipNets, func(i, j int) bool {
			return bytes.Compare(ipNets[i].IP, ipNets[j].IP) < 0
		})

		//logic to merge consecutive IP ranges
		for i := 0; i < len(ipNets); i++ {
			current := ipNets[i]
			for j := i + 1; j < len(ipNets); j++ {
				next := ipNets[j]
				if canMerge(current, next) {
					current = merge(current, next)
					i = j // Move index forward to skip merged blocks
				} else {
					break
				}
			}
			tmpIPs = append(tmpIPs, current.String())
		}
		if len(filteredIPs) == len(tmpIPs) {
			break
		} else {
			filteredIPs = tmpIPs
		}

	}
	return filteredIPs
}

// canMerge checks if two IP networks can be merged
func canMerge(ipNet1, ipNet2 *net.IPNet) bool {
	ones1, bits1 := ipNet1.Mask.Size()
	ones2, bits2 := ipNet2.Mask.Size()

	if bits1 != bits2 || ones1 != ones2 {
		return false
	}

	// Check if the two networks are consecutive and properly aligned for merging
	mask := net.CIDRMask(ones1-1, bits1)
	baseNetwork := ipNet1.IP.Mask(mask)

	return baseNetwork.Equal(ipNet2.IP.Mask(mask))
}

// merge performes the merge two IP networks
func merge(ipNet1, ipNet2 *net.IPNet) *net.IPNet {
	ones, bits := ipNet1.Mask.Size()
	newOnes := ones - 1
	newMask := net.CIDRMask(newOnes, bits)
	newIP := ipNet1.IP.Mask(newMask)
	if bytes.Compare(ipNet2.IP, newIP) < 0 {
		newIP = ipNet2.IP.Mask(newMask)
	}
	return &net.IPNet{IP: newIP, Mask: newMask}
}

// awsParse parses the AWS IP ranges JSON file
func awsParse(data []byte) map[string]bool {

	// Unmarshal the JSON data into the Go structure
	var awsIPRanges AWSIPRanges
	if err := json.Unmarshal(data, &awsIPRanges); err != nil {
		utils.LogErrorf("%s", err)
	}

	uniqueIPs := make(map[string]bool)
	for _, awsIPRange := range awsIPRanges.Prefixes {
		if ipv6check(awsIPRange.IPPrefix) {
			continue
		}
		//saving a list of original IPs ranges for test purposes
		if !uniqueIPs[awsIPRange.IPPrefix] {
			originalIPRanges = append(originalIPRanges, awsIPRange.IPPrefix)
		}
		uniqueIPs[awsIPRange.IPPrefix] = true
	}
	return uniqueIPs
}

// azurParse unmarshalles the Azure IP ranges JSON file into a list of IP unique IP ranges
func azureParse(data []byte) map[string]bool {

	// Unmarshal the JSON data into the Go structure
	var serviceTags ServiceTags
	if err := json.Unmarshal(data, &serviceTags); err != nil {
		utils.LogErrorf("%s", err)
	}
	uniqueIPs := make(map[string]bool)
	for _, serviceTag := range serviceTags.Values {
		for _, addressPrefix := range serviceTag.Properties.AddressPrefixes {
			if ipv6check(addressPrefix) {
				continue
			}
			if !uniqueIPs[addressPrefix] && testIPs {
				originalIPRanges = append(originalIPRanges, addressPrefix)
			}
			uniqueIPs[addressPrefix] = true
		}
	}
	return uniqueIPs
}

func fileRead() map[string]bool {

	uniqueIPs := make(map[string]bool)
	file, err := os.Open(fileName)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // skip empty lines and comments
		}

		_, ipNet, err := net.ParseCIDR(line)
		if err != nil {
			utils.LogErrorf("invalid CIDR in line: %s: %s", line, err)
		}
		if ipv6check(line) {
			continue
		}
		// Check if the IP is already in the map
		if _, exists := uniqueIPs[ipNet.String()]; !exists {
			// Add the IP to the map
			uniqueIPs[ipNet.String()] = true
		}

	}

	return uniqueIPs

}

// fetchAzureDownloadURL fetches the download URL Azure IP ranges JSON file from the webpage
func fetchAzureDownloadURL(url string) (string, error) {

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch confirmation page: %w", err)
	}

	defer resp.Body.Close()

	html, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read confirmation HTML: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("unexpected HTTP status: %s", resp.Status)
	}

	// Look for a URL like: ServiceTags_Public_YYYYMMDD.json
	re := regexp.MustCompile(`https:\/\/download\.microsoft\.com\/download\/[^"]+ServiceTags_Public_\d+\.json`)
	match := re.Find(html)
	if match == nil {
		return "", fmt.Errorf("failed to find download URL in page")
	}

	return string(match), nil
}

// downloadJSON downloads the JSON file from the given URL
func downloadJSON(url string) []byte {
	resp, err := http.Get(url)
	if err != nil {
		utils.LogErrorf("failed to download JSON: %v", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		utils.LogErrorf("unexpected HTTP status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		utils.LogErrorf("failed to read JSON data: %v", err)
	}
	return data
}

// capIPProcessing processes the IP ranges for any of the CSP build today....AWS and Azure today
func cspIPProcessing(csp, ipListUrl string) []string {

	var workingIPList []string
	var uniqueIPs map[string]bool
	switch strings.ToLower(csp) {
	case "aws":
		data := downloadJSON(ipListUrl)
		uniqueIPs = awsParse(data)

	case "azure":
		url := ""
		if ipListUrl == AZUREURL {
			tmpurl, err := fetchAzureDownloadURL(AZUREURL)
			if err != nil {
				utils.LogErrorf("Error finding download URL: %v\n", err)
			}
			url = tmpurl
		}
		data := downloadJSON(url)
		uniqueIPs = azureParse(data)

	case "file":
		uniqueIPs = fileRead()

	default:
		fmt.Println("Invalid CSP. Please enter either 'aws' or 'azure'.")
		return nil
	}
	workingIPList = removeSubsetIPs(uniqueIPs)
	workingIPList = mergeConsecutiveRanges(workingIPList)

	if testIPs {
		testIPRanges(workingIPList)
	}
	return workingIPList
}

func buildCSV(ips []string, csp string) {
	// Create a CSV file
	fileName := fmt.Sprintf("workloader-%s-iplist-%s.csv", csp, getCurrentTimeStamp())
	file, err := os.Create(fileName)
	if err != nil {
		utils.LogErrorf("%s", err)
	}
	defer file.Close()

	// Write the header
	header := []string{"ip entry", "description"}
	if _, err := file.WriteString(fmt.Sprintf("%s\n", strings.Join(header, ","))); err != nil {
		utils.LogErrorf("%s", err)
	}

	// Write the IP ranges to the CSV file
	for _, ip := range ips {
		if ip == "" {
			continue
		}
		if _, err := file.WriteString(fmt.Sprintf("%s,\n", ip)); err != nil {
			utils.LogErrorf("%s", err)
		}
	}
}

// TestIPRanges checks if the original IPs are part of the consolidated IP ranges
// It takes the consolidated IP ranges as an argument
func testIPRanges(consolidatedIPRanges []string) {
	// Loop through the original IPs and check if they are part of the consolidated IP ranges
	for _, originalIPRange := range originalIPRanges {
		found := false
		_, ipInner, err := net.ParseCIDR(originalIPRange)
		if err != nil {
			utils.LogInfof(false, "Invalid CIDR: %s", originalIPRange)
			continue
		}

		for _, consolidatedIPRange := range consolidatedIPRanges {
			_, ipOuter, err := net.ParseCIDR(consolidatedIPRange)
			if err != nil {
				utils.LogInfof(false, "Invalid CIDR: %s", consolidatedIPRange)
				continue
			}

			// Check if the entire original range is within the consolidated range
			if ipOuter.Contains(ipInner.IP) {
				// Calculate the last IP of the original range
				lastIP := make(net.IP, len(ipInner.IP))
				copy(lastIP, ipInner.IP)
				for i := len(lastIP) - 1; i >= 0; i-- {
					lastIP[i] |= ^ipInner.Mask[i]
				}

				// Check if the last IP is also within the consolidated range
				if ipOuter.Contains(lastIP) {
					found = true
					break
				}
			}
		}

		if !found {
			utils.LogInfof(false, "Original IP range %s is not fully within any consolidated range\n", originalIPRange)
		}
	}
}

func compareIPList(pce ia.PCE, iplName string, consolidatedIPs []string) bool {

	ipMap := make(map[string]bool)
	for _, ip := range consolidatedIPs {
		ipMap[ip] = true
	}

	queryParameters := map[string]string{
		"name": iplName,
	}

	// Get IPList
	a, err := pce.GetIPLists(queryParameters, "active")
	utils.LogAPIRespV2("GetAllActiveIPLists", a)
	if err != nil {
		utils.LogError(err.Error())
	}

	if _, ok := pce.IPLists[iplName]; !ok {
		utils.LogErrorf("IPList %s does not exist on the PCE. Please create it first or enter the name correctly.  Names are case sensiive.", iplName)
	}
	if len(*pce.IPLists[iplName].IPRanges) != len(consolidatedIPs) {
		return false
	}

	sameIPL := false
	// Check if all IPs in the PCE IPList are in the consolidated IPs
	pceIPMap := make(map[string]bool)
	for _, ip := range *pce.IPLists[iplName].IPRanges {
		_, _, err := net.ParseCIDR(ip.FromIP)
		if err != nil || ip.ToIP != "" || strings.Contains(ip.FromIP, "!") {
			utils.LogErrorf("IPList is invalid. IPLists must be only CIDRs: %v", ip)
		}
		pceIPMap[ip.FromIP] = true
		if !ipMap[ip.FromIP] {
			// Found an IP in the PCE IPList that's not in the consolidated list
			return false
		}
	}

	// Check if all consolidated IPs are in the PCE IPList
	for _, ip := range consolidatedIPs {
		if !pceIPMap[ip] {
			// Found an IP in the consolidated list that's not in the PCE IPList
			return false
		}
	}

	// If both checks passed, the lists are exactly the same
	sameIPL = true
	return sameIPL
}

//iplexport.ExportIPL(pce, iplName, "tmpipllist.txt")

// getCurrentTimeStamp returns the current timestamp in a specific format
func getCurrentTimeStamp() string {
	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d",
		time.Now().Year(), time.Now().Month(), time.Now().Day(),
		time.Now().Hour(), time.Now().Minute(), time.Now().Second())
}

// cspList is the main function that fetches and processes the IP ranges
// It takes the updatePCE, noPrompt, csp, ipListUrl, and pce as arguments
// It fetches the IP ranges from the given URL, parses the JSON data, and writes the unique IP ranges to a file
func cspiplist(updatePCE, noPrompt bool, csp, ipListUrl, iplName string, pce *ia.PCE) {

	var consolidatedIPs []string
	switch strings.ToLower(csp) {
	case "aws":

		if ipListUrl == "" {
			ipListUrl = AWSURL
		}
		consolidatedIPs = cspIPProcessing(csp, ipListUrl)
	case "azure":
		if ipListUrl == "" {
			ipListUrl = AZUREURL
		}
		consolidatedIPs = cspIPProcessing(csp, ipListUrl)

	case "file":
		if fileName == "" {
			fmt.Println("Please provide a file name.")
			return
		}
		consolidatedIPs = cspIPProcessing(csp, "")
	default:
		fmt.Println("Invalid CSP. Please enter either 'aws' or 'azure'.")
		return
	}
	buildCSV(consolidatedIPs, csp)

	if compareIPList(*pce, iplName, consolidatedIPs) {
		utils.LogInfof(false, "IPList %s is the same as the consolidated IP ranges. No changes made.", iplName)
		return
	}

}
