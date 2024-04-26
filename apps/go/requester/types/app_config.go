package types

import (
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"
	"packages/mongodb"
	"packages/pocket_rpc"
)

type App struct {
	Logger         *zerolog.Logger
	Config         *Config
	TemporalClient client.Client
	PocketRpc      pocket_rpc.Rpc
	Mongodb        mongodb.MongoDb
}
