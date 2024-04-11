package activities

import (
	"context"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"packages/logger"
)

type GetAppParams struct {
	Address string `json:"address"`
	Service string `json:"service"`
}

var GetAppName = "get_app"

func (aCtx *Ctx) GetApp(ctx context.Context, params GetAppParams) (*poktGoSdk.App, error) {
	l := logger.GetActivityLogger(GetAppName, ctx, logger.NewFieldsFromStruct(params))
	app, err := aCtx.App.PocketRpc.GetApp(params.Address)
	if err != nil {
		return nil, err
	}
	l.Info("GetApp", logger.NewFieldsFromStruct(app).GetLoggerFields()...)
	return app, nil
}
