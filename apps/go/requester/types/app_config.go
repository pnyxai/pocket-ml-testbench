package types

import (
	"net/http"
	"packages/mongodb"

	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"

	"packages/pocket_shannon"
)

type App struct {
	Logger                 *zerolog.Logger
	Config                 *Config
	TemporalClient         client.Client
	PocketFullNode         *pocket_shannon.LazyFullNode
	PocketApps             map[string]string
	PocketBlocksPerSession int64
	Mongodb                mongodb.MongoDb
	ExternalSuppliers      map[string]ExternalSupplierData
	ExternalHttpClient     *http.Client
}
