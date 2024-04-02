package types

import (
	"encoding/json"
)

type TemporalConfig struct {
	Host      string `json:"host"`
	Port      uint   `json:"port"`
	Namespace string `json:"namespace"`
	TaskQueue string `json:"task_queue"`
}

type Config struct {
	PostgresUri string          `json:"postgres_uri"`
	Apps        []string        `json:"apps"`
	RpcUrl      []string        `json:"rpc_url"`
	LogLevel    string          `json:"log_level"`
	Temporal    *TemporalConfig `json:"temporal"`
}

// UnmarshalJSON implement the Unmarshaler interface on Config
func (c *Config) UnmarshalJSON(b []byte) error {
	// We create an alias for the Config type to avoid recursive calls to the UnmarshalJSON method
	type Alias Config
	defaultValues := &Alias{
		PostgresUri: DefaultPostgresUri,
		Apps:        []string{},
		RpcUrl:      []string{},
		LogLevel:    DefaultLogLevel,
		Temporal: &TemporalConfig{
			Host:      DefaultTemporalHost,
			Port:      DefaultTemporalPort,
			Namespace: DefaultTemporalNamespace,
			TaskQueue: DefaultTemporalTaskQueue,
		},
	}

	if err := json.Unmarshal(b, &defaultValues); err != nil {
		return err
	}

	// If the incoming JSON has values, it will override the defaults. If not, the default values will be used.
	*c = Config(*defaultValues)

	return nil
}
