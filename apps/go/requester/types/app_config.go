package types

import (
	"github.com/rs/zerolog"
	"packages/mongodb"
	"packages/pocket_rpc"
)

type App struct {
	Logger    *zerolog.Logger
	Config    *Config
	PocketRpc pocket_rpc.Rpc
	Mongodb   *mongodb.MongoDb
}
