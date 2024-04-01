package app

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"os"
	"path/filepath"
)

var (
	DefaultPostgresUri       = "postgres://localhost:5432"
	DefaultLogLevel          = "info"
	DefaultTemporalHost      = "localhost"
	DefaultTemporalPort      = uint(7233)
	DefaultTemporalNamespace = "default"
	DefaultTemporalTaskQueue = "relay-tester"
)

type Temporal struct {
	Host      string `json:"host"`
	Port      uint   `json:"port"`
	Namespace string `json:"namespace"`
	TaskQueue string `json:"task_queue"`
}

type Config struct {
	PostgresUri string    `json:"postgres_uri"`
	Apps        []string  `json:"apps"`
	RpcUrl      []string  `json:"rpc_url"`
	LogLevel    string    `json:"log_level"`
	Temporal    *Temporal `json:"temporal"`
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
		Temporal: &Temporal{
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

// LoadConfigFile - read file from default path $HOME/tester/config.json
func LoadConfigFile() *Config {
	c := Config{}
	configPathEnv := os.Getenv("CONFIG_PATH")
	configPathDefault := filepath.Join("$HOME", "tester", "config.json")
	if configPathEnv == "" {
		log.Warn().Str("Default", configPathDefault).Msg("Missing CONFIG_PATH. Using default")
		configPathEnv = configPathDefault
	}
	configFilePath, err := filepath.Abs(configPathEnv)

	if err != nil {
		log.Fatal().Str("Path", configFilePath).Msg("unable to resolve path")
	}

	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Fatal().Msg("no config file found...")
	}

	var jsonFile *os.File
	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {
			return
		}
	}(jsonFile)

	jsonFile, err = os.OpenFile(configFilePath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Fatal().Err(err).Str("Path", configFilePath).Msg("cannot open config json file")
	}
	b, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		log.Fatal().Err(err).Str("Path", configFilePath).Msg("cannot read config file")
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		log.Fatal().Err(err).Str("Path", configFilePath).Msg("cannot read config file into json")
	}

	return &c
}
