package types

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
	Tasks      []string        `json:"tasks"`
	Services   []string        `json:"services"`
	Rpc        *RPCConfig      `json:"rpc"`
	LogLevel   string          `json:"log_level"`
	Temporal   *TemporalConfig `json:"temporal"`
}
