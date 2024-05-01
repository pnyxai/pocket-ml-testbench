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
	"requester/common"
	"time"
)

type RequesterParams struct {
	App     string `json:"app"`
	Service string `json:"service"`
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
	Node     *poktGoSdk.Node
	Request  *activities.GetTasksParams
	Response *activities.GetTaskRequestResults
}

var RequesterName = "requester"

// Requester check sessions
func (wCtx *Ctx) Requester(ctx workflow.Context, params RequesterParams) (r *RequesterResults, e error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10000 * time.Second,
	}
	activityCtx := workflow.WithActivityOptions(ctx, ao)

	// GetApp will try to retrieve the application state from the RPC
	// with this we ensure it exists and has the chain staked
	getAppResults := &poktGoSdk.App{}
	getAppErr := workflow.ExecuteActivity(activityCtx, activities.Activities.GetApp, activities.GetAppParams{
		Address: params.App,
		Service: params.Service,
	}).Get(activityCtx, getAppResults)
	if getAppErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get app", "GetApp", getAppErr)
		return
	}

	// get session
	sessionResult := poktGoSdk.DispatchOutput{}
	getSessionErr := workflow.ExecuteActivity(
		activityCtx,
		activities.Activities.GetSession,
		activities.GetSessionParams{
			App:     params.App,
			Service: params.Service,
		},
	).Get(activityCtx, &sessionResult)
	if getSessionErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get session", "GetSession", getSessionErr)
		return
	}

	// get_block_params
	allParams := poktGoSdk.AllParams{}
	getBlockErr := workflow.ExecuteActivity(
		activityCtx,
		activities.Activities.GetBlockParams,
		// latest height always
		int64(0),
	).Get(activityCtx, &allParams)

	if getBlockErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get block params", "GetBlockParams", getBlockErr)
		return
	}

	blocksPerSession, blocksPerSessionErr := common.GetBlocksPerSession(&allParams)
	if blocksPerSessionErr != nil {
		return nil, temporal.NewApplicationErrorWithCause(blocksPerSessionErr.Error(), "GetBlocksPerSession", blocksPerSessionErr)
	}

	selector := workflow.NewSelector(ctx)
	sessionHeight := int64(sessionResult.Session.Header.SessionHeight)
	nodes := sessionResult.Session.Nodes
	nodeAddresses := make([]string, len(nodes))
	// Define a channel to store GetTaskRequestResults objects
	lookupTaskResultsChan := make(chan LookupChanResponse, len(nodes))

	for _, node := range nodes {
		nodeAddresses = append(nodeAddresses, node.Address)
		request := activities.GetTasksParams{
			Node:    node.Address,
			Service: params.Service,
		}
		ltr := activities.GetTaskRequestResults{}
		selector.AddFuture(
			workflow.ExecuteActivity(
				activityCtx,
				activities.Activities.GetTasks,
				request,
			),
			func(f workflow.Future) {
				err1 := f.Get(activityCtx, &ltr)
				if err1 != nil {
					e = err1
					return
				}
				// Add the GetTaskRequestResults object to the channel
				lookupTaskResultsChan <- LookupChanResponse{
					Node:     &node,
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
				App:     getAppResults,
				Node:    ltr.Node,
				Session: sessionResult.Session,

				Service:          request.Service,
				SessionHeight:    sessionHeight,
				BlocksPerSession: blocksPerSession,

				PromptId: tr.PromptId,
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
		SessionHeight:      sessionHeight,
		TriggeredWorkflows: triggeredWorkflows,
		SkippedWorkflows:   skippedWorkflows,
	}

	return &result, nil
}
