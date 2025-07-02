package types

const ExternalSupplierIdentifier = "external_"
const ExternalServiceName = "external"

type ExternalSupplierData struct {
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers"`

	// TODO : Add support for these, specifically ModelName blocks any provider
	// 		  that is not a pocket gateway

	// ApiType       string            `json:"api_type"`
	// ModelName string `json:"model_name"`
	// ContextLength int64             `json:"context_length"`
	// Timeout       int64             `json:"timeout"`
}
