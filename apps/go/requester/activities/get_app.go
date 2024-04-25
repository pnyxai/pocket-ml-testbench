package activities

import (
	"context"
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"go.temporal.io/sdk/temporal"
	"packages/pocket_rpc"
)

type GetAppParams struct {
	Address string `json:"address"`
	Service string `json:"service"`
}

var GetAppName = "get_app"

func (aCtx *Ctx) GetApp(_ context.Context, params GetAppParams) (*poktGoSdk.App, error) {
	if e := pocket_rpc.AddressVerification(params.Address); e != nil {
		return nil, temporal.NewNonRetryableApplicationError("bad params", "BadParams", e)
	}

	app, err := aCtx.App.PocketRpc.GetApp(params.Address)
	if err != nil {
		if errors.Is(err, pocket_rpc.ErrBadRequestParams) {
			return nil, temporal.NewNonRetryableApplicationError("bad params", "BadParams", err)
		}
		return nil, temporal.NewApplicationError("unable to get app", "GetApp", err)
	}
	return app, nil
}
