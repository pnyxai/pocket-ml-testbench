package workflows

import (
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"requester/activities"
	"strconv"
	"time"
)

var RelayerName = "Relayer"

func GetBlocksPerSession(params *poktGoSdk.AllParams) (int64, error) {
	blocksPerSessionStr, ok := params.NodeParams.Get("pos/BlocksPerSession")
	if !ok {
		return 0, temporal.NewApplicationError("unable to get pos/BlocksPerSession from block params", "GetBlockParam")
	}
	blocksPerSession, parseIntErr := strconv.ParseInt(blocksPerSessionStr, 10, 64)
	if parseIntErr != nil {
		return 0, temporal.NewApplicationErrorWithCause("unable to parse to int the value provided by pos/BlocksPerSession from block params", "ParseInt", parseIntErr, blocksPerSessionStr)
	}
	return blocksPerSession, nil
}

func GetCurrentSession(currentHeight, blocksPerSession int64) int64 {
	currentSessionHeight := int64(0)

	if currentHeight%blocksPerSession == 0 {
		currentSessionHeight = currentHeight - blocksPerSession + 1
	} else {
		// calculate the latest session block height by diving the current block height by the blocksPerSession
		currentSessionHeight = (currentHeight/blocksPerSession)*blocksPerSession + 1
	}

	return currentSessionHeight
}

func CanHandleRelayWithinTolerance(currentSessionHeight, requestedSessionHeight, blocksPerSession, sessionTolerance int64) bool {
	tolerance := sessionTolerance * blocksPerSession
	minHeight := requestedSessionHeight - tolerance
	return minHeight <= currentSessionHeight && currentSessionHeight <= currentSessionHeight
}

func (wCtx *Ctx) Relayer(ctx workflow.Context, params activities.RelayerParams) (results *activities.RelayerResults, e error) {
	//l := logger.GetWorkflowLogger(RelayerName, ctx, params)
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	if params.SessionHeight <= 0 {
		return nil, temporal.NewNonRetryableApplicationError("session height <= 0", "BadParams", nil)
	}

	// get_height
	blockHeight := int64(0)
	getHeightErr := workflow.ExecuteActivity(ctx, activities.Activities.GetHeight).Get(ctx, &blockHeight)

	if getHeightErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get height", "GetHeight", getHeightErr)
		return
	}

	// get_block+params
	blockResult := activities.GetBlockResults{}
	getBlockErr := workflow.ExecuteActivity(
		ctx,
		activities.Activities.GetBlock,
		activities.GetBlockParams{
			Height: blockHeight,
		},
	).Get(ctx, &blockResult)

	if getBlockErr != nil {
		e = temporal.NewApplicationErrorWithCause("unable to get block and block params", "GetBlock", getBlockErr)
		return
	}

	blocksPerSession, blocksPerSessionErr := GetBlocksPerSession(blockResult.Params)
	if blocksPerSessionErr != nil {
		e = blocksPerSessionErr
		return
	}

	currentSessionHeight := GetCurrentSession(blockHeight, blocksPerSession)

	// Verify if the relay is able to be dispatched base on the current session height (calculated by the height) and
	// the session height in the params. Also, contemplate the session tolerance, basically how many sessions out it will
	// anyway try to dispatch the relay.
	if !CanHandleRelayWithinTolerance(currentSessionHeight, params.SessionHeight, blocksPerSession, wCtx.App.Config.Rpc.SessionTolerance) {
		e = temporal.NewNonRetryableApplicationError("out of session", "OutOfSession", nil)
		return
	}

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
