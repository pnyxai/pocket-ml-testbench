package workflows

import (
	"go.temporal.io/sdk/workflow"
	"tester/logger"
)

type SessionCheckerParams struct {
	App   string `json:"app"`
	Chain string `json:"chain"`
}

type SessionCheckerNodeResults struct {
	Address string  `json:"address"`
	Relays  uint    `json:"relays"`
	Success uint    `json:"success"`
	Failed  uint    `json:"failed"`
	AvgMs   float32 `json:"avg_ms"`
}

type SessionCheckerResults struct {
	App           string `json:"app"`
	Chain         string `json:"chain"`
	SessionHeight int64  `json:"session_height"`
	Nodes         []SessionCheckerNodeResults
}

var SessionCheckerName = "session_checker"

// SessionChecker checks the session and trigger relay_tester workflow.
func (wCtx *Ctx) SessionChecker(ctx workflow.Context, params SessionCheckerParams) (*SessionCheckerResults, error) {
	l := logger.GetWorkflowLogger(SessionCheckerName, ctx, params)
	// todo: remove this line
	l.DebugEvent().Msg("testing workflows")

	// activity: verify app + chain from params
	// if not ok: exit
	// if ok:
	// activities (parallel):
	// 1. get block
	// 2. get params
	// 3. lookup test requests for session.nodes
	// when 0 test requests: exit
	// when 1+ test requests:
	// call relay_tester workflow per each node as child workflow (calculate timeout dynamically base on amount of relays * base relay timeout):
	// merge results and return SessionCheckerResults
	// Make the results of the workflow available
	result := SessionCheckerResults{}

	return &result, nil
}
