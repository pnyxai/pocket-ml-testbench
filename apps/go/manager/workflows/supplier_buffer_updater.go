package workflows

import (
	"fmt"
	"time"

	"manager/activities"
	"manager/types"

	"go.temporal.io/sdk/workflow"
)

var SupplierBufferUpdaterName = "SupplierBufferUpdater"

// SupplierBufferUpdater - Is a workflow that loops over all existing suppliers
// in the database and updates their circular buffers (dropping stale samples)
// without triggering any new evaluation tasks.
func (wCtx *Ctx) SupplierBufferUpdater(ctx workflow.Context, params types.SupplierBufferUpdaterParams) (*types.SupplierBufferUpdaterResults, error) {

	l := wCtx.App.Logger
	l.Debug().Msg("Starting Supplier Buffer Updater Workflow.")

	// Create result
	result := types.SupplierBufferUpdaterResults{}

	// -------------------------------------------------------------------------
	// -------------------- Get all suppliers from DB --------------------------
	// -------------------------------------------------------------------------
	ctxTimeout := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute * 5,
	})

	var allSuppliers types.GetAllSuppliersResults
	err := workflow.ExecuteActivity(ctxTimeout, activities.GetAllSuppliersName).Get(ctx, &allSuppliers)
	if err != nil {
		return &result, err
	}

	l.Debug().Int("supplierCount", len(allSuppliers.Suppliers)).Msg("Loaded suppliers from DB.")

	// -------------------------------------------------------------------------
	// Phase 1: Discover existing tasks for each supplier
	// -------------------------------------------------------------------------
	ctxTimeout = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute,
	})

	selector := workflow.NewSelector(ctx)

	type supplierTestsChanItem struct {
		Supplier types.SupplierData
		Tests    []types.TestsData
	}

	supplierTestsChan := make(chan supplierTestsChanItem, len(allSuppliers.Suppliers))
	defer close(supplierTestsChan)

	for _, supplier := range allSuppliers.Suppliers {
		supplier := supplier // capture loop variable for closure
		res := types.GetSupplierTestsResults{}
		selector.AddFuture(
			workflow.ExecuteActivity(
				ctxTimeout,
				activities.GetSupplierTestsName,
				types.GetSupplierTestsParams{Supplier: supplier},
			),
			func(f workflow.Future) {
				err := f.Get(ctx, &res)
				if err != nil {
					return
				}
				supplierTestsChan <- supplierTestsChanItem{
					Supplier: supplier,
					Tests:    res.Tests,
				}
			},
		)
	}

	supplierTestsMap := make(map[string][]types.TestsData)
	for i := 0; i < len(allSuppliers.Suppliers); i++ {
		selector.Select(ctx)
		item := <-supplierTestsChan
		key := fmt.Sprintf("%s|%s", item.Supplier.Address, item.Supplier.Service)
		supplierTestsMap[key] = item.Tests
	}

	// -------------------------------------------------------------------------
	// Phase 2: Update buffers for each supplier using discovered tests
	// -------------------------------------------------------------------------
	ctxTimeout = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute,
	})

	selector = workflow.NewSelector(ctx)

	supplierUpdateResultsChan := make(chan types.UpdateSupplierBuffersResults, len(allSuppliers.Suppliers))
	defer close(supplierUpdateResultsChan)

	for _, supplier := range allSuppliers.Suppliers {
		supplier := supplier // capture loop variable for closure
		key := fmt.Sprintf("%s|%s", supplier.Address, supplier.Service)
		tests := supplierTestsMap[key]

		res := types.UpdateSupplierBuffersResults{}
		selector.AddFuture(
			workflow.ExecuteActivity(
				ctxTimeout,
				activities.UpdateSupplierBuffersName,
				types.UpdateSupplierBuffersParams{
					Supplier: supplier,
					Tests:    tests,
				},
			),
			func(f workflow.Future) {
				err := f.Get(ctx, &res)
				if err != nil {
					return
				}
				supplierUpdateResultsChan <- res
			},
		)
	}

	for i := 0; i < len(allSuppliers.Suppliers); i++ {
		selector.Select(ctx)
		response := <-supplierUpdateResultsChan
		if response.Success {
			result.ProcessedSuppliers += 1
		} else {
			result.FailedSuppliers += 1
		}
	}

	return &result, nil
}
