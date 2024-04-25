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

type RPCConfig struct {
	Urls       []string `json:"urls"`
	Retries    int      `json:"retries"`
	MinBackoff int      `json:"min_backoff"`
	MaxBackoff int      `json:"max_backoff"`
	ReqPerSec  int      `json:"req_per_sec"`
}

type Config struct {
	MongodbUri string          `json:"mongodb_uri"`
	Apps       []string        `json:"apps"`
	Rpc        *RPCConfig      `json:"rpc"`
	LogLevel   string          `json:"log_level"`
	Temporal   *TemporalConfig `json:"temporal"`
}

// UnmarshalJSON implement the Unmarshaler interface on Config
func (c *Config) UnmarshalJSON(b []byte) error {
	// We create an alias for the Config type to avoid recursive calls to the UnmarshalJSON method
	type Alias Config
	defaultValues := &Alias{
		MongodbUri: DefaultMongodbUri,
		Apps:       []string{},
		Rpc: &RPCConfig{
			Urls:       []string{},
			Retries:    3,
			MinBackoff: 10,
			MaxBackoff: 60,
			ReqPerSec:  10,
		},
		LogLevel: DefaultLogLevel,
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
