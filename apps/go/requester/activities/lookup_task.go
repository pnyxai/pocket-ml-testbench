package activities

import (
	"context"
)

type LookupTaskRequestParams struct {
	// Pass a 0 to get the latest
	Node string `json:"node"`
	App  string `json:"app"`
	// chain (morse) service (shannon)
	Service string `json:"service"`
}

type CompactTaskRequest struct {
	TaskId     string `json:"task_id"`
	InstanceId string `json:"instance_id"`
	PromptId   string `json:"prompt_id"`
}

type LookupTaskRequestResults struct {
	TaskRequests []CompactTaskRequest `json:"task_requests"`
}

var LookupTaskRequestName = "lookup_task_request"

func (aCtx *Ctx) LookupTaskRequest(ctx context.Context, params LookupTaskRequestParams) (*LookupTaskRequestResults, error) {
	result := LookupTaskRequestResults{
		TaskRequests: make([]CompactTaskRequest, 0),
	}
	return &result, nil
}
