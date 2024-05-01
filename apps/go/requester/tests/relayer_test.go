package tests

import (
	"encoding/json"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"io"
	"net/http"
	"net/http/httptest"
	"packages/mongodb"
	"packages/pocket_rpc"
	"packages/pocket_rpc/samples"
	"packages/utils"
	"requester/activities"
	"requester/common"
	"requester/types"
	"time"
)

type MockResponse struct {
	Route   string
	Method  string
	Data    interface{}
	GetData func(body []byte) (interface{}, error)
	Code    int
	Delay   *time.Duration
}

// define a test suite struct
type RelayerUnitTestSuite struct {
	BaseSuite
}

func (s *RelayerUnitTestSuite) NewMockServicerMockServer(mockResponse MockResponse) (server *httptest.Server, url string) {
	server = httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				// Check if the path is "/test"
				if r.URL.Path != mockResponse.Route {
					http.Error(w, "Not found", http.StatusNotFound)
					return
				}
				// Check if the method is GET
				if r.Method != mockResponse.Method {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					return
				}

				var data interface{}

				if mockResponse.GetData != nil {
					body, err := io.ReadAll(r.Body)
					if err != nil || len(body) == 0 {
						http.Error(w, "Wrong Payload", http.StatusBadRequest)
						return
					}
					defer func() {
						if e := r.Body.Close(); e != nil {
							s.app.Logger.Error().Err(e).Msg("error closing body")
						}
					}()

					// implemented to do paginated responses
					data, err = mockResponse.GetData(body)

					if err != nil {
						http.Error(w, "Wrong data resolution", http.StatusInternalServerError)
						return
					}
				} else if mockResponse.Data != nil {
					data = mockResponse.Data
				}

				if mockResponse.Delay != nil {
					time.Sleep(*mockResponse.Delay)
				}

				// write a json response with the proper response header and status code 200
				w.WriteHeader(mockResponse.Code)
				var err error

				if data != nil {
					err = json.NewEncoder(w).Encode(data)
				} else {
					_, err = w.Write([]byte{})
				}

				if err != nil {
					http.Error(w, "Unable to write response to JSON", http.StatusInternalServerError)
					return
				}
			},
		),
	)
	url = server.URL
	return
}

func (s *RelayerUnitTestSuite) Test_Relayer_AllGood() {
	// fake data
	task := types.Task{
		Id: primitive.NewObjectID(),
		RequesterArgs: types.RequesterArgs{
			Address: "1234",
			Service: "0001",
			Method:  "GET",
			Path:    "/test",
		},
		Done: false,
	}
	prompt := types.Prompt{
		Id:         primitive.NewObjectID(),
		Data:       "{\"data\":\"test\"}",
		Timeout:    10000,
		Done:       false,
		TaskId:     task.Id,
		Task:       &task,
		InstanceId: primitive.NewObjectID(),
	}
	prompts := []*types.Prompt{&prompt}
	relayResponse := samples.GetSuccessRelayOutput(s.app.Logger)
	height, _ := samples.GetHeightMock(s.app.Logger).Height.Int64()
	session := samples.GetSessionMock(s.app.Logger).Session
	blockParams := samples.GetAllParamsMock(s.app.Logger)
	app := samples.GetAppMock(s.app.Logger)
	node := utils.GetRandomFromSlice[poktGoSdk.Node](session.Nodes)
	service := *utils.GetRandomFromSlice[string](app.Chains)
	sessionHeight := int64(session.Header.SessionHeight)
	blocksPerSession, _ := common.GetBlocksPerSession(blockParams)
	relayDelay := time.Duration(100) * time.Millisecond
	mockServer, mockServerUrl := s.NewMockServicerMockServer(MockResponse{
		Route:   pocket_rpc.ClientRelayRoute,
		Method:  http.MethodPost,
		Data:    relayResponse,
		GetData: nil,
		Code:    http.StatusOK,
		Delay:   &relayDelay,
	})
	s.T().Cleanup(func() {
		mockServer.Close()
	})
	node.ServiceURL = mockServerUrl
	// end of fake data

	s.GetPocketRpcMock().
		On("GetHeight").
		Return(height, nil)

	tasksMockCollection := mongodb.MockCollection{}
	tasksMockCollection.On("Name").Return(types.TaskCollection).Times(1)
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("Aggregate", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Prompt](prompts), nil, nil)).
		Times(1)
	responseMockCollection := mongodb.MockCollection{}
	responseMockCollection.On(
		"UpdateOne", mock.Anything, mock.Anything,
		mock.MatchedBy(func(update interface{}) bool {
			relayerResponse, err := mongodb.GetDocFromBsonSetUpdateOperation[types.RelayResponse](update)
			if err != nil {
				s.T().Error(err)
				return false
			}
			s.True(relayerResponse.Ok)
			s.Equal(activities.RelayResponseCodes.Ok, relayerResponse.Code)
			s.GreaterOrEqual(relayerResponse.Ms, relayDelay.Milliseconds())
			s.Equal(relayResponse.Response, relayerResponse.Response)
			s.Equal("", relayerResponse.Error)
			s.Equal(task.Id, relayerResponse.TaskId)
			s.Equal(prompt.InstanceId, relayerResponse.InstanceId)
			s.Equal(prompt.Id, relayerResponse.PromptId)
			return true
		}),
		mock.Anything).
		Return(&mongo.UpdateResult{
			MatchedCount:  1,
			ModifiedCount: 0,
			UpsertedCount: 1,
			// fake id of course
			UpsertedID: primitive.NewObjectID(),
		}, nil).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&tasksMockCollection).Times(1)
	mockClient.On("GetCollection", types.PromptsCollection).Return(&promptMockCollection).Times(1)
	mockClient.On("GetCollection", types.ResponseCollection).Return(&responseMockCollection).Times(1)

	relayerParams := activities.RelayerParams{
		Session:          session,
		Node:             node,
		App:              app,
		Service:          service,
		SessionHeight:    sessionHeight,
		BlocksPerSession: blocksPerSession,
		PromptId:         prompt.Id.Hex(),
	}
	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.Relayer, relayerParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)

	// Check that there was no error returned from the Activity
	relayerResponse := activities.RelayerResponse{}
	s.NoError(future.Get(&relayerResponse))

	// check the expectations after get the result
	mockClient.AssertExpectations(s.T())
	tasksMockCollection.AssertExpectations(s.T())
	promptMockCollection.AssertExpectations(s.T())
	responseMockCollection.AssertExpectations(s.T())

	s.NotNil(relayerResponse)
	s.NotEmpty(relayerResponse.ResponseId)
}

