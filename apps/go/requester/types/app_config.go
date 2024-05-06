package types

import (
	"fmt"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/client"
	"packages/mongodb"
	"packages/pocket_rpc"
)

type App struct {
	Logger         *zerolog.Logger
	Config         *Config
	AppAccounts    *xsync.MapOf[string, *AppAccount]
	TemporalClient client.Client
	PocketRpc      pocket_rpc.Rpc
	Mongodb        mongodb.MongoDb
}

func (a *App) GenerateAppAccounts() {
	a.AppAccounts = xsync.NewMapOf[string, *AppAccount]()
	for i, pk := range a.Config.Apps {
		appAccount, appErr := NewAppAccount(pk)

		if appErr != nil {
			a.Logger.Fatal().Err(appErr).Msg(fmt.Sprintf("failed to load app private key into an app account %d", i))
		}

		a.AppAccounts.Store(appAccount.Signer.GetAddress(), appAccount)
	}
}
