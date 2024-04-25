package workflows

import (
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"requester/types"
)

// Ctx represents a context struct that holds an instance of `app.App`
// This is created because sharing dependencies by context is not recommended
type Ctx struct {
	App *types.App
}

// Workflows represent the context for executing workflow logic.
var Workflows *Ctx

// SetAppConfig sets the provided app config to the Workflows global variable in the Ctx struct.
func SetAppConfig(ac *types.App) *Ctx {
	if Workflows != nil {
		Workflows.App = ac
	} else {
		Workflows = &Ctx{
			App: ac,
		}
	}
	return Workflows
}

// Register registers the SessionChecker and RelayTester workflows with the provided worker.
func (wCtx *Ctx) Register(w worker.Worker) {
	w.RegisterWorkflowWithOptions(wCtx.Requester, workflow.RegisterOptions{
		Name: RequesterName,
	})
}
