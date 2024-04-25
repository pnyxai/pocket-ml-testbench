package activities

import (
	"context"
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"go.temporal.io/sdk/temporal"
	"packages/pocket_rpc"
)

type GetSessionParams struct {
	App     string `json:"app"`
	Service string `json:"service"`
}

var GetSessionName = "get_session"

func (aCtx *Ctx) GetSession(_ context.Context, params GetSessionParams) (*poktGoSdk.DispatchOutput, error) {
	if e := pocket_rpc.PubKeyVerification(params.App); e != nil {
		return nil, temporal.NewNonRetryableApplicationError("bad params", "BadParams", e)
	}

	if e := pocket_rpc.ServiceIdentifierVerification(params.Service); e != nil {
		return nil, temporal.NewNonRetryableApplicationError("bad params", "BadParams", e)
	}

	result, err := aCtx.App.PocketRpc.GetSession(params.App, params.Service)
	if err != nil {
		if errors.Is(err, pocket_rpc.ErrBadRequestParams) {
			return nil, temporal.NewNonRetryableApplicationError("bad params", "BadParams", err)
		}
		return nil, temporal.NewApplicationError("unable to get session", "GetSession", err)
	}

	return result, nil
}
