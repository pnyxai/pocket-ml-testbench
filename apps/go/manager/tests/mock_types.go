package tests

import (
	"context"
	"packages/pocket_rpc/samples"
	"packages/utils"
	"strconv"

	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/client"
)

type MockRelayData struct {
	Height           int64
	BlockParams      *poktGoSdk.AllParams
	Session          *poktGoSdk.Session
	App              *poktGoSdk.App
	Node             *poktGoSdk.Node
	Service          string
	SessionHeight    int64
	BlocksPerSession int64
	RelayResponse    *poktGoSdk.RelayOutput
}


type TemporalClientMock struct {
	client.Client
	mock.Mock
}

func (m *TemporalClientMock) ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	argsOut := m.Called(ctx, options, workflow, args)
	return argsOut.Get(0).(client.WorkflowRun), argsOut.Error(1)
}

type FakeWorkflowRun struct {
	mock.Mock
	client.WorkflowRun
}

func (w *FakeWorkflowRun) GetID() string {
	return w.Called().String(0)
}

func (w *FakeWorkflowRun) GetRunID() string {
	return w.Called().String(0)
}

func GetRelayMockData(l *zerolog.Logger) *MockRelayData {
	height, _ := samples.GetHeightMock(l).Height.Int64()
	blockParams := samples.GetAllParamsMock(l)
	session := samples.GetSessionMock(l).Session
	app := samples.GetAppMock(l)
	node := utils.GetRandomFromSlice[poktGoSdk.Node](session.Nodes)
	service := *utils.GetRandomFromSlice[string](app.Chains)
	sessionHeight := int64(session.Header.SessionHeight)
	blocksPerSessionRaw, _ := blockParams.NodeParams.Get("pos/BlocksPerSession")
	blocksPerSession, _ := strconv.ParseInt(blocksPerSessionRaw, 10, 64)
	relayResponse := samples.GetSuccessRelayOutput(l)

	return &MockRelayData{
		Height:           height,
		BlockParams:      blockParams,
		Session:          session,
		App:              app,
		Node:             node,
		Service:          service,
		SessionHeight:    sessionHeight,
		BlocksPerSession: blocksPerSession,
		RelayResponse:    relayResponse,
	}

}
