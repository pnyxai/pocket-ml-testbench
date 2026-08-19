package activities

import (
	"context"
	"manager/records"
	"manager/types"
	"time"
)

var UpdateSupplierBuffersName = "update_supplier_buffers"

func (aCtx *Ctx) UpdateSupplierBuffers(ctx context.Context, params types.UpdateSupplierBuffersParams) (*types.UpdateSupplierBuffersResults, error) {
	l := aCtx.App.Logger
	l.Debug().
		Str("address", params.Supplier.Address).
		Str("service", params.Supplier.Service).
		Msg("Updating supplier buffers.")

	// Get current height and time
	currHeight, err := aCtx.App.PocketFullNode.GetLatestBlockHeight()
	if err != nil {
		l.Error().
			Str("supplier", params.Supplier.Address).
			Str("service", params.Supplier.Service).
			Msg("Could not retrieve latest block height.")
		return nil, err
	}
	currTime := time.Now()

	// Retrieve this supplier entry
	var thisSupplierData records.SupplierRecord
	found, err := thisSupplierData.FindAndLoadSupplier(params.Supplier, aCtx.App.Mongodb, l)
	if err != nil {
		l.Error().
			Err(err).
			Str("address", params.Supplier.Address).
			Str("service", params.Supplier.Service).
			Msg("Failed to load supplier data.")
		return nil, err
	}

	if !found {
		l.Warn().
			Str("address", params.Supplier.Address).
			Str("service", params.Supplier.Service).
			Msg("Supplier not found in DB, skipping buffer update.")
		return &types.UpdateSupplierBuffersResults{Success: false}, nil
	}

	_, err = UpdateSupplierData(&thisSupplierData, true, params.Supplier, params.Tests, aCtx.App.Config.Frameworks, aCtx.App.Mongodb, l, currHeight, currTime)
	if err != nil {
		l.Error().
			Err(err).
			Str("address", params.Supplier.Address).
			Str("service", params.Supplier.Service).
			Msg("Failed to update supplier buffers.")
		return &types.UpdateSupplierBuffersResults{Success: false}, err
	}

	return &types.UpdateSupplierBuffersResults{Success: true}, nil
}
