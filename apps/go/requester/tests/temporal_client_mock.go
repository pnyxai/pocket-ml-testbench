package tests

import (
	"context"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/client"
)

type TemporalClientMock struct {
	client.Client
	mock.Mock
}

func (m *TemporalClientMock) ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	argsOut := m.Called(ctx, options, workflow, args)
	return argsOut.Get(0).(client.WorkflowRun), argsOut.Error(1)
}
