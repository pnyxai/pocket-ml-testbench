package x

import (
	"context"
	"encoding/json"
	"fmt"
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

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Set application name
var RequesterAppName = "requester"

func ensureTemporalNamespaceExists(opts *client.Options, l *zerolog.Logger) {
	grpcClient, err := grpc.Dial(opts.HostPort, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock()) //Replace with your Temporal Server host and port
	if err != nil {
		l.Fatal().Err(err).Msg("unable to create a Temporal GRPC Client")
	}
	defer func(grpcClient *grpc.ClientConn) {
		e := grpcClient.Close()
		if e != nil {
			l.Error().Err(e).Msg("unable to close a Temporal GRPC Client")
		}
	}(grpcClient)

	// Make register namespace request
	namespaceRegistry := workflowservice.NewWorkflowServiceClient(grpcClient)
	namespaceRequest := &workflowservice.RegisterNamespaceRequest{
		Namespace:                        opts.Namespace,
		WorkflowExecutionRetentionPeriod: &durationpb.Duration{Seconds: int64(1 * 24 * 60 * 60)},
	}

	// Try to register namespace
	_, err = namespaceRegistry.RegisterNamespace(context.Background(), namespaceRequest)
	if err != nil {
		grpcStatus := status.Convert(err)

		// If already exist error, ignore, else return error
		if grpcStatus.Code() != codes.AlreadyExists {
			l.Fatal().Err(err).Msg("Temporal Namespace registration failed")
		}
		l.Info().Str("Namespace", opts.Namespace).Msg("Namespace already exists")
	} else {
		l.Info().Str("Namespace", opts.Namespace).Msg("Namespace created successfully")
	}
}

func Initialize() *types.App {
	// get App config
	cfg := LoadConfigFile()
	// initialize logger
	l := InitLogger(cfg)
	// initialize mongodb
	m := mongodb.NewClient(cfg.MongodbUri, []string{
		types.TaskCollection,
		types.InstanceCollection,
		types.PromptsCollection,
		types.ResponseCollection,
	}, l)

	clientPoolOpts := pocket_rpc.ClientPoolOptions{
		MaxRetries: cfg.Rpc.Retries,
		ReqPerSec:  cfg.Rpc.ReqPerSec,
		MinBackoff: time.Duration(cfg.Rpc.MinBackoff),
		MaxBackoff: time.Duration(cfg.Rpc.MaxBackoff),
	}
	clientPool := pocket_rpc.NewClientPool(cfg.Rpc.Urls, &clientPoolOpts, l)
	pocketRpc := pocket_rpc.NewPocketRpc(clientPool)

	temporalClientOptions := client.Options{
		HostPort:  fmt.Sprintf("%s:%d", cfg.Temporal.Host, cfg.Temporal.Port),
		Namespace: cfg.Temporal.Namespace,
		Logger:    logger.NewZerologAdapter(*l),
	}
	ensureTemporalNamespaceExists(&temporalClientOptions, l)
	temporalClient, err := client.Dial(temporalClientOptions)
	if err != nil {
		l.Fatal().Err(err).Msg("unable to create ac Temporal Client")
	}
	l.Info().
		Str("Namespace", temporalClientOptions.Namespace).
		Str("TaskQueue", cfg.Temporal.TaskQueue).
		Msg("Successfully connected to Temporal Server")

	ac := &types.App{
		Logger:         l,
		Config:         cfg,
		PocketRpc:      pocketRpc,
		Mongodb:        m,
		TemporalClient: temporalClient,
	}

	// generate app accounts
	ac.GenerateAppAccounts()

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

	l := ctx.Str("App", RequesterAppName).Logger()

	zerolog.TimestampFieldName = "t"
	zerolog.MessageFieldName = "msg"
	zerolog.LevelFieldName = "lvl"

	return &l
}

// LoadConfigFile - read file from default path $HOME/requester/config.json
func LoadConfigFile() *types.Config {
	c := types.Config{}
	configPathEnv := os.Getenv("CONFIG_PATH")
	configPathDefault := filepath.Join(os.ExpandEnv("$HOME"), RequesterAppName, "config.json")
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
