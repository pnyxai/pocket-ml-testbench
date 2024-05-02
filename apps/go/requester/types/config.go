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

type EvaluatorConfig struct {
	WorkflowName string `json:"workflow_name"`
	TaskQueue    string `json:"task_queue"`
}

type RPCConfig struct {
	Urls             []string `json:"urls"`
	Retries          int      `json:"retries"`
	MinBackoff       int      `json:"min_backoff"`
	MaxBackoff       int      `json:"max_backoff"`
	ReqPerSec        int      `json:"req_per_sec"`
	SessionTolerance int64    `json:"session_tolerance"`
}

type Config struct {
	MongodbUri string           `json:"mongodb_uri"`
	Apps       []string         `json:"apps"`
	Rpc        *RPCConfig       `json:"rpc"`
	LogLevel   string           `json:"log_level"`
	Temporal   *TemporalConfig  `json:"temporal"`
	Evaluator  *EvaluatorConfig `json:"evaluator"`
}

// UnmarshalJSON implement the Unmarshaler interface on Config
func (c *Config) UnmarshalJSON(b []byte) error {
	// We create an alias for the Config type to avoid recursive calls to the UnmarshalJSON method
	type Alias Config
	defaultValues := &Alias{
		MongodbUri: DefaultMongodbUri,
		Apps:       []string{},
		Rpc:        &DefaultRpc,
		LogLevel:   DefaultLogLevel,
		Temporal:   &DefaultTemporal,
		Evaluator:  &DefaultEvaluator,
	}

	if err := json.Unmarshal(b, &defaultValues); err != nil {
		return err
	}

	// If the incoming JSON has values, it will override the defaults. If not, the default values will be used.
	*c = Config(*defaultValues)

	return nil
}
