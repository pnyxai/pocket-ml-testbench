package activities

import (
	"context"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"go.temporal.io/sdk/temporal"
)

var GetBlockName = "get_block"

func (aCtx *Ctx) GetBlock(_ context.Context, height int64) (*poktGoSdk.GetBlockOutput, error) {
	block, err := aCtx.App.PocketRpc.GetBlock(height)
	if err != nil {
		return nil, temporal.NewApplicationError("unable to get block", "GetBlock", err)
	}
	return block, nil
}
