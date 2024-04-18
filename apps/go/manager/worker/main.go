package main

import (
	"encoding/json"
	"fmt"
	"io"
	"manager/activities"
	"os"
	"packages/logger"
	"path/filepath"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"manager/types"
	"manager/workflows"
	"packages/mongodb"
	"packages/pocket_rpc"
	"time"
)

// Set application name
var ManagerAppName = "manager"

// Initialize - Initialize the application elements
func Initialize() *types.App {
	// get App config
	cfg := LoadConfigFile()
	// initialize logger
	l := InitLogger(cfg)
	// Initialize connection to mongo db
	collectionNames := []string{"nodes"}
	mongoCon := mongodb.Initialize(cfg.MongodbUri, collectionNames, l)
	// Initialize connection to RPC
	clientPoolOpts := pocket_rpc.ClientPoolOptions{
		MaxRetries: cfg.Rpc.Retries,
		ReqPerSec:  cfg.Rpc.ReqPerSec,
		MinBackoff: time.Duration(cfg.Rpc.MinBackoff),
		MaxBackoff: time.Duration(cfg.Rpc.MaxBackoff),
	}
	clientPool := pocket_rpc.NewClientPool(cfg.Rpc.Urls, &clientPoolOpts, l)
	pocketRpc := pocket_rpc.NewPocketRpc(clientPool)

	// Create instance of App data
	ac := &types.App{
		Logger:    l,
		Config:    cfg,
		Mongodb:   mongoCon,
		PocketRpc: pocketRpc,
	}

	// SetAppConfig sets the provided app config to the Workflows global variable in the Ctx struct.
	// This avoids abusing of context.Context
	workflows.SetAppConfig(ac)
	activities.SetAppConfig(ac)

	// Return the newly loaded config
	return ac
}

// InitLogger - Initialize the ZeroLog logger
func InitLogger(config *types.Config) *zerolog.Logger {
	lvl := zerolog.InfoLevel

	if config.LogLevel != "" {
		if l, err := zerolog.ParseLevel(config.LogLevel); err != nil {
			log.Fatal().Err(err).Msg("unable to parse log_level value")
		} else {
			lvl = l
		}
	}

	log.Logger.Level(lvl)

	ctx := zerolog.New(
		zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		},
	).Level(lvl).With().Timestamp()

	if lvl >= zerolog.DebugLevel {
		ctx = ctx.Caller()
	}

	l := ctx.Str("app", ManagerAppName).Logger()

	zerolog.TimestampFieldName = "t"
	zerolog.MessageFieldName = "msg"
	zerolog.LevelFieldName = "lvl"

	return &l
}

// LoadConfigFile - read file from default path $HOME/tester/config.json
func LoadConfigFile() *types.Config {
	// Create new instance of config struct
	c := types.Config{}
	// Construct default path for config
	configPathDefault := filepath.Join(os.ExpandEnv("$HOME"), ManagerAppName, "config.json")
	// Set the configuration path to use
	configPathEnv := os.Getenv("CONFIG_PATH")
	if configPathEnv == "" {
		log.Warn().Str("Default", configPathDefault).Msg("Missing CONFIG_PATH. Using default")
		configPathEnv = configPathDefault
	}
	configFilePath, err := filepath.Abs(configPathEnv)
	if err != nil {
		log.Fatal().Str("Path", configFilePath).Msg("unable to resolve path")
	}

	// Check if the file is there and if it is a file
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		log.Fatal().Msg("no config file found...")
	}

	// Open the config file
	var jsonFile *os.File
	jsonFile, err = os.OpenFile(configFilePath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Fatal().Err(err).Str("Path", configFilePath).Msg("cannot open config json file")
	}
	// Defer closing to function end
	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {
			log.Error().Err(err).Msg("error closing json file")
			return
		}
	}(jsonFile)

	// Read config file content
	b, err := io.ReadAll(jsonFile)
	if err != nil {
		log.Fatal().Err(err).Str("Path", configFilePath).Msg("cannot read config file")
	}
	// Unmarshal content into json and save
	err = json.Unmarshal(b, &c)
	if err != nil {
		log.Fatal().Err(err).Str("Path", configFilePath).Msg("cannot read config file into json")
	}

	// Return JSON content
	return &c
}

func main() {

	// Initialize application elements like logger/configs/etc
	ac := Initialize()

	// Initialize Temporal Client
	// using the provided namespace and logger
	clientOptions := client.Options{
		HostPort:  fmt.Sprintf("%s:%d", ac.Config.Temporal.Host, ac.Config.Temporal.Port),
		Namespace: ac.Config.Temporal.Namespace,
		Logger:    logger.NewZerologAdapter(*ac.Logger),
	}
	// Connect to Temporal server
	temporalClient, err := client.Dial(clientOptions)
	if err != nil {
		ac.Logger.Fatal().Err(err).Msg("unable to create ac Temporal Client")
	}
	defer temporalClient.Close()

	// Create new Temporal worker
	w := worker.New(temporalClient, ac.Config.Temporal.TaskQueue, worker.Options{})

	// Register Workflows
	workflows.Workflows.Register(w)

	// Register Activities
	activities.Activities.Register(w)

	// Start the Worker Process
	err = w.Run(worker.InterruptCh())
	if err != nil {
		ac.Logger.Fatal().Err(err).Msg("unable to start the Worker Process")
	}
}
