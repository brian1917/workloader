package nen

type BaseSwitchData []struct {
	Name          string   `json:"name,omitempty"`
	IntfName      string   `json:"intfname,omitempty"`
	Href          string   `json:"href,omitempty"`
	Ips           []string `json:"ips"`
	SetsRuleCount int      //number of rules that PCE sends natively using sets
	RuleCount     int      //number of rules if not supporting sets.
	Rules         struct {
		Outbound []struct {
			Action      string `json:"action,omitempty"`
			Port        string `json:"port,omitempty"`
			ProtocolNum string `json:"protocol,omitempty"`
			ProtocolTxt string
			Ips         []string `json:"ips,omitempty"`
			OutHash     uint64   `json:"outhash,omitempty"`
		} `json:"Outbound"`
		Inbound []struct {
			Action      string `json:"action,omitempty"`
			Port        string `json:"port,omitempty"`
			ProtocolNum string `json:"protocol,omitempty"`
			ProtocolTxt string
			Ips         []string `json:"ips,omitempty"`
			InHash      uint64   `json:"inhash,omitempty"`
		} `json:"Inbound"`
	} `json:"rules,omitempty"`
}

type ProtoPort struct {
	Port        string
	ProtocolNum string
	ProtocolTxt string
}

type SwitchACLData struct {
	BaseSwitch BaseSwitchData
	ProtoPort  map[string]ProtoPort
	HashList   map[uint64][]string
}

type BaseSwitchConfig []struct {
	Href   string `json:"href"`
	Config struct {
		EndpointType      string `json:"endpoint_type"`
		Name              string `json:"name"`
		WorkloadDiscovery bool   `json:"workload_discovery"`
	} `json:"config"`
	Workloads []struct {
		Href string `json:"href"`
	} `json:"workloads"`
	NetworkDevice struct {
		Href string `json:"href"`
	} `json:"network_device"`
	Status string `json:"status"`
}

type IntfConfig struct {
	EndpointType      string `json:"endpoint_type"`
	Name              string `json:"name"`
	WorkloadDiscovery bool   `json:"workload_discovery"`
}
