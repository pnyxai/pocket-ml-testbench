package activities

import (
	"context"

	"packages/pocket"

	"go.temporal.io/sdk/temporal"
)

type GetSessionParams struct {
	Address string `json:"address"`
	Service string `json:"service"`
}

var GetSessionName = "get_session"

func (aCtx *Ctx) GetSession(ctx context.Context, params GetSessionParams) (*pocket.SessionInfo, error) {

	appSession, err := aCtx.App.PocketClient.GetSession(ctx, params.Address, pocket.ServiceID(params.Service))
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError("Could not get session data", "SessionNotFound", err)
	}

	return appSession, nil
}
