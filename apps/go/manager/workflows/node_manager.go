package workflows

import (
	"fmt"
	"time"

	"manager/activities"
	"manager/types"

	"go.temporal.io/sdk/workflow"
)

var NodeManagerName = "node_manager"

// NodeManager - Is a method that orchestrates the tracking of staked ML nodes.
// It performs the following tasks:
// - Staked nodes retrieval
// - Analyze nodes data
// - Triggering new evaluation tasks
func (wCtx *Ctx) NodeManager(ctx workflow.Context, params types.NodeManagerParams) (*types.NodeManagerResults, error) {

	l := wCtx.App.Logger
	l.Debug().Msg("Starting Node Manager Workflow.")

	// Create result
	result := types.NodeManagerResults{Success: 0}

	// Check parameters
	if len(params.Tests) == 0 {
		l.Error().Msg("Tests array cannot be empty.")
		return &result, fmt.Errorf("tests array cannot be empty")
	}
	if len(params.Service) != 4 {
		l.Error().Msg("Service must be a 4 letter string (4 digit hex number).")
		return &result, fmt.Errorf("service must be a 4 letter string (4 digit hex number)")
	}

	// -------------------------------------------------------------------------
	// -------------------- Get all nodes staked -------------------------------
	// -------------------------------------------------------------------------
	// Set timeout to get staked nodes activity
	ctxTimeout := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute * 5,
		StartToCloseTimeout:    time.Minute * 5,
	})
	// Set activity input
	getStakedInput := types.GetStakedParams{
		Service: params.Service,
	}
	// Results will be kept logged by temporal
	var stakedNodes types.GetStakedResults
	// Execute activity
	err := workflow.ExecuteActivity(ctxTimeout, activities.GetStakedName, getStakedInput).Get(ctx, &stakedNodes)
	if err != nil {
		return &result, err
	}

	// -------------------------------------------------------------------------
	// -------------------- Analyze each node ----------------------------------
	// -------------------------------------------------------------------------
	// Set timeout for nodes analysis activity
	ctxTimeout = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		ScheduleToStartTimeout: time.Minute,
		StartToCloseTimeout:    time.Minute,
	})

	selector := workflow.NewSelector(ctx)

	// The channel requests are the nodes data
	nodes := stakedNodes.Nodes
	// Define a channel to store NodeAnalysisChanResponse objects
	nodeAnalysisResultsChan := make(chan types.NodeAnalysisChanResponse, len(nodes))
	// defer close lookup task results channel
	defer close(nodeAnalysisResultsChan)
	// Iterate all nodes and execute the analysis in futures
	for _, node := range nodes {
		input := types.AnalyzeNodeParams{
			Node:  node,
			Tests: params.Tests,
		}
		ltr := types.AnalyzeNodeResults{}
		selector.AddFuture(
			workflow.ExecuteActivity(
				ctxTimeout,
				activities.AnalyzeNodeName,
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
				nodeAnalysisResultsChan <- types.NodeAnalysisChanResponse{
					Request:  &node,
					Response: &ltr,
				}
			},
		)
	}

	for _, node := range nodes {
		// Each call to Select matches a single ready Future.
		// Each Future is matched only once independently on the number of Select calls.
		// Ensure there is a call to process
		selector.Select(ctx)
		if err != nil {
			return nil, err
		}
		// Retrieve the response from the channel
		response := <-nodeAnalysisResultsChan
		// Update workflow result
		if response.Response.Success {
			result.Success += 1
		} else {
			result.Failed += 1
		}
		l.Debug().Str("address", node.Address).Str("service", node.Service).Bool("success", response.Response.Success).Msg("Task Done.")
	}

	return &result, nil
}
