package types

import (
	poktGoSigner "github.com/pokt-foundation/pocket-go/signer"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"
	"packages/mongodb"
	"packages/pocket_rpc"
)

type App struct {
	Logger          *zerolog.Logger
	Config          *Config
	SignerByAddress *xsync.MapOf[string, *poktGoSigner.Signer]
	TemporalClient  client.Client
	PocketRpc       pocket_rpc.Rpc
	Mongodb         mongodb.MongoDb
}
