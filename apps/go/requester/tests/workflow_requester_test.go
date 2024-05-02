package tests

import (
	"context"
	"fmt"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	poktGoSigner "github.com/pokt-foundation/pocket-go/signer"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"packages/pocket_rpc/samples"
	"reflect"
	"requester/activities"
	"requester/common"
	"requester/workflows"
)

// define a test suite struct
type RequesterWorkflowUnitTestSuite struct {
	BaseSuite
}

// Test the ideal scenario where we get everything right
func (s *RequesterWorkflowUnitTestSuite) Test_RequesterWorkflow_No_Errors() {
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
	height := int64(dispatchOutput.BlockHeight)
	appMock := samples.GetAppMock(s.app.Logger)
	allParams := samples.GetAllParamsMock(s.app.Logger)
	sessionHeight := int64(dispatchOutput.Session.Header.SessionHeight)
	nodesInSession := len(dispatchOutput.Session.Nodes)
	blocksPerSession, _ := common.GetBlocksPerSession(allParams)

	temporalClient := s.GetTemporalClientMock()

	for i := range dispatchOutput.Session.Nodes {
		node := &dispatchOutput.Session.Nodes[i]
		wfId := fmt.Sprintf(
			"%s-%s-%s-%s-%s-%s-%d",
			params.App, node.Address, params.Service,
			node.Address, node.Address, node.Address,
			sessionHeight,
		)
		wfRunId := wfId
		relayerRequest := activities.RelayerParams{
			App:     appMock,
			Node:    node,
			Session: dispatchOutput.Session,

			Service:          params.Service,
			SessionHeight:    sessionHeight,
			BlocksPerSession: blocksPerSession,

			PromptId: primitive.NewObjectID().Hex(),
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
				p, ok := v[0].(activities.RelayerParams)
				if !ok {
					return false
				}
				if p.Service != relayerRequest.Service {
					return false
				}
				if p.SessionHeight != relayerRequest.SessionHeight {
					return false
				}
				if p.BlocksPerSession != relayerRequest.BlocksPerSession {
					return false
				}
				if p.PromptId == "" {
					return false
				}
				if p.Node == nil || !reflect.DeepEqual(p.Node, relayerRequest.Node) {
					return false
				}
				if p.App == nil || !reflect.DeepEqual(p.App, relayerRequest.App) {
					return false
				}
				if p.Session == nil || !reflect.DeepEqual(p.Session, dispatchOutput.Session) {
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

	s.workflowEnv.OnActivity(activities.Activities.GetBlockParams, mock.Anything, int64(0)).
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
		Return(func(_ context.Context, p activities.GetTasksParams) (*activities.GetTaskRequestResults, error) {
			// mock LookupTaskRequest activity response here
			return &activities.GetTaskRequestResults{
				TaskRequests: []activities.TaskRequest{{
					TaskId:     p.Node,
					InstanceId: p.Node,
					PromptId:   p.Node,
				}},
			}, nil
		}).
		Times(nodesInSession)

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.NoError(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
	temporalClient.AssertExpectations(s.T())
	r := workflows.RequesterResults{}
	s.NoError(s.workflowEnv.GetWorkflowResult(&r))
	s.Equal(height, r.Height)
	s.Equal(sessionHeight, r.SessionHeight)
	s.Equal(params.App, r.App)
	s.Equal(params.Service, r.Service)
	// here we are simulating that we have pending tasks for every node in the session
	s.Equal(nodesInSession, len(r.Nodes))
	s.Equal(nodesInSession, len(r.TriggeredWorkflows))
	s.Equal(0, len(r.SkippedWorkflows))
}

func (s *RequesterWorkflowUnitTestSuite) Test_RequesterWorkflow_No_Errors_FewNodes() {
	subsetOfNodes := 3
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
	height := int64(dispatchOutput.BlockHeight)
	appMock := samples.GetAppMock(s.app.Logger)
	allParams := samples.GetAllParamsMock(s.app.Logger)
	sessionHeight := int64(dispatchOutput.Session.Header.SessionHeight)
	nodesInSession := len(dispatchOutput.Session.Nodes)
	blocksPerSession, _ := common.GetBlocksPerSession(allParams)
	firstThreeNodes := dispatchOutput.Session.Nodes[:subsetOfNodes]
	temporalClient := s.GetTemporalClientMock()

	// mock temporal client to hold only a subset of nodes calls
	for i := range firstThreeNodes {
		node := &firstThreeNodes[i]
		wfId := fmt.Sprintf(
			"%s-%s-%s-%s-%s-%s-%d",
			params.App, node.Address, params.Service,
			node.Address, node.Address, node.Address,
			sessionHeight,
		)
		wfRunId := wfId
		relayerRequest := activities.RelayerParams{
			App:     appMock,
			Node:    node,
			Session: dispatchOutput.Session,

			Service:          params.Service,
			SessionHeight:    sessionHeight,
			BlocksPerSession: blocksPerSession,

			PromptId: primitive.NewObjectID().Hex(),
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
				p, ok := v[0].(activities.RelayerParams)
				if !ok {
					return false
				}
				if p.Service != relayerRequest.Service {
					return false
				}
				if p.SessionHeight != relayerRequest.SessionHeight {
					return false
				}
				if p.BlocksPerSession != relayerRequest.BlocksPerSession {
					return false
				}
				if p.PromptId == "" {
					return false
				}
				if p.Node == nil || !reflect.DeepEqual(p.Node, relayerRequest.Node) {
					return false
				}
				if p.App == nil || !reflect.DeepEqual(p.App, relayerRequest.App) {
					return false
				}
				if p.Session == nil || !reflect.DeepEqual(p.Session, dispatchOutput.Session) {
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

	s.workflowEnv.OnActivity(activities.Activities.GetBlockParams, mock.Anything, int64(0)).
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
		Return(func(_ context.Context, p activities.GetTasksParams) (*activities.GetTaskRequestResults, error) {
			for i := range firstThreeNodes {
				if firstThreeNodes[i].Address == p.Node {
					// if not found, returns empty, so the activity is ok, but the workflow should not get executed
					return &activities.GetTaskRequestResults{
						TaskRequests: []activities.TaskRequest{{
							TaskId:     p.Node,
							InstanceId: p.Node,
							PromptId:   p.Node,
						}},
					}, nil
				}
			}
			return &activities.GetTaskRequestResults{
				TaskRequests: make([]activities.TaskRequest, 0),
			}, nil
		}).
		Times(nodesInSession)

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.NoError(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
	temporalClient.AssertExpectations(s.T())
	r := workflows.RequesterResults{}
	s.NoError(s.workflowEnv.GetWorkflowResult(&r))
	s.Equal(height, r.Height)
	s.Equal(sessionHeight, r.SessionHeight)
	s.Equal(params.App, r.App)
	s.Equal(params.Service, r.Service)
	// here we are simulating that we have pending tasks for every node in the session
	s.Equal(subsetOfNodes, len(r.Nodes))
	s.Equal(subsetOfNodes, len(r.TriggeredWorkflows))
	s.Equal(0, len(r.SkippedWorkflows))
}

func (s *RequesterWorkflowUnitTestSuite) Test_RequesterWorkflow_Fail_ApplicationNotFound() {
	s.app.SignerByAddress = xsync.NewMapOf[string, *poktGoSigner.Signer]()
	params := workflows.RequesterParams{
		App:     "f3abbe313689a603a1a6d6a43330d0440a552288",
		Service: "0001",
	}

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.Error(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
}

func (s *RequesterWorkflowUnitTestSuite) Test_RequesterWorkflow_Fail_GetApp() {
	params := workflows.RequesterParams{
		App:     "f3abbe313689a603a1a6d6a43330d0440a552288",
		Service: "0001",
	}
	getAppParams := activities.GetAppParams{
		Address: params.App,
		Service: params.Service,
	}

	s.workflowEnv.OnActivity(activities.Activities.GetApp, mock.Anything, getAppParams).
		Return(func(_ context.Context, _ activities.GetAppParams) (*poktGoSdk.App, error) {
			return nil, temporal.NewApplicationError("unable to get app", "GetApp", nil)
		}).
		Times(1)

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.Error(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
}

func (s *RequesterWorkflowUnitTestSuite) Test_RequesterWorkflow_Fail_GetSession() {
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
	appMock := samples.GetAppMock(s.app.Logger)

	s.workflowEnv.OnActivity(activities.Activities.GetApp, mock.Anything, getAppParams).
		Return(func(_ context.Context, _ activities.GetAppParams) (*poktGoSdk.App, error) {
			// mock GetApp activity response here
			return appMock, nil
		}).
		Times(1)

	s.workflowEnv.OnActivity(activities.Activities.GetSession, mock.Anything, getSessionParams).
		Return(func(_ context.Context, _ activities.GetSessionParams) (*poktGoSdk.DispatchOutput, error) {
			return nil, temporal.NewApplicationError("unable to get session", "GetSession", nil)
		}).
		Times(1)

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.Error(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
}

func (s *RequesterWorkflowUnitTestSuite) Test_RequesterWorkflow_Fail_GetBlockParams() {
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

	s.workflowEnv.OnActivity(activities.Activities.GetApp, mock.Anything, getAppParams).
		Return(func(_ context.Context, _ activities.GetAppParams) (*poktGoSdk.App, error) {
			// mock GetApp activity response here
			return appMock, nil
		}).
		Times(1)

	s.workflowEnv.OnActivity(activities.Activities.GetSession, mock.Anything, getSessionParams).
		Return(func(_ context.Context, _ activities.GetSessionParams) (*poktGoSdk.DispatchOutput, error) {
			return dispatchOutput, nil
		}).
		Times(1)

	s.workflowEnv.OnActivity(activities.Activities.GetBlockParams, mock.Anything, height).
		Return(func(_ context.Context, _ int64) (*poktGoSdk.AllParams, error) {
			return nil, temporal.NewApplicationError("unable to get all params", "GetAllParams", nil)
		}).
		Times(1)

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.Error(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
}

func (s *RequesterWorkflowUnitTestSuite) Test_RequesterWorkflow_Fail_LookupTaskRequest() {
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
	nodesInSession := len(dispatchOutput.Session.Nodes)

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
		mock.Anything,
	).
		Return(func(_ context.Context, _ activities.GetTasksParams) (*activities.GetTaskRequestResults, error) {
			return nil, temporal.NewApplicationErrorWithCause("unable to find tasks on database", "Database", nil)
		}).
		Times(nodesInSession)

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.Error(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
}

func (s *RequesterWorkflowUnitTestSuite) Test_RequesterWorkflow_Zero_Nodes_In_Session() {
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
	nodesInSession := len(dispatchOutput.Session.Nodes)

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
			return &activities.GetTaskRequestResults{
				TaskRequests: make([]activities.TaskRequest, 0),
			}, nil
		}).
		Times(nodesInSession)

	s.workflowEnv.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.workflowEnv.IsWorkflowCompleted())
	s.NoError(s.workflowEnv.GetWorkflowError())
	s.workflowEnv.AssertExpectations(s.T())
}
