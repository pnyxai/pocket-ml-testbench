package tests

import (
	"context"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"github.com/stretchr/testify/mock"
	"packages/pocket_rpc/samples"
	"requester/activities"
	"requester/workflows"
)

// define a test suite struct
type RequesterWorkflowUnitTestSuite struct {
	BaseSuite
}

// Test the ideal scenario where we get everything right
func (s *RequesterWorkflowUnitTestSuite) Test_No_Errors() {
	params := workflows.RequesterParams{
		App:     "f3abbe313689a603a1a6d6a43330d0440a552288",
		Service: "0001",
	}
	getAppParams := activities.GetAppParams{
		Address: params.App,
		Service: params.Service,
	}
	getBlockParams := activities.GetBlockParams{
		Height: 0,
	}
	getSessionParams := activities.GetSessionParams{
		App:     params.App,
		Service: params.Service,
	}
	dispatchOutput := samples.GetSessionMock(s.app.Logger)
	taskRequestParam := activities.CompactTaskRequest{
		TaskId:     "1",
		InstanceId: "1",
		PromptId:   "1",
	}
	nodesInSession := len(dispatchOutput.Session.Nodes)

	s.workflowEnv.OnActivity(activities.Activities.GetApp, mock.Anything, getAppParams).
		Return(func(_ context.Context, _ activities.GetAppParams) (*poktGoSdk.App, error) {
			// mock GetApp activity response here
			return samples.GetAppMock(s.app.Logger), nil
		}).
		Times(1)

	s.workflowEnv.OnActivity(activities.Activities.GetBlock, mock.Anything, getBlockParams).
		Return(func(_ context.Context, _ activities.GetBlockParams) (*activities.GetBlockResults, error) {
			// mock GetBlock activity response here
			return &activities.GetBlockResults{
				Block:  samples.GetBlockMock(s.app.Logger),
				Params: samples.GetAllParamsMock(s.app.Logger),
			}, nil
		}).
		Times(1)

	s.workflowEnv.OnActivity(activities.Activities.GetSession, mock.Anything, getSessionParams).
		Return(func(_ context.Context, _ activities.GetSessionParams) (*poktGoSdk.DispatchOutput, error) {
			// mock GetSession activity response here
			return dispatchOutput, nil
		}).
		Times(1)

	s.workflowEnv.OnActivity(
		activities.Activities.LookupTaskRequest,
		mock.Anything,
		mock.MatchedBy(func(param activities.LookupTaskRequestParams) bool {
			// check for app and service if they are different no matter about node
			appAndService := param.App == params.App && param.Service == params.Service

			if !appAndService {
				return appAndService
			}

			for _, node := range dispatchOutput.Session.Nodes {
				if param.Node == node.Address {
					return true
				}
			}

			return false
		}),
	).
		Return(func(_ context.Context, _ activities.LookupTaskRequestParams) (*activities.LookupTaskRequestResults, error) {
			// mock LookupTaskRequest activity response here
			return &activities.LookupTaskRequestResults{
				TaskRequests: []activities.CompactTaskRequest{taskRequestParam},
			}, nil
		}).
		Times(nodesInSession)

	s.workflowEnv.OnActivity(
		activities.Activities.Relayer, mock.Anything,
		mock.MatchedBy(func(param activities.RelayerParams) bool {
			// check for app, service, taskId, instanceId and promptId if they are different no matter about node
			appAndService := param.App == params.App &&
				param.Service == params.Service &&
				param.TaskId == taskRequestParam.TaskId &&
				param.InstanceId == taskRequestParam.InstanceId &&
				param.PromptId == taskRequestParam.PromptId

			if !appAndService {
				return appAndService
			}

			for _, node := range dispatchOutput.Session.Nodes {
				if param.Node == node.Address {
					return true
				}
			}

			return false
		}),
	).
		Return(func(_ context.Context, _ activities.RelayerParams) (*activities.RelayerResults, error) {
			// mock Relay activity response here
			return &activities.RelayerResults{
				ResponseId: "1",
				Success:    true,
				Code:       0,
				Error:      "",
				Ms:         10,
				Retries:    0,
			}, nil
		}).
		Times(nodesInSession)

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.NoError(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
}

func (s *RequesterWorkflowUnitTestSuite) Test_Fail_GetApp() {
	// Could be:
	// 1. Missing a private key for retrieved app address
	// 2. App does not have service staked
	// 3. App not found on rpc provider
}
func (s *RequesterWorkflowUnitTestSuite) Test_Fail_GetBlock() {
	// Could be:
	// 1. rpc error
}
func (s *RequesterWorkflowUnitTestSuite) Test_Fail_GetSession() {
	// Could be:
	// 1. rpc error
}
func (s *RequesterWorkflowUnitTestSuite) Test_Fail_LookupTaskRequest() {
	// Could be:
	// 1. database error
}
func (s *RequesterWorkflowUnitTestSuite) Test_Fail_Relayer() {
	// Could be:
	// 1. database error
	// 2. any other pocket hashing error?
}

func (s *RequesterWorkflowUnitTestSuite) Test_Zero_Nodes_In_Session() {
	// everything is ok, but there is any task request for the nodes in the retrieved session
}
