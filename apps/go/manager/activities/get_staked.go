package activities

import (
	"context"
	"manager/types"
)

var GetStakedName = "get_staked"

func (aCtx *Ctx) GetStaked(ctx context.Context, params types.GetStakedParams) (*types.GetStakedResults, error) {

	l := aCtx.App.Logger
	l.Debug().Msg("Collecting staked suppliers from network.")

	result := types.GetStakedResults{}

	// Get all suppliers in given service
	appAddresses := aCtx.App.PocketClient.Apps()
	servicesNames := make([]string, 0)
	servicesNames = append(servicesNames, params.Service)
	l.Debug().Strs("service", servicesNames).Strs("Apps", appAddresses).Msg("Querying network...")
	suppliersPerService, err := aCtx.App.PocketClient.SuppliersInSession(ctx, appAddresses, servicesNames, l)
	if err != nil {
		l.Error().Msg("Could not retrieve suppliers in session.")
		return nil, err
	}
	l.Debug().Int("suppliersPerService", len(suppliersPerService)).Msg("Total Suppliers per service")

	for service, suppliers := range suppliersPerService {
		for _, supplier := range suppliers {
			this_supplier := types.SupplierData{
				Address: supplier,
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
	currHeight, err := aCtx.App.PocketClient.GetLatestBlockHeight()
	if err != nil {
		l.Error().Str("service", params.Service).Msg("Could not retrieve latest block height.")
		return nil, err
	}
	// Get blocks per session, as the chain reports it
	blocksPerSession := aCtx.App.PocketClient.BlocksPerSession()

	// Assign
	result.Block.BlocksPerSession = blocksPerSession
	result.Block.Height = currHeight

	return &result, nil
}
