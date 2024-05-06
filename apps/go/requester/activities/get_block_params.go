package activities

import (
	"context"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"go.temporal.io/sdk/temporal"
)

var GetBlockParamsName = "get_block_params"

func (aCtx *Ctx) GetBlockParams(_ context.Context, height int64) (*poktGoSdk.AllParams, error) {
	allParams, err := aCtx.App.PocketRpc.GetAllParams(height)
	if err != nil {
		return nil, temporal.NewApplicationError("unable to get all params", "GetAllParams", err)
	}
	return allParams, nil
}
