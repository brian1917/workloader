package vcenter

import (
	"net/http"
)

// apiResponse contains the information from the response of the API
type apiResponse struct {
	RespBody   string
	StatusCode int
	Header     http.Header
	Request    *http.Request
	ReqBody    string
}

type vcenterLabels struct {
	KeyID string `json:"name"`
	Key   string `json:"cardinality"`
	Value string `json:"description"`
}

type categoryDetail struct {
	Name            string   `json:"name"`
	Cardinality     string   `json:"cardinality"`
	Description     string   `json:"description"`
	ID              string   `json:"id"`
	AssociableTypes []string `json:"associable_types"`
	UsedBy          []string `json:"used_by"`
}

type tagDetail struct {
	Name        string   `json:"name"`
	CategoryID  string   `json:"category_id"`
	Description string   `json:"description"`
	ID          string   `json:"id"`
	UsedBy      []string `json:"used_by"`
}
type vmwareDetail struct {
	Name      string `json:"name"`
	HostName  string `json:"host_name"`
	IPAddress string `json:"ip_address"`
	Family    string `json:"family"`
	Found     bool
}

type vmwareVM struct {
	VM         string `json:"vm"`
	Name       string `json:"name"`
	PowerState string `json:"power_state"`
	Tags       map[string]string
	Detail     vmwareDetail
}

type vmwareVms struct {
	Value []vmwareVM `json:"value"`
}

type vcenterObjects struct {
	Name       string `json:"name"`
	Datacenter string `json:"datacenter"`
	Cluster    string `json:"cluster"`
}

type cloudData struct {
	Name     string
	VMID     string
	Tags     map[string]string
	Location string

	Interfaces []netInterface
	State      string
}

// NetInterface -
type netInterface struct {
	PrivateName string
	PrivateIP   []string
	PublicName  string
	PublicIP    string
	Primary     bool
	PublicDNS   string
	PrivateDNS  string
}
