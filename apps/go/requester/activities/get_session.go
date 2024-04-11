package activities

import (
	"context"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
)

type GetSessionParams struct {
	App     string `json:"app"`
	Service string `json:"service"`
}

var GetSessionName = "get_session"

func (aCtx *Ctx) GetSession(ctx context.Context, params GetSessionParams) (*poktGoSdk.DispatchOutput, error) {
	result := poktGoSdk.DispatchOutput{}
	return &result, nil
}
