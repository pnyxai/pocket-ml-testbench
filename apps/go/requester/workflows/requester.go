package workflows

import (
	"context"
	"fmt"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"packages/logger"
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
	l := logger.GetWorkflowLogger(RequesterName, ctx, params)
	defer func() {
		if e != nil {
			l.Error("Workflow ends with error", "error", e, params.App, "Service", params.Service)
		} else {
			l.Debug("Requester workflow ends successfully", params.App, "Service", params.Service)
		}
	}()

	ao := workflow.ActivityOptions{
		TaskQueue:           wCtx.App.Config.Temporal.TaskQueue,
		StartToCloseTimeout: 10 * time.Second,
		WaitForCancellation: false,
		RetryPolicy: &temporal.RetryPolicy{
			BackoffCoefficient: 1,
			MaximumAttempts:    3,
		},
	}
	l.Info("Starting workflow", "Application", params.App, "Service", params.Service)
	if _, ok := wCtx.App.AppAccounts.Load(params.App); !ok {
		e = temporal.NewNonRetryableApplicationError("application not found", "ApplicationNotFound", nil)
		return
	}

	// GetApp will try to retrieve the application state from the RPC
	// with this we ensure it exists and has the chain staked
	getAppActivityCtx := workflow.WithActivityOptions(ctx, ao)
	getAppResults := &poktGoSdk.App{}
	l.Debug("Calling GetApp activity")
	getAppErr := workflow.ExecuteActivity(getAppActivityCtx, activities.Activities.GetApp, activities.GetAppParams{
		Address: params.App,
		Service: params.Service,
	}).Get(getAppActivityCtx, getAppResults)
	if getAppErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get app", "GetApp", getAppErr)
		l.Error("GetApp activity ends with error", "error", e)
		return
	}
	l.Debug("GetApp activity ends successfully")

	// get session
	getSessionActivityCtx := workflow.WithActivityOptions(ctx, ao)
	sessionResult := poktGoSdk.DispatchOutput{}
	l.Debug("Calling GetSession activity")
	getSessionErr := workflow.ExecuteActivity(
		getSessionActivityCtx,
		activities.Activities.GetSession,
		activities.GetSessionParams{
			App:     params.App,
			Service: params.Service,
		},
	).Get(getSessionActivityCtx, &sessionResult)
	if getSessionErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get session", "GetSession", getSessionErr)
		l.Error("GetSession activity ends with error", "error", e)
		return
	}
	l.Debug("GetSession activity ends successfully")

	// get_block_params
	getAllParamsActivityCtx := workflow.WithActivityOptions(ctx, ao)
	allParams := poktGoSdk.AllParams{}
	l.Debug("Calling GetBlockParams activity")
	getBlockErr := workflow.ExecuteActivity(
		getAllParamsActivityCtx,
		activities.Activities.GetBlockParams,
		// latest height always
		int64(0),
	).Get(getAllParamsActivityCtx, &allParams)
	if getBlockErr != nil {
		l.Error("GetBlockParams activity ends with error", "error", e)
		e = temporal.NewApplicationErrorWithCause("unable to get block params", "GetBlockParams", getBlockErr)
		return
	}
	l.Debug("Calling GetBlockParams activity")

	blocksPerSession, blocksPerSessionErr := common.GetBlocksPerSession(&allParams)
	if blocksPerSessionErr != nil {
		return nil, temporal.NewApplicationErrorWithCause(blocksPerSessionErr.Error(), "GetBlocksPerSession", blocksPerSessionErr)
	}

	sessionHeight := int64(sessionResult.Session.Header.SessionHeight)
	nodes := sessionResult.Session.Nodes
	triggeredNodeAddresses := make([]string, 0)

	l.Debug("Calling GetTasks activity")
	request := activities.GetTasksParams{
		Nodes:   make([]string, len(nodes)),
		Service: params.Service,
	}
	nodesMap := make(map[string]*poktGoSdk.Node, len(nodes))
	for i, node := range nodes {
		request.Nodes[i] = node.Address
		nodesMap[node.Address] = &node
	}
	getTasksActivityCtx := workflow.WithActivityOptions(ctx, ao)
	ltr := activities.GetTaskRequestResults{}
	getTasksErr := workflow.ExecuteActivity(
		getTasksActivityCtx,
		activities.Activities.GetTasks,
		request,
	).Get(getAllParamsActivityCtx, &ltr)
	if getTasksErr != nil {
		l.Error("GetTasks activity ends with error", "error", e)
		e = temporal.NewApplicationErrorWithCause("unable to get tasks", "GetTasks", getTasksErr)
		return
	}

	skippedWorkflows := make([]string, 0)
	triggeredWorkflows := make([]string, 0)

	for _, tr := range ltr.TaskRequests {
		node := nodesMap[tr.Node]
		if node == nil {
			l.Error("missing node on Map from TaskRequest response", "node", tr.Node, "nodes", request.Nodes)
			continue
		}
		// add only those nodes that get pending tasks
		triggeredNodeAddresses = append(triggeredNodeAddresses, tr.Node)
		// You can access desired attributes here.
		relayerRequest := activities.RelayerParams{
			App:              getAppResults,
			Node:             node,
			Session:          sessionResult.Session,
			Service:          request.Service,
			SessionHeight:    sessionHeight,
			BlocksPerSession: blocksPerSession,
			PromptId:         tr.PromptId,
			RelayTimeout:     tr.RelayTimeout,
		}

		workflowOptions := client.StartWorkflowOptions{
			// with this format: "app-node-service-taskId-instanceId-promptId-sessionHeight"
			// we are sure that when its workflow runs again inside the same session and the task is still not done,
			// we will not get the same relayer workflow executed twice
			ID: fmt.Sprintf(
				"%s-%s-%s-%s-%s-%s-%d",
				params.App, tr.Node, request.Service,
				tr.TaskId, tr.InstanceId, tr.PromptId, sessionHeight,
			),
			TaskQueue:                                wCtx.App.Config.Temporal.TaskQueue,
			WorkflowExecutionErrorWhenAlreadyStarted: true,
			WorkflowIDReusePolicy:                    enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
			WorkflowTaskTimeout:                      time.Duration(tr.RelayTimeout) * time.Second,
			RetryPolicy: &temporal.RetryPolicy{
				MaximumAttempts: 3,
			},
		}

		// Do not wait for a result by not Calling .Get() on the returned future
		wf, err := wCtx.App.TemporalClient.ExecuteWorkflow(
			context.Background(),
			workflowOptions,
			wCtx.Relayer,
			relayerRequest,
		)

		if err != nil {
			// check if error is because workflow is already in queue/failed
			// OTHERWISE fail the workflow
			if wf != nil {
				skippedWorkflows = append(skippedWorkflows, fmt.Sprintf("ID:%s/RUN_ID:%s", wf.GetID(), wf.GetRunID()))
			}
			continue
		}

		triggeredWorkflows = append(triggeredWorkflows, fmt.Sprintf("ID:%s/RUN_ID:%s", wf.GetID(), wf.GetRunID()))
	}

	result := RequesterResults{
		App:     params.App,
		Service: params.Service,
		Nodes:   triggeredNodeAddresses,
		// check if this is the height of the block when the session is get or what
		Height:             int64(sessionResult.BlockHeight),
		SessionHeight:      sessionHeight,
		TriggeredWorkflows: triggeredWorkflows,
		SkippedWorkflows:   skippedWorkflows,
	}

	return &result, nil
}
