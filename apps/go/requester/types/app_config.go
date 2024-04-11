package types

import (
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/mongo"
	"packages/pocket_rpc"
)

type App struct {
	Logger    *zerolog.Logger
	Config    *Config
	Mongodb   *mongo.Client
	PocketRpc pocket_rpc.Rpc
}
