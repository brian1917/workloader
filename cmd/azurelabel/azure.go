package azurelabel

type AzureVirtualMachine struct {
	VirtualMachine AzureVM `json:"virtualMachine"`
}

type AzureVM struct {
	OsProfile     *AzureOsProfile `json:"osProfile"`
	Tags          AzureTags       `json:"tags"`
	Network       *AzureNetwork   `json:"network"`
	Name          string          `json:"name"`
	InterfaceList string
}

type AzureOsProfile struct {
	ComputerName string `json:"computerName"`
}

type AzureTags map[string]string

type AzureNetwork struct {
	PrivateIPAddresses []string                 `json:"privateIpAddresses"`
	PublicIPAddresses  []AzurePublicIPAddresses `json:"publicIpAddresses"`
}

type AzurePublicIPAddresses struct {
	ID                 string `json:"id"`
	IPAddress          string `json:"ipAddress"`
	IPAllocationMethod string `json:"ipAllocationMethod"`
	Name               string `json:"name"`
	ResourceGroup      string `json:"resourceGroup"`
	Zone               string `json:"zone"`
}
