package activities

import (
	"context"
)

type RelayerParams struct {
	App           string `json:"app"`
	Node          string `json:"node"`
	Service       string `json:"service"`
	SessionHeight int64  `json:"session_height"`
	TaskId        string `json:"task_id"`
	InstanceId    string `json:"instance_id"`
	PromptId      string `json:"prompt_id"`
}

type RelayerResults struct {
	// response record id
	ResponseId string  `json:"response_id"`
	Success    bool    `json:"success"`
	Code       int     `json:"code"`
	Error      string  `json:"error"`
	Ms         float32 `json:"ms"`
	Retries    int     `json:"retries"`
}

var RelayerName = "relayer"

func (aCtx *Ctx) Relayer(ctx context.Context, params RelayerParams) (*RelayerResults, error) {
	result := RelayerResults{}
	return &result, nil
}
