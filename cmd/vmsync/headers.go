package vmsync

// vcenterTags - container for tags and their category, categoryId and pce labeltype once matched
type vcenterTags struct {
	LabelType  string
	CategoryID string `json:"category_id"`
	Category   string `json:"category"`
	Tag        string `json:"tag"`
}

// categoryDetail - used to get the Category Name which equates to LabelType
type categoryDetail struct {
	Name            string   `json:"name"`
	Cardinality     string   `json:"cardinality"`
	Description     string   `json:"description"`
	ID              string   `json:"id"`
	AssociableTypes []string `json:"associable_types"`
	UsedBy          []string `json:"used_by"`
}

// tagDetail - Struct used to get the Tag Name from the TagID.
type tagDetail struct {
	Name        string   `json:"name"`
	CategoryID  string   `json:"category_id"`
	Description string   `json:"description"`
	ID          string   `json:"id"`
	UsedBy      []string `json:"used_by"`
}

// vcenterVM - Struct used to gather all VM informatoin to be the basis of the wkld.import file
type vcenterVM struct {
	VMID         string `json:"vm"`
	Name         string `json:"name"`
	VCName       string
	PowerState   string `json:"power_state"`
	Tags         map[string]string
	Interfaces   [][]string
	IPs          map[string]bool
	VMInterfaces []Netinterfaces
}
type VMIdentity struct {
	Family   string `json:"family"`
	FullName struct {
		Args           []string `json:"args"`
		DefaultMessage string   `json:"default_message"`
		ID             string   `json:"id"`
		Localized      string   `json:"localized"`
	} `json:"full_name"`
	HostName  string `json:"host_name"`
	IPAddress string `json:"ip_address"`
	Name      string `json:"name"`
}

// vcenterObjects - Struct that is used for filtering VMs.
type vcenterObjects struct {
	Name       string `json:"name"`
	Datacenter string `json:"datacenter"`
	Cluster    string `json:"cluster"`
	Folder     string `json:"folder"`
}

// RequestObject for getting all tags for a set of VMs
type objects struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
type requestObject struct {
	ObjectId []objects `json:"object_ids"`
}

// ResponseObject for what comes back when requesting all tags for a set of VMs
type responseObject struct {
	TagIds   []string `json:"tag_ids"`
	ObjectId objects  `json:"object_id"`
}

// Netinterface struct for GetVMDetail Call.  This is the network data discovered by VMTools
type Netinterfaces struct {
	IP struct {
		IPAddresses []struct {
			IPAddress    string `json:"ip_address"`
			Origin       string `json:"origin"`
			PrefixLength int    `json:"prefix_length"`
			State        string `json:"state"`
		} `json:"ip_addresses"`
	} `json:"ip"`
	Nic        string `json:"nic"`
	MacAddress string `json:"mac_address"`
}

// VCenter getVersion API
type VCVersion struct {
	Build       string `json:"build"`
	InstallTime string `json:"install_time"`
	Product     string `json:"product"`
	Releasedate string `json:"releasedate"`
	Summary     string `json:"summary"`
	Type        string `json:"type"`
	Version     string `json:"version"`
}

// VCenter represents a VMware VCenter environment
// API  calls are methods on VCenter
type VCenter struct {
	VCenterURL         string
	User               string
	Secret             string
	DisableTLSChecking bool
	VCVersion          VCVersion
	KeyMap             map[string]string
	Categories         []string
	VCTags             map[string]vcenterTags
	VCVMs              map[string]vcenterVM
	VCVMSlice          []vcenterVM
	Header             map[string]string
}
