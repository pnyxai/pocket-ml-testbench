package types

const ExternalSupplierIdentifier = "external_"
const ExternalServiceName = "external"

type ExternalSupplierData struct {
	Endpoint            string            `json:"endpoint"`
	Headers             map[string]string `json:"headers"`
	ModelName           string            `json:"model"`
	ServiceTier         string            `json:"service_tier"`
	TemperatureOverride float32           `json:"temperature_override"`
	NoStop              bool              `json:"no_stop"`
	NoSeed              bool              `json:"no_seed"`
	CustomApiPath       string            `json:"custom_api_path"`

	// TODO : Add support for these

	// ApiType       string            `json:"api_type"`
	// ContextLength int64             `json:"context_length"`
	// Timeout       int64             `json:"timeout"`
}
