package workflows

import (
	"context"
	"fmt"
	"math/rand"
	"packages/logger"
	"requester/activities"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"packages/pocket_shannon"
	shannon_types "packages/pocket_shannon/types"
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

var RequesterName = "Requester"

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
		StartToCloseTimeout: 300 * time.Second,
		WaitForCancellation: true,
		RetryPolicy: &temporal.RetryPolicy{
			BackoffCoefficient: 1,
			MaximumAttempts:    3,
		},
	}

	l.Info("Starting workflow", "Application", params.App, "Service", params.Service)

	// Get latest block
	currHeight, err := wCtx.App.PocketFullNode.GetLatestBlockHeight()
	if err != nil {
		e = temporal.NewNonRetryableApplicationError("Could not retrieve latest block height.", "LatestBlockQuery", nil)
		return
	}

	found := false
	for appAddress, _ := range wCtx.App.PocketApps {
		if appAddress == params.App {
			found = true
			break
		}
	}
	if !found {
		e = temporal.NewNonRetryableApplicationError("application not found in available Apps list", "ApplicationNotFound", nil)
		return
	}

	// Check if the app is correctly staked for service
	l.Debug("Checking app: ", params.App)
	ctxNode := context.Background()
	onchainApp, err := wCtx.App.PocketFullNode.GetApp(ctxNode, params.App)
	if err != nil {
		temporal.NewNonRetryableApplicationError("Error getting on-chain data", "ApplicationNotFound", nil)
		l.Error("Error getting on-chain data for app", params.App, " : ", err)
		return
	}
	if onchainApp == nil {
		temporal.NewNonRetryableApplicationError("Cannot find App on-chain data", "ApplicationNotFound", nil)
		l.Error("No on-chain data for app", params.App, " : ", err)
		return
	}

	// Check if the app is staked for the requested service
	if !pocket_shannon.AppIsStakedForService(shannon_types.ServiceID(params.Service), onchainApp) {
		temporal.NewNonRetryableApplicationError("App not staked for service", "ApplicationNotStaked", nil)
		l.Error(fmt.Sprintf("App %s is not staked for service %s", params.App, params.Service))
		return
	}

	// Get App session
	appSession, err := wCtx.App.PocketFullNode.GetSession(shannon_types.ServiceID(params.Service), params.App)
	if err != nil {
		temporal.NewNonRetryableApplicationError("Could not get session data", "SessionNotFound", nil)
		l.Error(fmt.Sprintf("Error getting session data for app %s in service %s", params.App, params.Service))
		return
	}

	// get_block_params
	blocksPerSession := appSession.NumBlocksPerSession
	sessionHeight := appSession.NumBlocksPerSession * appSession.SessionNumber

	// Get all the endpoint available in this session
	suppliers, err := pocket_shannon.EndpointsFromSession(appSession)

	// For these suppliers, get the pending tasks
	l.Debug("Calling GetTasks activity")
	request := activities.GetTasksParams{
		Nodes:          make([]string, len(suppliers)),
		Service:        params.Service,
		CurrentSession: sessionHeight,
	}
	i := 0
	for supplierAddrres, _ := range suppliers {
		request.Nodes[i] = string(supplierAddrres)
		i += 1
	}
	getTasksActivityCtx := workflow.WithActivityOptions(ctx, ao)
	ltr := activities.GetTaskRequestResults{}
	getTasksErr := workflow.ExecuteActivity(
		getTasksActivityCtx,
		activities.Activities.GetTasks,
		request,
	).Get(getTasksActivityCtx, &ltr)
	if getTasksErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get tasks", "GetTasks", getTasksErr)
		l.Error("GetTasks activity ends with error", "error", e)
		return
	}

	l.Debug("GetTasks activity ends", "tasks_found", len(ltr.TaskRequests))

	// With all the data, we will proceed to trigger the relaying workflow
	triggeredNodeAddresses := make([]string, 0)
	skippedWorkflows := make([]string, 0)
	triggeredWorkflows := make([]string, 0)

	// Now we must divide tasks into groups of tasks with the same ADDRESS
	reqMap := activities.SplitByUniqueAddress(ltr.TaskRequests)

	// For each group of tasks:
	for _, theseNodeReq := range reqMap {
		l.Debug("Processing group.", "node", theseNodeReq[0].Node, "number of elements", len(theseNodeReq))

		// For each address
		for reqIdx, tr := range theseNodeReq {
			// Create a random timeout with a fixed time that marks the rate: 0+1 sec; 2 +- 1 sec ; 4 +- 1 sec ; etc...
			randomDelay := (rand.Float64() * wCtx.App.Config.Relay.TimeDispersion) + (float64(reqIdx) * wCtx.App.Config.Relay.TimeBetweenRelays)
			// add only those nodes that get pending tasks
			triggeredNodeAddresses = append(triggeredNodeAddresses, tr.Node)
			// Create target endpoint, which already contains the session
			targetEndpoint := suppliers[shannon_types.EndpointAddr(tr.Node)]
			// You can access desired attributes here.
			relayerRequest := activities.RelayerParams{
				AppAddress:        params.App,
				AppPrivHex:        wCtx.App.PocketApps[params.App],
				NodeAddress:       tr.Node,
				TargetEndpoint:    targetEndpoint,
				Service:           request.Service,
				SessionHeight:     sessionHeight,
				BlocksPerSession:  blocksPerSession,
				PromptId:          tr.PromptId,
				RelayTimeout:      tr.RelayTimeout,
				RelayTriggerDelay: randomDelay,
			}

			//  Here we start the workflow that will ultimately dispatch the relays to the servicer nodes
			workflowOptions := client.StartWorkflowOptions{
				// with this format: "app-node-service-taskId-instanceId-promptId"
				// we are sure that when its workflow runs again inside the same session and the task is still not done,
				// we will not get the same relayer workflow executed twice
				ID: fmt.Sprintf(
					"%s-%s-%s-%s",
					request.Service, tr.Node, params.App,
					tr.PromptId,
				),
				TaskQueue:                                wCtx.App.Config.Temporal.TaskQueue,
				WorkflowExecutionErrorWhenAlreadyStarted: true,
				WorkflowIDReusePolicy:                    enums.WORKFLOW_ID_REUSE_POLICY_TERMINATE_IF_RUNNING,
				WorkflowTaskTimeout:                      (time.Duration(tr.RelayTimeout) * time.Second) + time.Duration(randomDelay*1000)*time.Millisecond + (time.Duration(30) * time.Second),
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

			// Update the prompt entry
			l.Debug("Calling SetPromptTriggerSession activity")
			triggerUpdate := activities.SetPromptTriggerSessionParams{
				PromptId:       tr.PromptId,
				TriggerSession: sessionHeight,
			}
			setPromptTriggerSessionActivityCtx := workflow.WithActivityOptions(ctx, ao)
			var errorSet error
			getTasksErr := workflow.ExecuteActivity(
				setPromptTriggerSessionActivityCtx,
				activities.Activities.SetPromptTriggerSession,
				triggerUpdate,
			).Get(setPromptTriggerSessionActivityCtx, &errorSet)
			if getTasksErr != nil {
				l.Error("SetPromptTriggerSession activity ends with error", "error", getTasksErr)
			}
			if errorSet != nil {
				l.Error("SetPromptTriggerSession mongo update ends with error", "error", errorSet)
			}

		}

	}

	result := RequesterResults{
		App:     params.App,
		Service: params.Service,
		Nodes:   triggeredNodeAddresses,
		// check if this is the height of the block when the session is get or what
		Height:             currHeight,
		SessionHeight:      sessionHeight,
		TriggeredWorkflows: triggeredWorkflows,
		SkippedWorkflows:   skippedWorkflows,
	}

	return &result, nil
}
