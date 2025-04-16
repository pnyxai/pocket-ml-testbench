package types

import (
	"packages/mongodb"
	"packages/pocket_shannon"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"
)

type App struct {
	Logger                 *zerolog.Logger
	Config                 *Config
	Mongodb                mongodb.MongoDb
	PocketFullNode         *pocket_shannon.LazyFullNode
	PocketApps             []string
	PocketServices         []string
	PocketBlocksPerSession int64
	TemporalClient         client.Client
}
