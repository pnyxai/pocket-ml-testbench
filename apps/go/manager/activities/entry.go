package activities

import (
	"manager/types"

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
func SetAppConfig(ac *types.App) {
	Activities = &Ctx{
		App: ac,
	}
}

// Register registers a worker activity with the provided activity function in the Ctx struct.
func (aCtx *Ctx) Register(w worker.Worker) {

	w.RegisterActivityWithOptions(aCtx.GetStaked, activity.RegisterOptions{
		Name: GetStakedName,
	})

	w.RegisterActivityWithOptions(aCtx.AnalyzeNode, activity.RegisterOptions{
		Name: AnalyzeNodeName,
	})
}
