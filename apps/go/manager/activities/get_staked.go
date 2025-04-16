package activities

import (
	"context"
	"manager/types"
	"packages/pocket_shannon"
)

var GetStakedName = "get_staked"

func (aCtx *Ctx) GetStaked(ctx context.Context, params types.GetStakedParams) (*types.GetStakedResults, error) {

	l := aCtx.App.Logger
	l.Debug().Msg("Collecting staked suppliers from network.")

	result := types.GetStakedResults{}

	// Get all suppliers in given chain
	l.Debug().Str("service", params.Service).Msg("Querying service...")

	suppliersPerService, err := pocket_shannon.SupliersInSession(aCtx.App.PocketFullNode, aCtx.App.PocketApps, aCtx.App.PocketServices)
	if err != nil {
		l.Error().Msg("Could not retrieve suppliers in session.")
		return nil, err
	}

	for service, suppliers := range suppliersPerService {
		for _, supplier := range suppliers {
			this_supplier := types.SupplierData{
				Address: string(supplier),
				Service: service,
			}
			result.Suppliers = append(result.Suppliers, this_supplier)
		}
	}

	if len(result.Suppliers) == 0 {
		l.Warn().Msg("No suppliers were found on any of the given services")
	} else {
		l.Debug().Int("suppliers_staked", len(result.Suppliers)).Msg("Successfully pulled staked supplier-services.")
	}

	// Get latest block
	currHeight, err := aCtx.App.PocketFullNode.GetLatestBlockHeight()
	if err != nil {
		l.Error().Str("service", params.Service).Msg("Could not retrieve latest block height.")
		return nil, err
	}
	// Get blocks per session
	// TODO : Add SDK support for this, in the meantime it is a parameter
	blocksPerSession := aCtx.App.PocketBlocksPerSession

	// Assign
	result.Block.BlocksPerSession = blocksPerSession
	result.Block.Height = currHeight

	return &result, nil
}
