package cspiplist

type AWSIPRanges struct {
	SyncToken  string `json:"syncToken"`
	CreateDate string `json:"createDate"`
	Prefixes   []struct {
		IPv4Prefix         string `json:"ip_prefix"`
		Region             string `json:"region"`
		Service            string `json:"service"`
		NetworkBorderGroup string `json:"network_border_group"`
	} `json:"prefixes"`
	IPv6Prefixes []struct {
		IPv6Prefix         string `json:"ipv6_prefix"`
		Region             string `json:"region"`
		Service            string `json:"service"`
		NetworkBorderGroup string `json:"network_border_group"`
	} `json:"ipv6_prefixes"`
}

type AzureServiceTags struct {
	ChangeNumber int    `json:"changeNumber"`
	Cloud        string `json:"cloud"`
	Values       []struct {
		Name       string `json:"name"`
		ID         string `json:"id"`
		Properties struct {
			ChangeNumber    int      `json:"changeNumber"`
			Region          string   `json:"region"`
			RegionID        int      `json:"regionId"`
			Platform        string   `json:"platform"`
			SystemService   string   `json:"systemService"`
			AddressPrefixes []string `json:"addressPrefixes"`
			NetworkFeatures []string `json:"networkFeatures"`
		} `json:"properties"`
	} `json:"values"`
}

type Azure365IPRanges []struct {
	ID                     int      `json:"id"`
	ServiceArea            string   `json:"serviceArea"`
	ServiceAreaDisplayName string   `json:"serviceAreaDisplayName"`
	Urls                   []string `json:"urls,omitempty"`
	Ips                    []string `json:"ips,omitempty"`
	TCPPorts               string   `json:"tcpPorts,omitempty"`
	UDPPorts               string   `json:"udpPorts,omitempty"`
	ExpressRoute           bool     `json:"expressRoute"`
	Category               string   `json:"category"`
	Required               bool     `json:"required"`
	Notes                  string   `json:"notes,omitempty"`
}

type GCPIPRanges struct {
	SyncToken  string `json:"syncToken"`
	CreateDate string `json:"creationTime"`
	Prefixes   []struct {
		IPv4Prefix string `json:"ipv4Prefix"`
		IPv6Prefix string `json:"ipv6Prefix"`
		Scope      string `json:"scope"`
		Service    string `json:"service"`
	} `json:"prefixes"`
}

type IPRangeProperties struct {
	Region  string `json:"region"`
	Service string `json:"service"`
}
