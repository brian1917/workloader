package nen

type BaseSwitchACLData []struct {
	Name          string   `json:"name,omitempty"`
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
		} `json:"Outbound"`
		Inbound []struct {
			Action      string `json:"action,omitempty"`
			Port        string `json:"port,omitempty"`
			ProtocolNum string `json:"protocol,omitempty"`
			ProtocolTxt string
			Ips         []string `json:"ips,omitempty"`
		} `json:"Inbound"`
	} `json:"rules,omitempty"`
}

type ProtoPort struct {
	Port        string
	ProtocolNum string
	ProtocolTxt string
}

type SwitchACLData struct {
	BaseSwitch BaseSwitchACLData
	ProtoPort  map[string]ProtoPort
}
