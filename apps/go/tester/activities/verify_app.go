package activities

import (
	"context"
)

type VerifyAppParams struct {
	Address string `json:"address"`
	Chain   string `json:"chain"`
}

type VerifyAppResults struct {
	// todo: change for a proper type
	App interface{} `json:"app"`
}

func (aCtx *Ctx) VerifyApp(ctx context.Context, params *VerifyAppParams) (*VerifyAppResults, error) {
	result := VerifyAppResults{}
	return &result, nil
}
