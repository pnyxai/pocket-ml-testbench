package activities

import (
	"context"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
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

func (aCtx *Ctx) GetBlock(ctx context.Context, params GetBlockParams) (*GetBlockResults, error) {
	result := GetBlockResults{}
	return &result, nil
}
