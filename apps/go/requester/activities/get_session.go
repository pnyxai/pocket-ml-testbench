package activities

import (
	"context"

	shannon_types "packages/pocket_shannon/types"

	sessiontypes "github.com/pokt-network/poktroll/x/session/types"

	"go.temporal.io/sdk/temporal"
)

type GetSessionParams struct {
	Address string `json:"address"`
	Service string `json:"service"`
}

var GetSessionName = "get_session"

func (aCtx *Ctx) GetSession(_ context.Context, params GetSessionParams) (*sessiontypes.Session, error) {

	appSession, err := aCtx.App.PocketFullNode.GetSession(shannon_types.ServiceID(params.Service), params.Address)
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError("Could not get session data", "SessionNotFound", err)
	}

	return &appSession, nil
}
