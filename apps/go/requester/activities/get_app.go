package activities

import (
	"context"

	"packages/pocket_shannon"
	shannon_types "packages/pocket_shannon/types"

	"go.temporal.io/sdk/temporal"
)

type GetAppParams struct {
	Address string `json:"address"`
	Service string `json:"service"`
}

var GetAppName = "get_app"

func (aCtx *Ctx) GetApp(_ context.Context, params GetAppParams) (bool, error) {

	found := false
	for appAddress, _ := range aCtx.App.PocketApps {
		if appAddress == params.Address {
			found = true
			break
		}
	}
	if !found {
		return found, temporal.NewNonRetryableApplicationError("application not found in available Apps list", "ApplicationNotFound", nil)
	}

	ctxNode := context.Background()
	onchainApp, err := aCtx.App.PocketFullNode.GetApp(ctxNode, params.Address)
	if err != nil {
		return found, temporal.NewNonRetryableApplicationError("Error getting on-chain data", "ApplicationNotFound", nil)
	}
	if onchainApp == nil {
		return found, temporal.NewNonRetryableApplicationError("Cannot find App on-chain data", "ApplicationNotFound", nil)
	}
	// Check if the app is staked for the requested service
	if !pocket_shannon.AppIsStakedForService(shannon_types.ServiceID(params.Service), onchainApp) {
		return found, temporal.NewNonRetryableApplicationError("App not staked for service", "ApplicationNotStaked", nil)
	}

	return found, nil
}
