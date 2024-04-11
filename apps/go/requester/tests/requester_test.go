package tests_test

import (
	"context"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"os"
	"packages/logger"
	"requester/tests/samples"
	"requester/types"
	"testing"
	"time"

	"requester/activities"
	"requester/workflows"

	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

// define a test suite struct
type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env *testsuite.TestWorkflowEnvironment
	app *types.App
}

func (s *UnitTestSuite) InitializeTestApp() {
	// mock App config
	cfg := &types.Config{
		MongodbUri: types.DefaultMongodbUri,
		Apps:       []string{},
		Rpc: &types.RPCConfig{
			Urls:       []string{},
			Retries:    0,
			MinBackoff: 1,
			MaxBackoff: 10,
			ReqPerSec:  1,
		},
		LogLevel: types.DefaultLogLevel,
		Temporal: &types.TemporalConfig{
			Host:      types.DefaultTemporalHost,
			Port:      types.DefaultTemporalPort,
			Namespace: types.DefaultTemporalNamespace,
			TaskQueue: types.DefaultTemporalTaskQueue,
		},
	}
	// initialize logger
	l := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).Level(zerolog.NoLevel).With().Timestamp().Logger()

	zeroLoggerAdapter := logger.NewZerologAdapter(l)

	s.SetLogger(zeroLoggerAdapter)

	ac := &types.App{
		Logger: &l,
		Config: cfg,
	}

	// set the app to test env
	s.app = ac

	return
}

// initialize the test suite
func (s *UnitTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()

	s.InitializeTestApp()

	// set this to workflows and activities to avoid use of context.Context
	workflows.SetAppConfig(s.app)
	activities.SetAppConfig(s.app)

	// register the activities that will need to be mock up here
	s.env.RegisterActivity(activities.Activities.GetApp)
	s.env.RegisterActivity(activities.Activities.GetBlock)
	s.env.RegisterActivity(activities.Activities.GetSession)
	s.env.RegisterActivity(activities.Activities.LookupTaskRequest)
	s.env.RegisterActivity(activities.Activities.Relayer)

	// register the workflows
	s.env.RegisterWorkflow(workflows.Workflows.Requester)
}

// Test the ideal scenario where we get everything right
func (s *UnitTestSuite) Test_No_Errors() {
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
	dispatchOutput := samples.GetDispatchMock(s.app.Logger)
	taskRequestParam := activities.CompactTaskRequest{
		TaskId:     "1",
		InstanceId: "1",
		PromptId:   "1",
	}
	nodesInSession := len(dispatchOutput.Session.Nodes)

	s.env.OnActivity(activities.Activities.GetApp, mock.Anything, getAppParams).
		Return(func(_ context.Context, _ activities.GetAppParams) (*poktGoSdk.App, error) {
			// mock GetApp activity response here
			return samples.GetAppMock(s.app.Logger), nil
		}).
		Times(1)

	s.env.OnActivity(activities.Activities.GetBlock, mock.Anything, getBlockParams).
		Return(func(_ context.Context, _ activities.GetBlockParams) (*activities.GetBlockResults, error) {
			// mock GetBlock activity response here
			return &activities.GetBlockResults{
				Block:  samples.GetBlockMock(s.app.Logger),
				Params: samples.GetAllParamsMock(s.app.Logger),
			}, nil
		}).
		Times(1)

	s.env.OnActivity(activities.Activities.GetSession, mock.Anything, getSessionParams).
		Return(func(_ context.Context, _ activities.GetSessionParams) (*poktGoSdk.DispatchOutput, error) {
			// mock GetSession activity response here
			return dispatchOutput, nil
		}).
		Times(1)

	s.env.OnActivity(
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

	s.env.OnActivity(
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

	s.env.ExecuteWorkflow(workflows.Workflows.Requester, params)

	s.True(s.env.IsWorkflowCompleted())
	s.NoError(s.env.GetWorkflowError())
	s.env.AssertExpectations(s.T())
}

func (s *UnitTestSuite) Test_Fail_GetApp() {
	// Could be:
	// 1. Missing a private key for retrieved app address
	// 2. App does not have service staked
	// 3. App not found on rpc provider
}
func (s *UnitTestSuite) Test_Fail_GetBlock() {
	// Could be:
	// 1. rpc error
}
func (s *UnitTestSuite) Test_Fail_GetSession() {
	// Could be:
	// 1. rpc error
}
func (s *UnitTestSuite) Test_Fail_LookupTaskRequest() {
	// Could be:
	// 1. database error
}
func (s *UnitTestSuite) Test_Fail_Relayer() {
	// Could be:
	// 1. database error
	// 2. any other pocket hashing error?
}

func (s *UnitTestSuite) Test_Zero_Nodes_In_Session() {
	// everything is ok, but there is any task request for the nodes in the retrieved session
}

func TestUnitTestSuite(t *testing.T) {
	// run all the tests
	suite.Run(t, new(UnitTestSuite))
}
