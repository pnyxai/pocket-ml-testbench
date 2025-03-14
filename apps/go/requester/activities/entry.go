package activities

import (
	"requester/types"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
)

// Ctx represents a context struct that holds an instance of `app.App`
// This is created because sharing dependencies by context is not recommended
type Ctx struct {
	App *types.App
}

// Activities represent the context for executing activities logic.
var Activities *Ctx

// SetAppConfig sets the provided app configuration to the global Activities variable in the Ctx struct.
func SetAppConfig(ac *types.App) *Ctx {
	if Activities != nil {
		Activities.App = ac
	} else {
		Activities = &Ctx{
			App: ac,
		}
	}
	return Activities
}

// Register registers a worker activity with the provided activity function in the Ctx struct.
func (aCtx *Ctx) Register(w worker.Worker) {
	w.RegisterActivityWithOptions(aCtx.GetApp, activity.RegisterOptions{
		Name: GetAppName,
	})

	w.RegisterActivityWithOptions(aCtx.GetHeight, activity.RegisterOptions{
		Name: GetHeightName,
	})

	w.RegisterActivityWithOptions(aCtx.GetBlockParams, activity.RegisterOptions{
		Name: GetBlockParamsName,
	})

	w.RegisterActivityWithOptions(aCtx.GetSession, activity.RegisterOptions{
		Name: GetSessionName,
	})

	w.RegisterActivityWithOptions(aCtx.GetTasks, activity.RegisterOptions{
		Name: GetTasksName,
	})

	w.RegisterActivityWithOptions(aCtx.SetPromptTriggerSession, activity.RegisterOptions{
		Name: SetPromptTriggerSessionName,
	})

	w.RegisterActivityWithOptions(aCtx.Relayer, activity.RegisterOptions{
		Name: RelayerName,
	})

	w.RegisterActivityWithOptions(aCtx.UpdateTaskTree, activity.RegisterOptions{
		Name: UpdateTaskTreeName,
	})
}
