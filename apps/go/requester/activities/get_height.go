package activities

import (
	"context"

	"go.temporal.io/sdk/temporal"
)

type GetHeightResults struct {
	Height int64 `json:"height"`
	// BlocksPerSession is the chain's session length. It rides along with the
	// height because both are chain state, and reading chain state is what an
	// activity is for — a workflow that asked the client directly would not
	// replay deterministically.
	BlocksPerSession int64 `json:"blocks_per_session"`
}

var GetHeightName = "get_height"

func (aCtx *Ctx) GetHeight(_ context.Context) (GetHeightResults, error) {
	currHeight, err := aCtx.App.PocketClient.GetLatestBlockHeight()
	if err != nil || currHeight <= 0 {
		return GetHeightResults{Height: currHeight}, temporal.NewApplicationError("unable to get height", "GetHeight", err)
	}
	return GetHeightResults{
		Height:           currHeight,
		BlocksPerSession: aCtx.App.PocketClient.BlocksPerSession(),
	}, nil
}
