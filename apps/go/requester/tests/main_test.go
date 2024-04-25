package tests

import (
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"os"
	"packages/logger"
	"packages/pocket_rpc"
	"packages/pocket_rpc/samples"
	"path"
	"requester/activities"
	"requester/types"
	"requester/workflows"
	"testing"
	"time"
)

type BaseSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	workflowEnv *testsuite.TestWorkflowEnvironment
	activityEnv *testsuite.TestActivityEnvironment

	app     *types.App
	mockRpc *pocket_rpc.MockRpc
}

func (s *BaseSuite) InitializeTestApp() {
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

	ac := &types.App{
		Logger:    &l,
		Config:    cfg,
		PocketRpc: s.mockRpc,
	}

	// set the app to test env
	s.app = ac

	samples.SetBasePath(path.Join("..", "..", "..", "..", "packages", "go", "pocket_rpc", "samples"))

	return
}

func (s *BaseSuite) BeforeTest(_, _ string) {
	s.mockRpc = pocket_rpc.NewMockRpc()

	s.InitializeTestApp()

	zeroLoggerAdapter := logger.NewZerologAdapter(*s.app.Logger)
	s.SetLogger(zeroLoggerAdapter)

	// set this to workflows and activities to avoid use of context.Context
	wCtx := workflows.SetAppConfig(s.app)
	aCtx := activities.SetAppConfig(s.app)

	s.workflowEnv = s.NewTestWorkflowEnvironment()
	s.activityEnv = s.NewTestActivityEnvironment()

	var activityList = []interface{}{
		aCtx.GetApp,
		aCtx.GetBlock,
		aCtx.GetSession,
		aCtx.LookupTaskRequest,
		aCtx.Relayer,
	}

	// register the activities that will need to be mock up here
	for i := range activityList {
		s.activityEnv.RegisterActivity(activityList[i])
		s.workflowEnv.RegisterActivity(activityList[i])
	}

	// register the workflows
	s.workflowEnv.RegisterWorkflow(wCtx.Requester)
}

func TestAll(t *testing.T) {
	// test all activities
	suite.Run(t, new(GetAppUnitTestSuite))
	suite.Run(t, new(GetBlockUnitTestSuite))
	suite.Run(t, new(GetSessionUnitTestSuite))
	// then test the workflow that will use all those activities
	suite.Run(t, new(RequesterWorkflowUnitTestSuite))
}
