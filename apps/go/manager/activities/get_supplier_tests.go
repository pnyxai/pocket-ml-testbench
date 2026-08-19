package activities

import (
	"context"
	"manager/records"
	"manager/types"
)

var GetSupplierTestsName = "get_supplier_tests"

func (aCtx *Ctx) GetSupplierTests(ctx context.Context, params types.GetSupplierTestsParams) (*types.GetSupplierTestsResults, error) {
	l := aCtx.App.Logger
	l.Debug().
		Str("address", params.Supplier.Address).
		Str("service", params.Supplier.Service).
		Msg("Retrieving supplier tests from DB.")

	// Retrieve this supplier entry to get its ObjectID
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
			Msg("Supplier not found in DB, returning empty tests.")
		return &types.GetSupplierTestsResults{Tests: []types.TestsData{}}, nil
	}

	tests, err := records.GetAllTasksForSupplier(thisSupplierData.ID, aCtx.App.Mongodb, l)
	if err != nil {
		l.Error().
			Err(err).
			Str("address", params.Supplier.Address).
			Str("service", params.Supplier.Service).
			Msg("Failed to retrieve tasks for supplier.")
		return nil, err
	}

	return &types.GetSupplierTestsResults{Tests: tests}, nil
}