func (s *RelayerUnitTestSuite) Test_Relayer_Error() {
	// fake data
	task := types.Task{
		Id: primitive.NewObjectID(),
		RequesterArgs: types.RequesterArgs{
			Address: "1234",
			Service: "0001",
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
	prompts := []*types.Prompt{&prompt}
	relayDelay := time.Duration(100) * time.Millisecond
	relayResponse := samples.GetErroredRelayOutput(s.app.Logger)
	height, _ := samples.GetHeightMock(s.app.Logger).Height.Int64()
	session := samples.GetSessionMock(s.app.Logger).Session
	blockParams := samples.GetAllParamsMock(s.app.Logger)
	app := samples.GetAppMock(s.app.Logger)
	node := utils.GetRandomFromSlice[poktGoSdk.Node](session.Nodes)
	service := *utils.GetRandomFromSlice[string](app.Chains)
	sessionHeight := int64(session.Header.SessionHeight)
	blocksPerSession, _ := common.GetBlocksPerSession(blockParams)
	mockServer, mockServerUrl := s.NewMockServicerMockServer(MockResponse{
		Route:   pocket_rpc.ClientRelayRoute,
		Method:  http.MethodPost,
		Data:    relayResponse, // todo: check how we can "build" what is supposed to be on the relay payload here because poktGoSdk does not expose buildRelay method
		GetData: nil,
		Code:    http.StatusBadRequest,
		Delay:   &relayDelay,
	})
	s.T().Cleanup(func() {
		mockServer.Close()
	})
	node.ServiceURL = mockServerUrl
	// end of fake data

	s.GetPocketRpcMock().
		On("GetHeight").
		Return(height, nil)

	tasksMockCollection := mongodb.MockCollection{}
	tasksMockCollection.On("Name").Return(types.TaskCollection).Times(1)
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("Aggregate", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Prompt](prompts), nil, nil)).
		Times(1)
	responseMockCollection := mongodb.MockCollection{}
	responseMockCollection.On(
		"UpdateOne", mock.Anything, mock.Anything,
		mock.MatchedBy(func(update interface{}) bool {
			relayerResponse, err := mongodb.GetDocFromBsonSetUpdateOperation[types.RelayResponse](update)
			if err != nil {
				s.T().Error(err)
				return false
			}
			s.False(relayerResponse.Ok)
			s.Equal(activities.RelayResponseCodes.Relay, relayerResponse.Code)
			s.GreaterOrEqual(relayerResponse.Ms, relayDelay.Milliseconds())
			s.Equal("", relayerResponse.Response)
			s.NotEmpty(relayerResponse.Error)
			s.Equal(task.Id, relayerResponse.TaskId)
			s.Equal(prompt.InstanceId, relayerResponse.InstanceId)
			s.Equal(prompt.Id, relayerResponse.PromptId)
			return true
		}),
		mock.Anything,
	).
		Return(&mongo.UpdateResult{
			MatchedCount:  1,
			ModifiedCount: 0,
			UpsertedCount: 1,
			// fake id of course
			UpsertedID: primitive.NewObjectID(),
		}, nil).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&tasksMockCollection).Times(1)
	mockClient.On("GetCollection", types.PromptsCollection).Return(&promptMockCollection).Times(1)
	mockClient.On("GetCollection", types.ResponseCollection).Return(&responseMockCollection).Times(1)

	relayerParams := activities.RelayerParams{
		Session:          session,
		Node:             node,
		App:              app,
		Service:          service,
		SessionHeight:    sessionHeight,
		BlocksPerSession: blocksPerSession,
		PromptId:         prompt.Id.Hex(),
	}
	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.Relayer, relayerParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)

	// Check that there was no error returned from the Activity
	relayerResponse := activities.RelayerResponse{}
	s.NoError(future.Get(&relayerResponse))

	// check the expectations after get the result
	mockClient.AssertExpectations(s.T())
	tasksMockCollection.AssertExpectations(s.T())
	promptMockCollection.AssertExpectations(s.T())
	responseMockCollection.AssertExpectations(s.T())

	s.NotNil(relayerResponse)
	s.NotEmpty(relayerResponse.ResponseId)
}

