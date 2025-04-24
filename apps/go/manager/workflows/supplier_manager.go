package workflows

import (
	"fmt"
	"time"

	"manager/activities"
	"manager/types"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

var SupplierManagerName = "Manager"

// SuppliereManager - Is a method that orchestrates the tracking of staked ML suppliers.
// It performs the following activities:
// - Staked suppliers retrieval
// - Analyze suppliers data
// - Triggering new evaluation tasks
func (wCtx *Ctx) SupplierManager(ctx workflow.Context, params types.SupplierManagerParams) (*types.SupplierManagerResults, error) {

	l := wCtx.App.Logger
	l.Debug().Msg("Starting Supplier Manager Workflow.")

	// Create result
	result := types.SupplierManagerResults{SuccessSuppliers: 0}

	// Check parameters
	if len(params.Tests) == 0 {
		l.Error().Msg("Tests array cannot be empty.")
		return &result, fmt.Errorf("tests array cannot be empty")
	}

	// -------------------------------------------------------------------------
	// -------------------- Get all suppliers staked -------------------------------
	// -------------------------------------------------------------------------
	// Set timeout to get staked suppliers activity
	ctxTimeout := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute * 5,
		StartToCloseTimeout:    time.Minute * 5,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second * 5,
			BackoffCoefficient: 2,
			MaximumInterval:    time.Second * 32,
			MaximumAttempts:    5,
		},
	})
	// Set activity input
	getStakedInput := types.GetStakedParams{
		Service: params.Service,
	}
	// Results will be kept logged by temporal
	var pocketNetworkData types.GetStakedResults
	// Execute activity
	err := workflow.ExecuteActivity(ctxTimeout, activities.GetStakedName, getStakedInput).Get(ctx, &pocketNetworkData)
	if err != nil {
		return &result, err
	}

	// -------------------------------------------------------------------------
	// -------------------- Analyze each supplier ------------------------------
	// -------------------------------------------------------------------------
	// Set timeout for supplier analysis activity
	ctxTimeout = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute,
	})

	selector := workflow.NewSelector(ctx)

	// The channel requests are the suppliers data
	suppliers := pocketNetworkData.Suppliers
	// Define a channel to store SupplierAnalysisChanResponse objects
	supplierAnalysisResultsChan := make(chan types.SupplierAnalysisChanResponse, len(suppliers))
	// defer close lookup task results channel
	defer close(supplierAnalysisResultsChan)
	// Iterate all suppliers and execute the analysis in futures
	for _, supplier := range suppliers {
		input := types.AnalyzeSupplierParams{
			Supplier: supplier,
			Block:    pocketNetworkData.Block,
			Tests:    params.Tests,
		}
		ltr := types.AnalyzeSupplierResults{}
		selector.AddFuture(
			workflow.ExecuteActivity(
				ctxTimeout,
				activities.AnalyzeSupplierName,
				input,
			),
			// Declare the function to execute on activity end
			func(f workflow.Future) {
				err1 := f.Get(ctx, &ltr)
				if err1 != nil {
					err = err1
					return
				}
				// Fill the output channel
				supplierAnalysisResultsChan <- types.SupplierAnalysisChanResponse{
					Request:  &supplier,
					Response: &ltr,
				}
			},
		)
	}

	var allTriggers []types.TaskTrigger
	for i := 0; i < len(suppliers); i++ {
		// Each call to Select matches a single ready Future.
		// Each Future is matched only once independently on the number of Select calls.
		// Ensure there is a call to process
		selector.Select(ctx)
		if err != nil {
			return nil, err
		}
		// Retrieve the response from the channel
		response := <-supplierAnalysisResultsChan
		// Append to triggers
		allTriggers = append(allTriggers, response.Response.Triggers...)
		// Keep count
		// Update workflow result
		if response.Response.IsNew {
			result.NewSuppliers += 1
		}
		result.TriggeredTasks += uint(len(response.Response.Triggers))
	}

	// -------------------------------------------------------------------------
	// -------------------- Trigger Sampler ------------------------------------
	// -------------------------------------------------------------------------

	l.Debug().Str("service", params.Service).Int("TriggersNums", len(allTriggers))

	// Define a channel to store TriggerSamplerResults objects
	taskTriggerResultsChan := make(chan *types.TriggerSamplerResults, len(allTriggers))
	// defer close lookup task results channel
	defer close(taskTriggerResultsChan)
	// Iterate all suppliers and execute the analysis in futures
	for _, trigger := range allTriggers {
		input := types.TriggerSamplerParams{
			Trigger: trigger,
		}
		ltr := types.TriggerSamplerResults{}
		selector.AddFuture(
			workflow.ExecuteActivity(
				ctxTimeout,
				activities.TriggerSamplerName,
				input,
			),
			// Declare the function to execute on activity end
			func(f workflow.Future) {
				err1 := f.Get(ctx, &ltr)
				if err1 != nil {
					err = err1
					return
				}
				// Fill the output channel
				taskTriggerResultsChan <- &ltr
			},
		)
	}

	for i := 0; i < len(allTriggers); i++ {
		selector.Select(ctx)
		if err != nil {
			return nil, err
		}
		// Retrieve the response from the channel
		response := <-taskTriggerResultsChan
		// Keep count
		// Update workflow result
		if response.Success {
			result.SuccessSuppliers += 1
		} else {
			result.FailedSuppliers += 1
		}
	}

	return &result, nil
}
