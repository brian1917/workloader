package gcplabel

type GcpCLIResponse struct {
	Name              string             `json:"name"`
	Id                string             `json:"id"`
	Labels            map[string]string  `json:"labels"`
	NetworkInterfaces []NetworkInterface `json:"networkInterfaces"`
}

type NetworkInterface struct {
	Name      string `json:"name"`
	NetworkIP string `json:"networkIP"`
}

type Tags map[string]string