func (s *RelayerUnitTestSuite) Test_Relayer_Out_of_Session_Error() {
	// fake data
	task := types.Task{
		Id: primitive.NewObjectID(),
		RequesterArgs: types.RequesterArgs{
			Address: "1234",
			Service: "0001",
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
	prompts := []*types.Prompt{&prompt}
	relayDelay := time.Duration(100) * time.Millisecond
	relayResponse := samples.GetEvidenceSealedRelayOutput(s.app.Logger)
	height, _ := samples.GetHeightMock(s.app.Logger).Height.Int64()
	session := samples.GetSessionMock(s.app.Logger).Session
	blockParams := samples.GetAllParamsMock(s.app.Logger)
	app := samples.GetAppMock(s.app.Logger)
	node := utils.GetRandomFromSlice[poktGoSdk.Node](session.Nodes)
	service := *utils.GetRandomFromSlice[string](app.Chains)
	sessionHeight := int64(session.Header.SessionHeight)
	blocksPerSession, _ := common.GetBlocksPerSession(blockParams)
	mockServer, mockServerUrl := s.NewMockServicerMockServer(MockResponse{
		Route:   pocket_rpc.ClientRelayRoute,
		Method:  http.MethodPost,
		Data:    relayResponse,
		GetData: nil,
		Code:    http.StatusBadRequest,
		Delay:   &relayDelay,
	})
	s.T().Cleanup(func() {
		mockServer.Close()
	})
	node.ServiceURL = mockServerUrl
	// end of fake data

	s.GetPocketRpcMock().
		On("GetHeight").
		Return(height, nil)

	tasksMockCollection := mongodb.MockCollection{}
	tasksMockCollection.On("Name").Return(types.TaskCollection).Times(1)
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("Aggregate", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Prompt](prompts), nil, nil)).
		Times(1)
	responseMockCollection := mongodb.MockCollection{}
	responseMockCollection.On(
		"UpdateOne", mock.Anything, mock.Anything,
		mock.MatchedBy(func(update interface{}) bool {
			relayerResponse, err := mongodb.GetDocFromBsonSetUpdateOperation[types.RelayResponse](update)
			if err != nil {
				s.T().Error(err)
				return false
			}
			s.False(relayerResponse.Ok)
			s.Equal(activities.RelayResponseCodes.OutOfSession, relayerResponse.Code)
			s.GreaterOrEqual(relayerResponse.Ms, relayDelay.Milliseconds())
			s.Equal("", relayerResponse.Response)
			s.NotEmpty(relayerResponse.Error)
			s.Equal(task.Id, relayerResponse.TaskId)
			s.Equal(prompt.InstanceId, relayerResponse.InstanceId)
			s.Equal(prompt.Id, relayerResponse.PromptId)
			return true
		}),
		mock.Anything,
	).
		Return(&mongo.UpdateResult{
			MatchedCount:  1,
			ModifiedCount: 0,
			UpsertedCount: 1,
			// fake id of course
			UpsertedID: primitive.NewObjectID(),
		}, nil).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&tasksMockCollection).Times(1)
	mockClient.On("GetCollection", types.PromptsCollection).Return(&promptMockCollection).Times(1)
	mockClient.On("GetCollection", types.ResponseCollection).Return(&responseMockCollection).Times(1)

	relayerParams := activities.RelayerParams{
		Session:          session,
		Node:             node,
		App:              app,
		Service:          service,
		SessionHeight:    sessionHeight,
		BlocksPerSession: blocksPerSession,
		PromptId:         prompt.Id.Hex(),
	}
	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.Relayer, relayerParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)

	// Check that there was no error returned from the Activity
	relayerResponse := activities.RelayerResponse{}
	s.NoError(future.Get(&relayerResponse))

	// check the expectations after get the result
	mockClient.AssertExpectations(s.T())
	tasksMockCollection.AssertExpectations(s.T())
	promptMockCollection.AssertExpectations(s.T())
	responseMockCollection.AssertExpectations(s.T())

	s.NotNil(relayerResponse)
	s.NotEmpty(relayerResponse.ResponseId)
}

