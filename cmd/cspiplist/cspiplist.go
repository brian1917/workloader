package cspiplist

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/brian1917/illumioapi"
)

// confirmationURL is the URL of the Azure IP ranges confirmation page
const confirmationURL = "https://www.microsoft.com/en-us/download/details.aspx?id=56519"
const AWSURL = "https://ip-ranges.amazonaws.com/ip-ranges.json"

// ipv6check checks if the ip is ipv6
func ipv6check(ip string) bool {
	parsedIP := net.ParseIP(ip)
	return parsedIP != nil && parsedIP.To4() == nil
}

// removeSubsetIPs removes any IP ranges that are a subset of another IP range
func removeSubsetIPs(uniqueIPs map[string]bool) []string {
	ipNets := []*net.IPNet{}
	for ip := range uniqueIPs {
		_, ipNet, err := net.ParseCIDR(ip)
		//only save ipv4 addresses
		if err != nil && ipv6check(ip) {
			log.Printf("Invalid CIDR: %s", ip)
			continue
		}
		ipNets = append(ipNets, ipNet)
	}

	// Filter out subset IP ranges
	filteredIPs := []string{}
	for i, ipNet1 := range ipNets {
		isSubset := false
		for j, ipNet2 := range ipNets {
			// Check if ipNet1 is a subset of ipNet2 and dont add it to the filtered list
			if i != j && ipNet2.Contains(ipNet1.IP) && ipNet2.String() != ipNet1.String() {
				isSubset = true
				//fmt.Println(ipNet1.String(), "is subset of", ipNet2.String())
				break

			}
		}

		// If ipNet1 is not a subset of any other IP range, add it to the filtered list
		if !isSubset {
			filteredIPs = append(filteredIPs, ipNet1.String())
		}
	}

	return filteredIPs
}

