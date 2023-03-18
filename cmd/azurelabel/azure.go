package azurelabel

type AzureVM struct {
	OsProfile *OsProfile `json:"osProfile"`
	Tags      Tags       `json:"tags"`
}

type OsProfile struct {
	ComputerName string `json:"computerName"`
}

type Tags map[string]string