func (s *RelayerUnitTestSuite) Test_Relayer_Node_Error() {
	// fake data
	task := types.Task{
		Id: primitive.NewObjectID(),
		RequesterArgs: types.RequesterArgs{
			Address: "1234",
			Service: "0001",
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
	prompts := []*types.Prompt{&prompt}
	relayDelay := time.Duration(100) * time.Millisecond
	//relayResponse := samples.GetEvidenceSealedRelayOutput(s.app.Logger)
	height, _ := samples.GetHeightMock(s.app.Logger).Height.Int64()
	session := samples.GetSessionMock(s.app.Logger).Session
	blockParams := samples.GetAllParamsMock(s.app.Logger)
	app := samples.GetAppMock(s.app.Logger)
	node := utils.GetRandomFromSlice[poktGoSdk.Node](session.Nodes)
	service := *utils.GetRandomFromSlice[string](app.Chains)
	sessionHeight := int64(session.Header.SessionHeight)
	blocksPerSession, _ := common.GetBlocksPerSession(blockParams)
	mockServer, mockServerUrl := s.NewMockServicerMockServer(MockResponse{
		Route:   pocket_rpc.ClientRelayRoute,
		Method:  http.MethodPost,
		Data:    nil,
		GetData: nil,
		Code:    http.StatusInternalServerError,
		Delay:   &relayDelay,
	})
	s.T().Cleanup(func() {
		mockServer.Close()
	})
	node.ServiceURL = mockServerUrl
	// end of fake data

	s.GetPocketRpcMock().
		On("GetHeight").
		Return(height, nil)

	tasksMockCollection := mongodb.MockCollection{}
	tasksMockCollection.On("Name").Return(types.TaskCollection).Times(1)
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("Aggregate", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Prompt](prompts), nil, nil)).
		Times(1)
	responseMockCollection := mongodb.MockCollection{}
	responseMockCollection.On(
		"UpdateOne", mock.Anything, mock.Anything,
		mock.MatchedBy(func(update interface{}) bool {
			relayerResponse, err := mongodb.GetDocFromBsonSetUpdateOperation[types.RelayResponse](update)
			if err != nil {
				s.T().Error(err)
				return false
			}
			s.False(relayerResponse.Ok)
			s.Equal(activities.RelayResponseCodes.Node, relayerResponse.Code)
			s.GreaterOrEqual(relayerResponse.Ms, relayDelay.Milliseconds())
			s.Equal("", relayerResponse.Response)
			s.NotEmpty(relayerResponse.Error)
			s.Equal(task.Id, relayerResponse.TaskId)
			s.Equal(prompt.InstanceId, relayerResponse.InstanceId)
			s.Equal(prompt.Id, relayerResponse.PromptId)
			return true
		}),
		mock.Anything,
	).
		Return(&mongo.UpdateResult{
			MatchedCount:  1,
			ModifiedCount: 0,
			UpsertedCount: 1,
			// fake id of course
			UpsertedID: primitive.NewObjectID(),
		}, nil).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&tasksMockCollection).Times(1)
	mockClient.On("GetCollection", types.PromptsCollection).Return(&promptMockCollection).Times(1)
	mockClient.On("GetCollection", types.ResponseCollection).Return(&responseMockCollection).Times(1)

	relayerParams := activities.RelayerParams{
		Session:          session,
		Node:             node,
		App:              app,
		Service:          service,
		SessionHeight:    sessionHeight,
		BlocksPerSession: blocksPerSession,
		PromptId:         prompt.Id.Hex(),
	}
	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.Relayer, relayerParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)

	// Check that there was no error returned from the Activity
	relayerResponse := activities.RelayerResponse{}
	s.NoError(future.Get(&relayerResponse))

	// check the expectations after get the result
	mockClient.AssertExpectations(s.T())
	tasksMockCollection.AssertExpectations(s.T())
	promptMockCollection.AssertExpectations(s.T())
	responseMockCollection.AssertExpectations(s.T())

	s.NotNil(relayerResponse)
	s.NotEmpty(relayerResponse.ResponseId)
}
