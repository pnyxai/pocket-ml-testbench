package workflows

import (
	"context"
	"fmt"
	"math/rand"
	"packages/logger"
	"requester/activities"
	"requester/types"
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
	Suppliers          []string `json:"suppliers"`
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

	// get_block_params
	getHeightActivityCtx := workflow.WithActivityOptions(ctx, ao)
	var currHeight int64 = -1
	l.Debug("Calling GetHeight activity")
	getBlockErr := workflow.ExecuteActivity(
		getHeightActivityCtx,
		activities.Activities.GetHeight,
	).Get(getHeightActivityCtx, &currHeight)
	if getBlockErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get height", "GetHeight", getBlockErr)
		l.Error("GetHeight activity ends with error", "error", e)
		return nil, e
	}
	l.Debug("Calling GetHeight activity ends")

	// Get request data depending if this is a POKT service or an external call
	var blocksPerSession int64
	var sessionHeight int64
	var suppliers map[string]pocket_shannon.Endpoint
	if params.Service != types.ExternalServiceName {

		// get app
		appFound := false
		getAppActivityCtx := workflow.WithActivityOptions(ctx, ao)
		l.Debug("Calling GetApp activity")
		getAppErr := workflow.ExecuteActivity(
			getAppActivityCtx,
			activities.Activities.GetApp,
			activities.GetAppParams{
				Address: params.App,
				Service: params.Service,
			},
		).Get(getAppActivityCtx, &appFound)
		if getAppErr != nil {
			e = temporal.NewApplicationErrorWithCause("unable to get app", "GetApp", getBlockErr)
			l.Error("GetApp activity ends with error", "error", e)
			return nil, e
		}
		l.Debug("Calling GetApp activity ends")
		if !appFound {
			e = temporal.NewNonRetryableApplicationError("application not found in available Apps list", "ApplicationNotFound", nil)
			return nil, e
		}

		// get session
		// TODO : This throws an error when temporal tries to decode the returned
		// 		  variable, specifically: "payload item 0: unable to decode: unknown value \"JSON_RPC\" for enum pocket.shared.RPCType"
		// 		  this is related to the poktroll package and I cannot find a fix, right now.
		//		  LEAVING AS TECH DEBT, USING IN-PLACE CODE INSTEAD
		appSession, err := wCtx.App.PocketFullNode.GetSession(shannon_types.ServiceID(params.Service), params.App)
		if err != nil {
			e = temporal.NewNonRetryableApplicationError("Could not get session data", "SessionNotFound", nil)
			l.Error(fmt.Sprintf("Error getting session data for app %s in service %s", params.App, params.Service))
			return nil, e
		}

		// get_block_params
		blocksPerSession = appSession.NumBlocksPerSession
		sessionHeight = appSession.NumBlocksPerSession * appSession.SessionNumber

		// Get all the endpoint available in this session
		// TODO : Idem previous problem with "GetSession" activity
		suppliers, err = pocket_shannon.EndpointsFromSession(appSession)
		if err != nil {
			e = temporal.NewApplicationErrorWithCause("unable to get endpoints", "GetEndpoints", err)
			l.Error("Error getting endpoints", "error", e)
			return nil, e
		}

	} else {
		// This is a workflow for external services, get the list from the
		// configuration
		suppliers = make(map[string]pocket_shannon.Endpoint)
		for thisAddr, thisData := range wCtx.App.ExternalSuppliers {
			suppliers[thisAddr] = pocket_shannon.Endpoint{
				// This supplier name
				Supplier: thisAddr,
				// The endpoint, we add it here to avoid reading this again later
				Url: thisData.Endpoint,
				// Session left empty, we wont use it
			}
		}
		// This is a placeholder to go through the task search
		sessionHeight = 10
		// And this is hardcoded currently (ShannonSDK is missing this)
		blocksPerSession = wCtx.App.PocketBlocksPerSession
	}

	// For these suppliers, get the pending tasks
	l.Debug("Calling GetTasks activity")
	request := activities.GetTasksParams{
		Suppliers:      make([]string, len(suppliers)),
		Service:        params.Service,
		CurrentSession: sessionHeight,
	}
	i := 0
	for supplierAddrres, _ := range suppliers {
		request.Suppliers[i] = supplierAddrres
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
	triggeredSupplierAddresses := make([]string, 0)
	skippedWorkflows := make([]string, 0)
	triggeredWorkflows := make([]string, 0)

	// Now we must divide tasks into groups of tasks with the same ADDRESS
	reqMap := activities.SplitByUniqueAddress(ltr.TaskRequests)

	// For each group of tasks:
	for _, theseSupplierReq := range reqMap {
		l.Debug("Processing group.", "supplier", theseSupplierReq[0].Supplier, "number of elements", len(theseSupplierReq))

		// For each address
		for reqIdx, tr := range theseSupplierReq {
			// Create a random timeout with a fixed time that marks the rate: 0+1 sec; 2 +- 1 sec ; 4 +- 1 sec ; etc...
			randomDelay := (rand.Float64() * wCtx.App.Config.Relay.TimeDispersion) + (float64(reqIdx) * wCtx.App.Config.Relay.TimeBetweenRelays)
			// add only those suppliers that get pending tasks
			triggeredSupplierAddresses = append(triggeredSupplierAddresses, tr.Supplier)
			// Create target endpoint, which already contains the session
			targetEndpoint := suppliers[tr.Supplier]
			// You can access desired attributes here.
			relayerRequest := activities.RelayerParams{
				AppAddress:        params.App,
				SupplierAddress:   tr.Supplier,
				TargetEndpoint:    targetEndpoint,
				Service:           request.Service,
				SessionHeight:     sessionHeight,
				BlocksPerSession:  blocksPerSession,
				PromptId:          tr.PromptId,
				RelayTimeout:      tr.RelayTimeout,
				RelayTriggerDelay: randomDelay,
			}

			//  Here we start the workflow that will ultimately dispatch the relays to the supplier
			workflowOptions := client.StartWorkflowOptions{
				// with this format: "app-supplier-service-taskId-instanceId-promptId"
				// we are sure that when its workflow runs again inside the same session and the task is still not done,
				// we will not get the same relayer workflow executed twice
				ID: fmt.Sprintf(
					"%s-%s-%s-%s",
					request.Service, tr.Supplier, params.App,
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
			if params.Service == types.ExternalServiceName {
				// patch session height since it will never change in external services
				sessionHeight = 9
			}
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
		App:       params.App,
		Service:   params.Service,
		Suppliers: triggeredSupplierAddresses,
		// check if this is the height of the block when the session is get or what
		Height:             currHeight,
		SessionHeight:      sessionHeight,
		TriggeredWorkflows: triggeredWorkflows,
		SkippedWorkflows:   skippedWorkflows,
	}

	return &result, nil
}
