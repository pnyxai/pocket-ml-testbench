package tests

import (
	"context"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"net/http"
	"packages/pocket_rpc"
	"packages/pocket_rpc/samples"
	"packages/utils"
	"requester/activities"
	"requester/common"
	"requester/types"
	"requester/workflows"
	"time"
)

// define a test suite struct
type RelayerWorkflowUnitTestSuite struct {
	BaseSuite
}

func (s *RelayerWorkflowUnitTestSuite) Test_RelayerWorkflow_AllGood() {
	relayResponse := samples.GetSuccessRelayOutput(s.app.Logger)
	session := samples.GetSessionMock(s.app.Logger).Session
	blockParams := samples.GetAllParamsMock(s.app.Logger)
	app := samples.GetAppMock(s.app.Logger)
	node := utils.GetRandomFromSlice[poktGoSdk.Node](session.Nodes)
	service := *utils.GetRandomFromSlice[string](app.Chains)
	sessionHeight := int64(session.Header.SessionHeight)
	blocksPerSession, _ := common.GetBlocksPerSession(blockParams)
	relayDelay := time.Duration(100) * time.Millisecond
	mockReqRes := MockHttpReqRes{
		Route:   pocket_rpc.ClientRelayRoute,
		Method:  http.MethodPost,
		Data:    relayResponse,
		GetData: nil,
		Code:    http.StatusOK,
		Delay:   &relayDelay,
	}
	_, mockServerUrl := mockReqRes.NewMockServer(s.T())
	node.ServiceURL = mockServerUrl
	task := types.Task{
		Id: primitive.NewObjectID(),
		RequesterArgs: types.RequesterArgs{
			Address: node.Address,
			Service: service,
			Method:  "GET",
			Path:    "/test",
		},
		Done: false,
	}
	prompt := types.Prompt{
		Id:      primitive.NewObjectID(),
		Data:    "{\"data\":\"test\"}",
		Timeout: 10,
		Done:    false,
		TaskId:  task.Id,
		Task:    &task,
	}
	relayerParams := activities.RelayerParams{
		Session:          session,
		Node:             node,
		App:              app,
		Service:          service,
		SessionHeight:    sessionHeight,
		BlocksPerSession: blocksPerSession,
		PromptId:         prompt.Id.Hex(),
		RelayTimeout:     10,
	}
	relayerResponse := activities.RelayerResponse{ResponseId: primitive.NewObjectID().Hex()}
	s.workflowEnv.OnActivity(activities.Activities.Relayer, mock.Anything, relayerParams).
		Return(func(_ context.Context, _ activities.RelayerParams) (activities.RelayerResponse, error) {
			return relayerResponse, nil
		}).
		Times(1)
	updateTaskTreeParams := activities.UpdateTaskTreeRequest{PromptId: prompt.Id.Hex()}
	updateTaskTreeResponse := activities.UpdateTaskTreeResponse{TaskId: task.Id.Hex(), IsDone: true}
	s.workflowEnv.OnActivity(activities.Activities.UpdateTaskTree, mock.Anything, updateTaskTreeParams).
		Return(func(_ context.Context, _ activities.UpdateTaskTreeRequest) (*activities.UpdateTaskTreeResponse, error) {
			return &updateTaskTreeResponse, nil
		}).
		Times(1)

	mockWorkflowRun := &FakeWorkflowRun{}
	mockTemporalClient := s.GetTemporalClientMock()
	evaluatorWorkflowOptions := client.StartWorkflowOptions{
		ID:                    task.Id.Hex(),
		TaskQueue:             s.app.Config.Temporal.Evaluator.TaskQueue,
		WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
	}
	evaluatorParams := workflows.EvaluatorWorkflowParams{TaskId: task.Id.Hex()}
	mockTemporalClient.
		On(
			"ExecuteWorkflow", mock.Anything,
			evaluatorWorkflowOptions, s.app.Config.Temporal.Evaluator.WorkflowName, mock.MatchedBy(func(v []interface{}) bool {
				if len(v) == 0 {
					return false
				}
				params, ok := v[0].(workflows.EvaluatorWorkflowParams)
				if !ok {
					return false
				}
				if params.TaskId != evaluatorParams.TaskId {
					return false
				}
				return true
			}),
		).
		Return(mockWorkflowRun, nil).Times(1)

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Relayer, relayerParams)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.NoError(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
	mockTemporalClient.AssertExpectations(s.T())
}

func (s *RelayerWorkflowUnitTestSuite) Test_RelayerWorkflow_Fail_AppNotFound() {
	relayResponse := samples.GetSuccessRelayOutput(s.app.Logger)
	session := samples.GetSessionMock(s.app.Logger).Session
	blockParams := samples.GetAllParamsMock(s.app.Logger)
	app := samples.GetAppMock(s.app.Logger)
	node := utils.GetRandomFromSlice[poktGoSdk.Node](session.Nodes)
	service := *utils.GetRandomFromSlice[string](app.Chains)
	sessionHeight := int64(session.Header.SessionHeight)
	blocksPerSession, _ := common.GetBlocksPerSession(blockParams)
	relayDelay := time.Duration(100) * time.Millisecond
	mockReqRes := MockHttpReqRes{
		Route:   pocket_rpc.ClientRelayRoute,
		Method:  http.MethodPost,
		Data:    relayResponse,
		GetData: nil,
		Code:    http.StatusOK,
		Delay:   &relayDelay,
	}
	_, mockServerUrl := mockReqRes.NewMockServer(s.T())
	node.ServiceURL = mockServerUrl
	task := types.Task{
		Id: primitive.NewObjectID(),
		RequesterArgs: types.RequesterArgs{
			Address: node.Address,
			Service: service,
			Method:  "GET",
			Path:    "/test",
		},
		Done: false,
	}
	prompt := types.Prompt{
		Id:      primitive.NewObjectID(),
		Data:    "{\"data\":\"test\"}",
		Timeout: 10000,
		Done:    false,
		TaskId:  task.Id,
		Task:    &task,
	}
	relayerParams := activities.RelayerParams{
		Session:          session,
		Node:             node,
		App:              app,
		Service:          service,
		SessionHeight:    sessionHeight,
		BlocksPerSession: blocksPerSession,
		PromptId:         prompt.Id.Hex(),
	}
	s.app.AppAccounts = xsync.NewMapOf[string, *types.AppAccount]()

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Relayer, relayerParams)
	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.Error(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
}
