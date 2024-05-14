package activities

import (
	"context"
	"manager/types"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

var TriggerSamplerName = "trigger_sampler"

func (aCtx *Ctx) TriggerSampler(ctx context.Context, params types.TriggerSamplerParams) (*types.TriggerSamplerResults, error) {

	l := aCtx.App.Logger
	l.Debug().Str("address", params.Trigger.Address).Str("service", params.Trigger.Service).Str("framework", params.Trigger.Framework).Str("task", params.Trigger.Task).Msg("Triggering task...")

	result := types.TriggerSamplerResults{}
	result.Success = false

	samplerParams := types.SamplerWorkflowParams{
		Evaluation: params.Trigger.Framework,
		Task:       params.Trigger.Task,
		RequesterArgs: types.RequesterArgs{
			Address: params.Trigger.Address,
			Service: params.Trigger.Service,
			Method:  "",
			Path:    "",
		},
		Blacklist: params.Trigger.Blacklist,
		Qty:       params.Trigger.Qty,
	}
	evaluatorWorkflowOptions := client.StartWorkflowOptions{
		TaskQueue:             aCtx.App.Config.Temporal.Sampler.TaskQueue,
		WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}
	// Do not wait for a result by not calling .Get() on the returned future
	_, err := aCtx.App.TemporalClient.ExecuteWorkflow(
		context.Background(),
		evaluatorWorkflowOptions,
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
