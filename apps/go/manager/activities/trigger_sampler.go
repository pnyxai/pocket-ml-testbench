package activities

import (
	"context"
	"fmt"
	"manager/types"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

var TriggerSamplerName = "trigger_sampler"

func (aCtx *Ctx) TriggerSampler(_ context.Context, params types.TriggerSamplerParams) (*types.TriggerSamplerResults, error) {

	l := aCtx.App.Logger
	l.Debug().Str("address", params.Trigger.Address).Str("service", params.Trigger.Service).Str("framework", params.Trigger.Framework).Str("task", params.Trigger.Task).Msg("Triggering task...")

	result := types.TriggerSamplerResults{}
	result.Success = false

	samplerParams := types.SamplerWorkflowParams{
		Framework: params.Trigger.Framework,
		Task:      params.Trigger.Task,
		RequesterArgs: types.RequesterArgs{
			Address: params.Trigger.Address,
			Service: params.Trigger.Service,
		},
		Blacklist: params.Trigger.Blacklist,
		Qty:       params.Trigger.Qty,
	}
	samplerWorkflowOptions := client.StartWorkflowOptions{
		ID: fmt.Sprintf(
			// lmeh-hellaswag-nodeaddress-servicehex
			"%s-%s-%s-%s",
			params.Trigger.Framework,
			params.Trigger.Task,
			params.Trigger.Address,
			params.Trigger.Service,
		),
		TaskQueue:                                aCtx.App.Config.Temporal.Sampler.TaskQueue,
		WorkflowExecutionErrorWhenAlreadyStarted: true,
		WorkflowIDReusePolicy:                    enums.WORKFLOW_ID_REUSE_POLICY_TERMINATE_IF_RUNNING,
		WorkflowTaskTimeout:                      120 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
	// Do not wait for a result by not calling .Get() on the returned future
	_, err := aCtx.App.TemporalClient.ExecuteWorkflow(
		context.Background(),
		samplerWorkflowOptions,
		aCtx.App.Config.Temporal.Sampler.WorkflowName,
		samplerParams,
	)
	if err != nil {
		err = temporal.NewApplicationErrorWithCause("error triggering Sampler workflow", "WorkflowTrigger", err)
		return &result, err
	}
	result.Success = true
	return &result, nil
}
