package activities

import (
	"context"
	"tester/logger"
)

type VerifyAppParams struct {
	Address string `json:"address"`
	Chain   string `json:"chain"`
}

type VerifyAppResults struct {
	// todo: change for a proper type
	App interface{} `json:"app"`
}

var VerifyAppName = "verify_app"

func (aCtx *Ctx) VerifyApp(ctx context.Context, params VerifyAppParams) (*VerifyAppResults, error) {
	// retrieves activity logger
	l := logger.GetActivityLogger(VerifyAppName, ctx, params)
	// todo: remove this line
	l.DebugEvent().Msg("testing activities")

	result := VerifyAppResults{}
	return &result, nil
}
