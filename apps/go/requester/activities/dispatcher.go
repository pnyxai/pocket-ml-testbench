package activities

import (
	"context"
	"packages/logger"
)

type DispatcherParams struct {
}

var DispatcherName = "dispatcher"

// Dispatcher is a Boilerplate of what an activity is
func (aCtx *Ctx) Dispatcher(ctx context.Context, params DispatcherParams) (string, error) {
	// aCtx.App is the app config that holds logger base, config object and database connection.
	l := logger.GetActivityLogger(DispatcherName, ctx, params)
	// todo: remove this line
	l.DebugEvent().Msg("testing activities")

	result := "pass"
	return result, nil
}
