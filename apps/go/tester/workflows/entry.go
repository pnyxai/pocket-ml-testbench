package workflows

import (
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"tester/app"
)

// Ctx represents a context struct that holds an instance of `app.App`
// This is created because sharing dependencies by context is not recommended
type Ctx struct {
	App *app.App
}

// Workflows represent the context for executing workflow logic.
var Workflows *Ctx

// SetAppConfig sets the provided app config to the Workflows global variable in the Ctx struct.
func SetAppConfig(ac *app.App) {
	Workflows = &Ctx{
		App: ac,
	}
}

// Register registers the SessionChecker and RelayTester workflows with the provided worker.
func (wCtx *Ctx) Register(w worker.Worker) {
	w.RegisterWorkflowWithOptions(wCtx.SessionChecker, workflow.RegisterOptions{
		Name: SessionCheckerName,
	})
	w.RegisterWorkflowWithOptions(wCtx.RelayTester, workflow.RegisterOptions{
		Name: RelayTesterName,
	})
}
