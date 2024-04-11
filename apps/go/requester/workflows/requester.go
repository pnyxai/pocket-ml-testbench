package workflows

import (
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
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
	App           string `json:"app"`
	Chain         string `json:"chain"`
	SessionHeight int64  `json:"session_height"`
	Nodes         []RequesterNodeResults
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
func (wCtx *Ctx) Requester(ctx workflow.Context, params RequesterParams) (*RequesterResults, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// GetApp will try to retrieve the application state from the RPC
	// with this we ensure it exists and has the chain staked
	getAppResults := poktGoSdk.App{}
	err := workflow.ExecuteActivity(ctx, activities.Activities.GetApp, activities.GetAppParams{
		Address: params.App,
		Service: params.Service,
	}).Get(ctx, &getAppResults)

	if err != nil {
		return nil, err
	}

	blockResult := activities.GetBlockResults{}
	sessionResult := poktGoSdk.DispatchOutput{}

	selector := workflow.NewSelector(ctx)
	// Read block+params using GetBlock activity
	selector.AddFuture(
		workflow.ExecuteActivity(
			ctx,
			activities.Activities.GetBlock,
			activities.GetBlockParams{
				Height: 0,
			},
		),
		func(f workflow.Future) {
			err1 := f.Get(ctx, &blockResult)
			if err1 != nil {
				err = err1
				return
			}
		},
	)
	// Read current Session for the give App+Service using GetSession activity
	selector.AddFuture(
		workflow.ExecuteActivity(
			ctx,
			activities.Activities.GetSession,
			activities.GetSessionParams{
				App:     params.App,
				Service: params.Service,
			},
		),
		func(f workflow.Future) {
			err1 := f.Get(ctx, &sessionResult)
			if err1 != nil {
				err = err1
				return
			}
		},
	)

	// 1 GetBlock
	// 2 GetSession
	// (order does not matter)
	for i := 0; i < 2; i++ {
		// Each call to Select matches a single ready Future.
		// Each Future is matched only once independently on the number of Select calls.
		selector.Select(ctx)
		if err != nil {
			return nil, err
		}
	}

	nodes := sessionResult.Session.Nodes
	// Define a channel to store LookupTaskRequestResults objects
	lookupTaskResultsChan := make(chan LookupChanResponse, len(nodes))

	for _, node := range nodes {
		request := activities.LookupTaskRequestParams{
			Node:    node.Address,
			App:     params.App,
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
					err = err1
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
		if err != nil {
			return nil, err
		}
	}

	// close lookup task results channel
	close(lookupTaskResultsChan)

	relayerResultsChan := make(chan RelayerChanResponse, len(nodes))

	relayerActivities := 0

	for ltr := range lookupTaskResultsChan {
		request := ltr.Request
		for _, tr := range ltr.Response.TaskRequests {
			// You can access desired attributes here.
			relayerRequest := activities.RelayerParams{
				// todo: check if need to add anything else
				App:     params.App,
				Node:    request.Node,
				Service: request.Service,

				TaskId:     tr.TaskId,
				InstanceId: tr.TaskId,
				PromptId:   tr.PromptId,
			}
			rr := activities.RelayerResults{}
			selector.AddFuture(
				workflow.ExecuteActivity(
					ctx,
					activities.Activities.Relayer,
					relayerRequest,
				),
				func(f workflow.Future) {
					err1 := f.Get(ctx, &rr)
					if err1 != nil {
						err = err1
						return
					}
					// Add the LookupTaskRequestResults object to the channel
					relayerResultsChan <- RelayerChanResponse{
						Request:  &relayerRequest,
						Response: &rr,
					}
				},
			)
			relayerActivities++
		}
	}

	for i := 0; i < relayerActivities; i++ {
		// Each call to Select matches a single ready Future.
		// Each Future is matched only once independently on the number of Select calls.
		selector.Select(ctx)
		if err != nil {
			return nil, err
		}
	}

	close(relayerResultsChan)

	//for rr := range relayerResultsChan {
	//	// todo: iterate over this to create workflow result grouping by node.
	//}
	// for each app in config.apps
	// activity: get_app
	// if not ok: exit
	// if ok:
	// activities in parallel:
	// 1. activity: get_session
	// 2. activity: get_block
	// for each app + service (chain) -> activities (parallel):
	// 2. lookup tasks requests that match session.nodes.address + service and return (task, instance, prompts - ids) from mongodb
	// when 0 task requests: exit
	// when 1+ task requests:

	// Activities (parallel with future https://github.com/temporalio/samples-go/tree/main/splitmerge-selector)
	// for each compact task request call relayer activity
	// This will do the relay and save results on test request record.

	// Merge the results and prepare the result
	// merge results and return SessionCheckerResults
	// Make the results of the workflow available
	result := RequesterResults{}

	return &result, nil
}
