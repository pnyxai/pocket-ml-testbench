package types

import (
	"net/http"
	"packages/mongodb"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"

	"packages/pocket"
)

type App struct {
	Logger             *zerolog.Logger
	Config             *Config
	TemporalClient     client.Client
	PocketClient       *pocket.Client
	Mongodb            mongodb.MongoDb
	ExternalSuppliers  map[string]ExternalSupplierData
	ExternalHttpClient *http.Client
}
