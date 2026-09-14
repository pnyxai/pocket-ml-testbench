package activities

import (
	"context"

	"packages/pocket"

	"go.temporal.io/sdk/temporal"
)

type GetAppParams struct {
	Address string `json:"address"`
	Service string `json:"service"`
}

var GetAppName = "get_app"

func (aCtx *Ctx) GetApp(_ context.Context, params GetAppParams) (bool, error) {

	// Every configured app was resolved against the chain when the client was
	// built — it exists, its key matches its address, and the single service it
	// is staked for is known. So both checks below are local lookups.
	if _, found := aCtx.App.PocketClient.ServiceForApp(params.Address); !found {
		return false, temporal.NewNonRetryableApplicationError("application not found in available Apps list", "ApplicationNotFound", nil)
	}

	if !aCtx.App.PocketClient.AppIsStakedForService(params.Address, pocket.ServiceID(params.Service)) {
		return true, temporal.NewNonRetryableApplicationError("App not staked for service", "ApplicationNotStaked", nil)
	}

	return true, nil
}
