package activities

import (
	"context"

	"go.temporal.io/sdk/temporal"
)

type GetHeightResults struct {
	Height int64 `json:"height"`
}

var GetHeightName = "get_height"

func (aCtx *Ctx) GetHeight(_ context.Context) (int64, error) {
	currHeight, err := aCtx.App.PocketFullNode.GetLatestBlockHeight()
	if err != nil || currHeight <= 0 {
		return currHeight, temporal.NewApplicationError("unable to get height", "GetHeight", err)
	}
	return currHeight, nil
}
