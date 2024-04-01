package workflows

import (
	"go.temporal.io/sdk/workflow"
	"tester/app"
)

type RelayTesterParams struct {
	Address       string `json:"address"`
	App           string `json:"app"`
	Chain         string `json:"chain"`
	SessionHeight int64  `json:"session_height"`
}

type RelayTesterResults struct {
	Success uint `json:"success"`
	Failed  uint `json:"failed"`
	AvgMs   uint `json:"avg_ms"`
}

var RelayTesterName = "relay_tester"

// RelayTester is a method that performs the relay testing workflow.
// It verifies the session height with height read, loads test requests from Postgres, and performs relay testing for
func (wCtx *Ctx) RelayTester(ctx workflow.Context, param RelayTesterParams) (*RelayTesterResults, error) {
	logger := workflow.GetLogger(ctx)
	// todo: remove this line
	logger.Debug("testing", app.LogFields{"foo": 1, "bar": ""})

	// Activity: verify session height with height read (get height and session height from params)
	// Activity: load test requests from postgres
	// Activities (parallel with future https://github.com/temporalio/samples-go/tree/main/splitmerge-selector)
	// for each test request. This will do the relay and save results on test request record.
	// Merge the results and prepare the result
	// Trigger Evaluation Workflow without waiting for it to
	// Make the results of the workflow available
	result := RelayTesterResults{}

	return &result, nil
}
