package workflows

import (
	"context"
	"requester/activities"
	"requester/types"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type RelayerResults struct {
	TaskIsDone bool
}

type EvaluatorWorkflowParams struct {
	TaskId string `json:"task_id"`
}

var RelayerName = "Relayer"

func (wCtx *Ctx) Relayer(ctx workflow.Context, params activities.RelayerParams) (results *RelayerResults, e error) {
	results = &RelayerResults{}

	// Use workflow.Sleep to make the workflow sleep before actually executing the task.
	// This is to avoid clogging the node with tests
	e = workflow.Sleep(ctx, time.Duration(params.RelayTriggerDelay*1000)*time.Millisecond)
	if e != nil {
		return
	}
	// Now process and send the relay

	if params.RelayTimeout == 0 {
		params.RelayTimeout = (time.Duration(120) * time.Second).Seconds()
	}

	relayerCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue: wCtx.App.Config.Temporal.TaskQueue,
		// we need to give it more time than the relay one because it does other things.
		StartToCloseTimeout: time.Duration(params.RelayTimeout*(float64(activities.RelayRetries+1))) * time.Second,
		WaitForCancellation: true,
		RetryPolicy: &temporal.RetryPolicy{
			BackoffCoefficient: 1,
			MaximumAttempts:    int32(activities.RelayRetries),
		},
	})

	if _, ok := wCtx.App.AppAccounts.Load(params.App.Address); !ok {
		e = temporal.NewNonRetryableApplicationError("application not found", "ApplicationNotFound", nil)
		return
	}

	// Now we will execute the activity that makes the relay to the servicer node
	relayerResponse := types.RelayResponse{}
	relayerErr := workflow.ExecuteActivity(relayerCtx, activities.Activities.Relayer, params).Get(relayerCtx, &relayerResponse)
	if relayerErr != nil {
		e = temporal.NewApplicationErrorWithCause("error retrieve from relayer activity", "Relayer", relayerErr)
		return
	}

	// trigger UpdateTaskTree activity and pass the results
	updateTaskTreeCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		TaskQueue:           wCtx.App.Config.Temporal.TaskQueue,
		StartToCloseTimeout: 60 * time.Second, // TODO : This was 10, find out why this takes so long, maybe make it configurable?
		WaitForCancellation: true,
		RetryPolicy: &temporal.RetryPolicy{
			BackoffCoefficient: 1,
			MaximumAttempts:    3,
		},
	})
	updateTaskTreeResult := activities.UpdateTaskTreeResponse{}
	updateTaskTreeParams := activities.UpdateTaskTreeRequest{PromptId: params.PromptId}
	updateTaskTreeErr := workflow.ExecuteActivity(updateTaskTreeCtx, activities.Activities.UpdateTaskTree, updateTaskTreeParams).Get(updateTaskTreeCtx, &updateTaskTreeResult)
	if updateTaskTreeErr != nil {
		e = temporal.NewApplicationErrorWithCause("error triggering UpdateTaskTree activity", "Relayer", updateTaskTreeErr)
		return
	}

	if updateTaskTreeResult.IsDone {
		// execute the evaluator workflow
		evaluatorParams := EvaluatorWorkflowParams{
			TaskId: updateTaskTreeResult.TaskId,
		}
		evaluatorWorkflowOptions := client.StartWorkflowOptions{
			ID:                       updateTaskTreeResult.TaskId,
			TaskQueue:                wCtx.App.Config.Temporal.Evaluator.TaskQueue,
			WorkflowIDReusePolicy:    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
			WorkflowTaskTimeout:      60 * time.Second,
			WorkflowExecutionTimeout: 120 * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 1,
			},
		}
		// Do not wait for a result by not calling .Get() on the returned future
		_, wfErr := wCtx.App.TemporalClient.ExecuteWorkflow(
			context.Background(),
			evaluatorWorkflowOptions,
			wCtx.App.Config.Temporal.Evaluator.WorkflowName,
			evaluatorParams,
		)
		if wfErr != nil {
			e = temporal.NewApplicationErrorWithCause("error triggering Evaluator workflow", "WorkflowTrigger", wfErr)
			return
		}
	}

	return
}
