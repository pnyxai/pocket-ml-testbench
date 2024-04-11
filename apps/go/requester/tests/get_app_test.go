package tests

import (
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"os"
	"packages/logger"
	"packages/pocket_rpc"
	"requester/activities"
	"requester/types"
	"testing"
	"time"
)

// define a test suite struct
type UnitTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite

	env     *testsuite.TestActivityEnvironment
	app     *types.App
	mockRpc *pocket_rpc.MockRpc
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

	s.mockRpc = pocket_rpc.NewMockRpc()

	ac := &types.App{
		Logger:    &l,
		Config:    cfg,
		PocketRpc: s.mockRpc,
	}

	// set the app to test env
	s.app = ac

	return
}

// initialize the test suite
func (s *UnitTestSuite) SetupTest() {
	s.env = s.NewTestActivityEnvironment()

	s.InitializeTestApp()

	activities.SetAppConfig(s.app)

	s.env.RegisterActivity(activities.Activities.GetApp)
}

// Test_GetApp_Activity tests the GetApp activity
func (s *UnitTestSuite) Test_GetApp_Activity() {
	getAppParams := activities.GetAppParams{
		Address: "f3abbe313689a603a1a6d6a43330d0440a552288",
		Service: "0001",
	}

	clientPool := pocket_rpc.NewClientPool([]string{"http://localhost:8081"}, nil, s.app.Logger)
	s.app.PocketRpc = pocket_rpc.NewPocketRpc(clientPool)

	//s.app.PocketRpc
	//s.mockRpc.
	//	On("GetApp", getAppParams.Address).
	//	Return(samples.GetAppMock(s.app.Logger), nil)

	// Run the Activity in the test environment
	future, err := s.env.ExecuteActivity(activities.Activities.GetApp, getAppParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)
	// Check that there was no error returned from the Activity
	result := poktGoSdk.App{}
	s.NoError(future.Get(&result))
	// Check for the expected return value.
	s.Equal(getAppParams.Address, result.Address)
}

func (s *UnitTestSuite) Test_GetApp_Error_Activity() {
	getAppParams := activities.GetAppParams{
		Address: "f3abbe313689a603a1a6d6a43330d0440a552288",
		Service: "0001",
	}

	s.mockRpc.
		On("GetApp", getAppParams.Address).
		Return(nil, errors.New("not found"))

	// Run the Activity in the test environment
	_, err := s.env.ExecuteActivity(activities.Activities.GetApp, getAppParams)
	// Check there was no error on the call to execute the Activity
	s.Error(err)
}

func TestUnitTestSuite(t *testing.T) {
	// run all the tests
	suite.Run(t, new(UnitTestSuite))
}
