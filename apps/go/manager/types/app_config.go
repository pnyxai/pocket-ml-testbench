package types

import (
	"packages/mongodb"
	"packages/pocket"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"
)

type App struct {
	Logger            *zerolog.Logger
	Config            *Config
	Mongodb           mongodb.MongoDb
	PocketClient      *pocket.Client
	TemporalClient    client.Client
	ExternalSuppliers []string
}
