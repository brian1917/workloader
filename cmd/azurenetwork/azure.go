package azurenetwork

type AzureNetwork struct {
	AddressSpace         *AddressSpace `json:"addressSpace"`
	EnableDdosProtection bool          `json:"enableDdosProtection"`
	Etag                 string        `json:"etag"`
	ID                   string        `json:"id"`
	Location             string        `json:"location"`
	Name                 string        `json:"name"`
	ProvisioningState    string        `json:"provisioningState"`
	ResourceGroup        string        `json:"resourceGroup"`
	ResourceGUID         string        `json:"resourceGuid"`
	Subnets              *[]Subnets    `json:"subnets"`
	Type                 string        `json:"type"`
}

type AddressSpace struct {
	AddressPrefixes []string `json:"addressPrefixes"`
}

type Subnets struct {
	AddressPrefix                     string              `json:"addressPrefix"`
	Etag                              string              `json:"etag"`
	ID                                string              `json:"id"`
	IPConfigurations                  *[]IPConfigurations `json:"ipConfigurations"`
	Name                              string              `json:"name"`
	PrivateEndpointNetworkPolicies    string              `json:"privateEndpointNetworkPolicies"`
	PrivateLinkServiceNetworkPolicies string              `json:"privateLinkServiceNetworkPolicies"`
	ProvisioningState                 string              `json:"provisioningState"`
	ResourceGroup                     string              `json:"resourceGroup"`
	Type                              string              `json:"type"`
}

type IPConfigurations struct {
	ID            string `json:"id"`
	ResourceGroup string `json:"resourceGroup"`
}