// mergeConsecutiveRanges merges consecutive IP ranges
// loop thorugh the list of ips and merge the consecutive ranges until no more available consecutive IP ranges
func mergeConsecutiveRanges(ips []string) []string {

	filteredIPs := ips
	for {
		tmpIPs := []string{}
		fmt.Println("starting loop", len(tmpIPs))
		ipNets := []*net.IPNet{}
		for _, ip := range filteredIPs {
			_, ipNet, err := net.ParseCIDR(ip)
			if err != nil {
				log.Printf("Invalid CIDR: %s", ip)
				continue
			}
			ipNets = append(ipNets, ipNet)
		}

		// Sort IP networks by IP address
		sort.Slice(ipNets, func(i, j int) bool {
			return bytes.Compare(ipNets[i].IP, ipNets[j].IP) < 0
		})

		for i := 0; i < len(ipNets); i++ {
			current := ipNets[i]
			for j := i + 1; j < len(ipNets); j++ {
				next := ipNets[j]
				if canMerge(current, next) {
					current = merge(current, next)
					i = j // Move index forward to skip merged blocks
				} else {
					//fmt.Println(current.String(), next.String())
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
		log.Fatal(err)
	}
	uniqueIPs := make(map[string]bool)
	for _, awsIPRange := range awsIPRanges.Prefixes {
		uniqueIPs[awsIPRange.IPPrefix] = true
	}
	return uniqueIPs
}

// azurParse unmarshalles the Azure IP ranges JSON file into a list of IP unique IP ranges
func azureParse(data []byte) map[string]bool {

	// Unmarshal the JSON data into the Go structure
	var serviceTags ServiceTags
	if err := json.Unmarshal(data, &serviceTags); err != nil {
		log.Fatal(err)
	}
	uniqueIPs := make(map[string]bool)
	for _, serviceTag := range serviceTags.Values {
		for _, addressPrefix := range serviceTag.Properties.AddressPrefixes {
			uniqueIPs[addressPrefix] = true
		}
	}
	return uniqueIPs
}

// downloadURL is the URL of the Azure IP ranges JSON file
func fetchAzureDownloadURL(url string) (string, error) {

	client := &http.Client{}
	// transport := &http.Transport{
	// 	DisableKeepAlives: true, // Optional: Disable connection reuse
	// 	TLSNextProto:      make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	// }

	// client := &http.Client{
	// 	Transport: transport,
	// }
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/113.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Referer", "https://www.microsoft.com/")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch confirmation page: %w", err)
	}
	fmt.Printf("Protocol Version: %s\n", resp.Proto)
	fmt.Printf("Final URL: %s\n", resp.Request.URL.String())

	for key, values := range resp.Header {
		fmt.Printf("%s: %s\n", key, values)
	}
	defer resp.Body.Close()

	html, err := io.ReadAll(resp.Body)
	fmt.Printf("Response Body: %s\n", string(html))
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
		log.Fatalf("failed to download JSON: %v", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Fatalf("unexpected HTTP status: %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to read JSON data: %v", err)
	}
	return data
}
func awsProcessing(ipListUrl string) []string {
	var cspURL string
	if ipListUrl == "" {
		fmt.Println("Using default AWS URL")
		cspURL = AWSURL
	} else {
		cspURL = ipListUrl
	}

	data := downloadJSON(cspURL)

	awsUniqueIPs := awsParse(data)
	awsFilteredIPs := removeSubsetIPs(awsUniqueIPs)
	awsMergedIPs := mergeConsecutiveRanges(awsFilteredIPs)

	return awsMergedIPs
}

func azureProcessing(ipListUrl string) []string {
	var cspUrl string
	if ipListUrl == "" {
		fmt.Println("Using default Azure URL")
		url, err := fetchAzureDownloadURL(confirmationURL)
		if err != nil {
			fmt.Printf("Error finding download URL: %v\n", err)
			os.Exit(1)
		}
		cspUrl = url
	} else {
		cspUrl = ipListUrl
	}

	data := downloadJSON(cspUrl)

	azureUniqueIPs := azureParse(data)
	azureFilteredIPs := removeSubsetIPs(azureUniqueIPs)
	azureMergedIPs := mergeConsecutiveRanges(azureFilteredIPs)

	return azureMergedIPs

}

func buildCSV(ips []string, csp string) {
	// Create a CSV file
	fileName := fmt.Sprintf("%s-iplist-%s.csv", csp, getCurrentTimeStamp())
	file, err := os.Create(fileName)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Write the header
	header := []string{"ip entry", "description"}
	if _, err := file.WriteString(fmt.Sprintf("%s\n", strings.Join(header, ","))); err != nil {
		log.Fatal(err)
	}

	// Write the IP ranges to the CSV file
	for _, ip := range ips {
		if _, err := file.WriteString(fmt.Sprintf("%s,\n", ip)); err != nil {
			log.Fatal(err)
		}
	}
}

// azureurl is the URL of the Azure IP ranges JSON file

// getCurrentTimeStamp returns the current timestamp in a specific format
func getCurrentTimeStamp() string {
	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d",
		time.Now().Year(), time.Now().Month(), time.Now().Day(),
		time.Now().Hour(), time.Now().Minute(), time.Now().Second())
}

// cspList is the main function that fetches and processes the IP ranges
// It takes the updatePCE, noPrompt, csp, ipListUrl, and pce as arguments
// It fetches the IP ranges from the given URL, parses the JSON data, and writes the unique IP ranges to a file
func cspiplist(updatePCE, noPrompt bool, csp, ipListUrl string, pce *illumioapi.PCE) {

	if csp != "aws" && csp != "azure" {
		fmt.Println("Invalid CSP. Please enter either 'aws' or 'azure'.")
		return
	}

	var consolidateIPs []string
	if csp == "aws" {
		consolidateIPs = awsProcessing(ipListUrl)
		buildCSV(consolidateIPs, "aws")
	} else if csp == "azure" {

		consolidateIPs = azureProcessing(ipListUrl)
		buildCSV(consolidateIPs, "azure")
	}
}
