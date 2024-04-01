package activities

import (
	"context"
)

type DispatcherParams struct {
}

// Dispatcher is a Boilerplate of what an activity is
func (aCtx *Ctx) Dispatcher(ctx context.Context, param DispatcherParams) (string, error) {
	// aCtx.App is the app config that holds logger base, config object and database connection.
	logger := aCtx.App.GetLoggerByComponent("workflow")
	// todo: remove this line
	logger.Debug().Msg("testing workflow")

	result := "pass"
	return result, nil
}
