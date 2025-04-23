package activities

import (
	"context"

	"packages/pocket_shannon"

	sessiontypes "github.com/pokt-network/poktroll/x/session/types"

	"go.temporal.io/sdk/temporal"
)

var GetEndpointsName = "get_endpoint"

func (aCtx *Ctx) GetEndpoints(_ context.Context, appSession sessiontypes.Session) (map[string]pocket_shannon.Endpoint, error) {

	suppliers, err := pocket_shannon.EndpointsFromSession(appSession)

	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError("Could not get endpoints data", "EndpointsNotFound", err)
	}

	return suppliers, nil
}
