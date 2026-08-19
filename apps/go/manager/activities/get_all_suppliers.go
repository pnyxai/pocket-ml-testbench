package activities

import (
	"context"
	"manager/records"
	"manager/types"
)

var GetAllSuppliersName = "get_all_suppliers"

func (aCtx *Ctx) GetAllSuppliers(ctx context.Context) (*types.GetAllSuppliersResults, error) {
	l := aCtx.App.Logger
	l.Debug().Msg("Getting all suppliers from DB.")

	suppliers, err := records.GetAllSuppliers(aCtx.App.Mongodb, l)
	if err != nil {
		return nil, err
	}

	return &types.GetAllSuppliersResults{Suppliers: suppliers}, nil
}
