package activities

import (
	"context"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"go.temporal.io/sdk/temporal"
)

type GetBlockParams struct {
	// Pass a 0 to get the latest
	Height int64 `json:"height"`
}

type GetBlockResults struct {
	Block  *poktGoSdk.GetBlockOutput `json:"block"`
	Params *poktGoSdk.AllParams      `json:"params"`
}

var GetBlockName = "get_block"

func (aCtx *Ctx) GetBlock(_ context.Context, params GetBlockParams) (*GetBlockResults, error) {
	block, err := aCtx.App.PocketRpc.GetBlock(params.Height)
	if err != nil {
		return nil, temporal.NewApplicationError("unable to get block", "GetBlock", err)
	}
	allParams, err := aCtx.App.PocketRpc.GetAllParams(params.Height)
	if err != nil {
		return nil, temporal.NewApplicationError("unable to get all params", "GetAllParams", err)
	}
	result := GetBlockResults{
		Block:  block,
		Params: allParams,
	}
	return &result, nil
}
