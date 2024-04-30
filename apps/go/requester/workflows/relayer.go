package workflows

import (
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"requester/activities"
	"time"
)

var RelayerName = "Relayer"

func (wCtx *Ctx) Relayer(ctx workflow.Context, params activities.RelayerParams) (results *activities.RelayerResults, e error) {
	//l := logger.GetWorkflowLogger(RelayerName, ctx, params)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	relayerErr := workflow.ExecuteActivity(ctx, activities.Activities.Relayer, params).Get(ctx, &results)
	if relayerErr != nil {
		e = temporal.NewApplicationErrorWithCause("error retrieve from relayer activity", "Relayer", relayerErr)
		return
	}

	// trigger another workflow/activities to evaluate Instance.Done and Task.Done after this one is done
	// this will mark prompt.done = true, so we need to evaluate the related instance to see if every prompt is done=true
	// to mark instance.done = true also, and if all the instance are done=true then mark the related task with done=true too.
	// and if the task is marked as done=true, then it needs to trigger the evaluator workflow giving to it the task id that is done

	return
}
