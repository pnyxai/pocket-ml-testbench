package activities

import (
	"context"
)

type RelayerParams struct {
	App        string `json:"app"`
	Node       string `json:"node"`
	Service    string `json:"service"`
	TaskId     string `json:"task_id"`
	InstanceId string `json:"instance_id"`
	PromptId   string `json:"prompt_id"`
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
	// get_height (function)
	// get_params (function)
	// with both verify if we are in session
	// use function that is use on geo-mesh to calculate it
	// todo: check if we can as config add the "extra" block window like on geo-mesh

	result := RelayerResults{}
	return &result, nil
}
