package main

import (
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"io"
	"os"
	"packages/logger"
	"packages/mongodb"
	"packages/pocket_rpc"
	"path/filepath"
	"requester/activities"
	"requester/types"
	"requester/workflows"
	"time"
)

func Initialize() *types.App {
	// get App config
	cfg := LoadConfigFile()
	// initialize logger
	l := InitLogger(cfg)

	ac := &types.App{
		Logger: l,
		Config: cfg,
	}

	mongodb.Initialize(cfg.MongodbUri, l)

	clientPoolOpts := pocket_rpc.ClientPoolOptions{
		MaxRetries: cfg.Rpc.Retries,
		ReqPerSec:  cfg.Rpc.ReqPerSec,
		MinBackoff: time.Duration(cfg.Rpc.MinBackoff),
		MaxBackoff: time.Duration(cfg.Rpc.MaxBackoff),
	}
	clientPool := pocket_rpc.NewClientPool(cfg.Rpc.Urls, &clientPoolOpts, l)
	ac.PocketRpc = pocket_rpc.NewPocketRpc(clientPool)

	// set this to workflows and activities to avoid use of context.Context
	workflows.SetAppConfig(ac)
	activities.SetAppConfig(ac)

	return ac
}

// InitLogger - initialize logger
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

	l := ctx.Str("App", "requester").Logger()

	zerolog.TimestampFieldName = "t"
	zerolog.MessageFieldName = "msg"
	zerolog.LevelFieldName = "lvl"

	return &l
}

// LoadConfigFile - read file from default path $HOME/tester/config.json
func LoadConfigFile() *types.Config {
	c := types.Config{}
	configPathEnv := os.Getenv("CONFIG_PATH")
	configPathDefault := filepath.Join(os.ExpandEnv("$HOME"), "tester", "config.json")
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
	jsonFile, err = os.OpenFile(configFilePath, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Fatal().Err(err).Str("Path", configFilePath).Msg("cannot open config json file")
	}

	defer func(jsonFile *os.File) {
		err := jsonFile.Close()
		if err != nil {
			log.Error().Err(err).Msg("error closing json file")
			return
		}
	}(jsonFile)

	b, err := io.ReadAll(jsonFile)
	if err != nil {
		log.Fatal().Err(err).Str("Path", configFilePath).Msg("cannot read config file")
	}
	err = json.Unmarshal(b, &c)
	if err != nil {
		log.Fatal().Err(err).Str("Path", configFilePath).Msg("cannot read config file into json")
	}

	return &c
}

func main() {
	// Initialize application things like logger/configs/etc
	ac := Initialize()
	// logger with tagged as worker
	// Initialize ac Temporal Client
	// Specify the Namespace in the Client options
	clientOptions := client.Options{
		HostPort:  fmt.Sprintf("%s:%d", ac.Config.Temporal.Host, ac.Config.Temporal.Port),
		Namespace: ac.Config.Temporal.Namespace,
		Logger:    logger.NewZerologAdapter(*ac.Logger),
	}
	temporalClient, err := client.Dial(clientOptions)
	if err != nil {
		ac.Logger.Fatal().Err(err).Msg("unable to create ac Temporal Client")
	}
	defer temporalClient.Close()

	// Create ac new Worker
	w := worker.New(temporalClient, ac.Config.Temporal.TaskQueue, worker.Options{
		// turn on replay logs only when debug level is on
		EnableLoggingInReplay: ac.Logger.GetLevel() == zerolog.DebugLevel,
	})

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
