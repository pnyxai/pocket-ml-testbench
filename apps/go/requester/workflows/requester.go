package workflows

import (
	"context"
	"fmt"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"requester/activities"
	"time"
)

type RequesterParams struct {
	App     string `json:"app"`
	Service string `json:"service"`
}

type RequesterNodeResults struct {
	Address string  `json:"address"`
	Relays  uint    `json:"relays"`
	Success uint    `json:"success"`
	Failed  uint    `json:"failed"`
	AvgMs   float32 `json:"avg_ms"`
}

type RequesterResults struct {
	App                string   `json:"app"`
	Service            string   `json:"service"`
	Nodes              []string `json:"nodes"`
	Height             int64    `json:"height"`
	SessionHeight      int64    `json:"session_height"`
	TriggeredWorkflows []string `json:"workflows"`
	// somehow, maybe we could identify when workflow trigger fail because it is already waiting
	SkippedWorkflows []string `json:"skipped_workflows"`
}

type LookupChanResponse struct {
	Request  *activities.LookupTaskRequestParams
	Response *activities.LookupTaskRequestResults
}

type RelayerChanResponse struct {
	Request  *activities.RelayerParams
	Response *activities.RelayerResults
}

var RequesterName = "requester"

// Requester check sessions
func (wCtx *Ctx) Requester(ctx workflow.Context, params RequesterParams) (r *RequesterResults, e error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// GetApp will try to retrieve the application state from the RPC
	// with this we ensure it exists and has the chain staked
	getAppResults := poktGoSdk.App{}
	getAppErr := workflow.ExecuteActivity(ctx, activities.Activities.GetApp, activities.GetAppParams{
		Address: params.App,
		Service: params.Service,
	}).Get(ctx, &getAppResults)

	if getAppErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get app", "GetApp", getAppErr)
		return
	}

	sessionResult := poktGoSdk.DispatchOutput{}
	getSessionErr := workflow.ExecuteActivity(
		ctx,
		activities.Activities.GetSession,
		activities.GetSessionParams{
			App:     params.App,
			Service: params.Service,
		},
	).Get(ctx, &sessionResult)
	if getSessionErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get session", "GetSession", getSessionErr)
		return
	}

	selector := workflow.NewSelector(ctx)
	sessionHeight := int64(sessionResult.Session.Header.SessionHeight)
	nodes := sessionResult.Session.Nodes
	nodeAddresses := make([]string, len(nodes))
	// Define a channel to store LookupTaskRequestResults objects
	lookupTaskResultsChan := make(chan LookupChanResponse, len(nodes))

	for _, node := range nodes {
		nodeAddresses = append(nodeAddresses, node.Address)
		request := activities.LookupTaskRequestParams{
			Node:    node.Address,
			Service: params.Service,
		}
		ltr := activities.LookupTaskRequestResults{}
		selector.AddFuture(
			workflow.ExecuteActivity(
				ctx,
				activities.Activities.LookupTaskRequest,
				request,
			),
			func(f workflow.Future) {
				err1 := f.Get(ctx, &ltr)
				if err1 != nil {
					e = err1
					return
				}
				// Add the LookupTaskRequestResults object to the channel
				lookupTaskResultsChan <- LookupChanResponse{
					Request:  &request,
					Response: &ltr,
				}
			},
		)
	}

	for i := 0; i < len(nodes); i++ {
		// Each call to Select matches a single ready Future.
		// Each Future is matched only once independently on the number of Select calls.
		selector.Select(ctx)
		if e != nil {
			return
		}
	}

	// close lookup task results channel
	close(lookupTaskResultsChan)

	skippedWorkflows := make([]string, 0)
	triggeredWorkflows := make([]string, 0)

	for ltr := range lookupTaskResultsChan {
		request := ltr.Request
		for _, tr := range ltr.Response.TaskRequests {
			// You can access desired attributes here.
			relayerRequest := activities.RelayerParams{
				// todo: check if need to add anything else
				App:           params.App,
				Node:          request.Node,
				Service:       request.Service,
				SessionHeight: sessionHeight,
				TaskId:        tr.TaskId,
				InstanceId:    tr.TaskId,
				PromptId:      tr.PromptId,
			}

			workflowOptions := client.StartWorkflowOptions{
				// with this format: "app-node-service-taskId-instanceId-promptId-sessionHeight"
				// we are sure that when its workflow runs again inside the same session and the task is still not done,
				// we will not get the same relayer workflow executed twice
				ID: fmt.Sprintf(
					"%s-%s-%s-%s-%s-%s-%d",
					params.App, request.Node, request.Service,
					tr.TaskId, tr.InstanceId, tr.PromptId, sessionHeight,
				),
				// todo: check if this is the proper strategy here.
				WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
			}

			// Do not wait for a result by not calling .Get() on the returned future
			wf, err := wCtx.App.TemporalClient.ExecuteWorkflow(
				context.Background(),
				workflowOptions,
				wCtx.Relayer,
				relayerRequest,
			)

			if err != nil {
				// check if error is because workflow is already in queue/failed
				// OTHERWISE fail the workflow
				// skippedWorkflows = append(skippedWorkflows, fmt.Sprintf("ID:%s/RUN_ID:%s", wf.GetID(), wf.GetRunID()))
				continue
			}

			triggeredWorkflows = append(triggeredWorkflows, fmt.Sprintf("ID:%s/RUN_ID:%s", wf.GetID(), wf.GetRunID()))
		}
	}

	result := RequesterResults{
		App:     params.App,
		Service: params.Service,
		Nodes:   nodeAddresses,
		// check if this is the height of the block when the session is get or what
		Height:             int64(sessionResult.BlockHeight),
		SessionHeight:      int64(sessionHeight),
		TriggeredWorkflows: triggeredWorkflows,
		SkippedWorkflows:   skippedWorkflows,
	}

	return &result, nil
}
