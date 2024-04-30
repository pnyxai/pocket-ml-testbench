package tests

import (
	"context"
	"fmt"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"packages/pocket_rpc/samples"
	"reflect"
	"requester/activities"
	"requester/workflows"
)

// define a test suite struct
type RequesterWorkflowUnitTestSuite struct {
	BaseSuite
}

// Test the ideal scenario where we get everything right
func (s *RequesterWorkflowUnitTestSuite) Test_No_Errors() {
	height := int64(0)
	params := workflows.RequesterParams{
		App:     "f3abbe313689a603a1a6d6a43330d0440a552288",
		Service: "0001",
	}
	getAppParams := activities.GetAppParams{
		Address: params.App,
		Service: params.Service,
	}
	getSessionParams := activities.GetSessionParams{
		App:     params.App,
		Service: params.Service,
	}
	dispatchOutput := samples.GetSessionMock(s.app.Logger)
	appMock := samples.GetAppMock(s.app.Logger)
	allParams := samples.GetAllParamsMock(s.app.Logger)
	sessionHeight := int64(dispatchOutput.Session.Header.SessionHeight)
	taskRequestParam := activities.TaskRequest{
		TaskId:     "1",
		InstanceId: "1",
		PromptId:   "1",
	}
	nodesInSession := len(dispatchOutput.Session.Nodes)
	blocksPerSession, _ := workflows.GetBlocksPerSession(allParams)

	temporalClient := &TemporalClientMock{}
	s.app.TemporalClient = temporalClient

	for i := range dispatchOutput.Session.Nodes {
		node := &dispatchOutput.Session.Nodes[i]
		wfId := fmt.Sprintf(
			"%s-%s-%s-%s-%s-%s-%d",
			params.App, node.Address, params.Service,
			taskRequestParam.TaskId, taskRequestParam.InstanceId, taskRequestParam.PromptId,
			sessionHeight,
		)
		wfRunId := fmt.Sprintf("%s-run", wfId)
		relayerRequest := activities.RelayerParams{
			App:     appMock,
			Node:    node,
			Session: dispatchOutput.Session,

			Service:          params.Service,
			SessionHeight:    sessionHeight,
			BlocksPerSession: blocksPerSession,

			PromptId: taskRequestParam.PromptId,
		}
		workflowOptions := client.StartWorkflowOptions{
			// with this format: "app-node-service-taskId-instanceId-promptId-sessionHeight"
			// we are sure that when its workflow runs again inside the same session and the task is still not done,
			// we will not get the same relayer workflow executed twice
			ID:                    wfId,
			WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY,
		}
		mockWorkflowRun := &FakeWorkflowRun{}
		mockWorkflowRun.On("GetID").Return(wfId)
		mockWorkflowRun.On("GetRunID").Return(wfRunId)
		temporalClient.
			On("ExecuteWorkflow", mock.Anything, workflowOptions, mock.Anything, mock.MatchedBy(func(v []interface{}) bool {
				if len(v) == 0 {
					return false
				}
				params, ok := v[0].(activities.RelayerParams)
				if !ok {
					return false
				}
				if params.Service != relayerRequest.Service {
					return false
				}
				if params.SessionHeight != relayerRequest.SessionHeight {
					return false
				}
				if params.BlocksPerSession != relayerRequest.BlocksPerSession {
					return false
				}
				if params.PromptId != relayerRequest.PromptId {
					return false
				}
				if params.Node == nil || !reflect.DeepEqual(params.Node, relayerRequest.Node) {
					return false
				}
				if params.App == nil || !reflect.DeepEqual(params.App, relayerRequest.App) {
					return false
				}
				if params.Session == nil || !reflect.DeepEqual(params.Session, dispatchOutput.Session) {
					return false
				}
				return true
			})).
			Return(mockWorkflowRun, nil).Times(1)
	}

	s.workflowEnv.OnActivity(activities.Activities.GetApp, mock.Anything, getAppParams).
		Return(func(_ context.Context, _ activities.GetAppParams) (*poktGoSdk.App, error) {
			// mock GetApp activity response here
			return appMock, nil
		}).
		Times(1)

	s.workflowEnv.OnActivity(activities.Activities.GetSession, mock.Anything, getSessionParams).
		Return(func(_ context.Context, _ activities.GetSessionParams) (*poktGoSdk.DispatchOutput, error) {
			// mock GetSession activity response here
			return dispatchOutput, nil
		}).
		Times(1)

	s.workflowEnv.OnActivity(activities.Activities.GetBlockParams, mock.Anything, height).
		Return(func(_ context.Context, _ int64) (*poktGoSdk.AllParams, error) {
			// mock GetBlock activity response here
			return allParams, nil
		}).
		Times(1)

	s.workflowEnv.OnActivity(
		activities.Activities.GetTasks,
		mock.Anything,
		mock.MatchedBy(func(param activities.GetTasksParams) bool {
			// check for app and service if they are different no matter about node
			if param.Service != params.Service {
				return false
			}
			// check node is part of the read session
			for _, node := range dispatchOutput.Session.Nodes {
				if param.Node == node.Address {
					return true
				}
			}
			return false
		}),
	).
		Return(func(_ context.Context, _ activities.GetTasksParams) (*activities.GetTaskRequestResults, error) {
			// mock LookupTaskRequest activity response here
			return &activities.GetTaskRequestResults{
				TaskRequests: []activities.TaskRequest{taskRequestParam},
			}, nil
		}).
		Times(nodesInSession)

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.NoError(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
	temporalClient.AssertExpectations(s.T())
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
