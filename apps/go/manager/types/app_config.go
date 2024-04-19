package types

import (
	"packages/mongodb"
	"packages/pocket_rpc"

	"github.com/rs/zerolog"
)

type App struct {
	Logger    *zerolog.Logger
	Config    *Config
	Mongodb   *mongodb.MongoDb
	PocketRpc pocket_rpc.Rpc
}
