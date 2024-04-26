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
	height, err := aCtx.App.PocketRpc.GetHeight()
	if err != nil || height <= 0 {
		return height, temporal.NewApplicationError("unable to get height", "GetHeight", err)
	}
	return height, nil
}
