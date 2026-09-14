package activities

import (
	"context"
	"manager/types"
)

var GetBlockDataName = "get_block_data"

// GetBlockData reports the current chain head and session length.
//
// The external-service branch of the supplier manager needs both but has no
// session to read them from, so it cannot go through GetStaked. Reading them
// through an activity rather than straight off the client is what keeps that
// branch of the workflow replayable.
func (aCtx *Ctx) GetBlockData(_ context.Context) (*types.BlockData, error) {
	currHeight, err := aCtx.App.PocketClient.GetLatestBlockHeight()
	if err != nil {
		aCtx.App.Logger.Error().Err(err).Msg("Could not retrieve latest block height.")
		return nil, err
	}

	return &types.BlockData{
		Height:           currHeight,
		BlocksPerSession: aCtx.App.PocketClient.BlocksPerSession(),
	}, nil
}
