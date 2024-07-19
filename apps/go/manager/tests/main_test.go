package tests

import (
	"manager/activities"
	"manager/types"
	"manager/workflows"
	"os"
	"packages/logger"
	"packages/mongodb"
	"packages/pocket_rpc"
	"packages/pocket_rpc/samples"
	"path"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
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
	// initialize logger
	l := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).Level(zerolog.NoLevel).With().Timestamp().Logger()

	samples.SetBasePath(path.Join("..", "..", "..", "..", "packages", "go", "pocket_rpc", "samples"))

	// mock App config
	cfg := &types.Config{
		MongodbUri: types.DefaultMongodbUri,
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
			Sampler: &types.SamplerConfig{
				WorkflowName: "Sampler",
				TaskQueue:    "sampler",
			},
		},
	}

	ac := &types.App{
		Logger:         &l,
		Config:         cfg,
		PocketRpc:      pocket_rpc.NewMockRpc(),
		Mongodb:        &mongodb.MockClient{},
		TemporalClient: &TemporalClientMock{},
	}

	// set the app to test env
	s.app = ac

	return
}

func (s *BaseSuite) BeforeTest(_, _ string) {
	s.InitializeTestApp()

	zeroLoggerAdapter := logger.NewZerologAdapter(*s.app.Logger)
	s.SetLogger(zeroLoggerAdapter)

	s.app.Mongodb = mongodb.NewMockClient("mongodb://localhost:27017/test", s.app.Logger)

	// set this to workflows and activities to avoid use of context.Context
	wCtx := workflows.SetAppConfig(s.app)
	aCtx := activities.SetAppConfig(s.app)

	s.workflowEnv = s.NewTestWorkflowEnvironment()
	s.activityEnv = s.NewTestActivityEnvironment()

	var activityList = []interface{}{
		aCtx.AnalyzeNode,
		aCtx.GetStaked,
		aCtx.TriggerSampler,
	}

	// register the activities that will need to be mock up here
	for i := range activityList {
		s.activityEnv.RegisterActivity(activityList[i])
		s.workflowEnv.RegisterActivity(activityList[i])
	}

	// register the workflows
	s.workflowEnv.RegisterWorkflow(wCtx.NodeManager)
}

func (s *BaseSuite) GetPocketRpcMock() *pocket_rpc.MockRpc {
	if s.app == nil || s.app.PocketRpc == nil {
		panic("app or pocket rpc client not initialized")
	}
	if c, ok := s.app.PocketRpc.(*pocket_rpc.MockRpc); ok {
		return c
	} else {
		panic("pocket rpc client is not an instance of pocket_rpc.MockRpc")
	}
}

func (s *BaseSuite) GetMongoClientMock() *mongodb.MockClient {
	if s.app == nil || s.app.Mongodb == nil {
		panic("app or mongo client not initialized")
	}
	if c, ok := s.app.Mongodb.(*mongodb.MockClient); ok {
		return c
	} else {
		panic("mongodb client is not an instance of mongodb.MockClient")
	}
}

func (s *BaseSuite) GetTemporalClientMock() *TemporalClientMock {
	if s.app == nil || s.app.TemporalClient == nil {
		panic("app or temporal client not initialized")
	}
	if c, ok := s.app.TemporalClient.(*TemporalClientMock); ok {
		return c
	} else {
		panic("temporal client is not an instance of tests.TemporalClientMock")
	}
}

func TestAll(t *testing.T) {
	// Test functionality
	suite.Run(t, new(CircularBuffertUnitTestSuite))
	suite.Run(t, new(RecordstUnitTestSuite))

	// test all activities
	// TODO

	// then test the workflow that will use all those activities
	// TODO
}
