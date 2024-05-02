package tests

import (
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"packages/mongodb"
	"packages/pocket_rpc"
	"packages/pocket_rpc/samples"
	"packages/utils"
	"requester/activities"
	"requester/types"
	"time"
)

// define a test suite struct
type RelayerUnitTestSuite struct {
	BaseSuite
}

func (s *RelayerUnitTestSuite) Test_Relayer_AllGood() {
	task := GetMockTask()
	prompts := GetMockPrompts(1, task, nil)
	prompt := prompts[0]
	relayMockData := GetRelayMockData(s.app.Logger)

	relayDelay := time.Duration(100) * time.Millisecond
	mockResponse := MockHttpReqRes{
		Route:   pocket_rpc.ClientRelayRoute,
		Method:  http.MethodPost,
		Data:    relayMockData.RelayResponse,
		GetData: nil,
		Code:    http.StatusOK,
		Delay:   &relayDelay,
	}
	_, mockServerUrl := mockResponse.NewMockServer(s.T())
	relayMockData.Node.ServiceURL = mockServerUrl

	s.GetPocketRpcMock().On("GetHeight").Return(relayMockData.Height, nil).Times(1)

	tasksMockCollection := mongodb.MockCollection{}
	tasksMockCollection.On("Name").Return(types.TaskCollection).Times(1)
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("Aggregate", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Prompt](prompts), nil, nil)).
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
			s.Equal(relayMockData.RelayResponse.Response, relayerResponse.Response)
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
		Session:          relayMockData.Session,
		Node:             relayMockData.Node,
		App:              relayMockData.App,
		Service:          relayMockData.Service,
		SessionHeight:    relayMockData.SessionHeight,
		BlocksPerSession: relayMockData.BlocksPerSession,
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
	s.GetPocketRpcMock().AssertExpectations(s.T())

	s.NotNil(relayerResponse)
	s.NotEmpty(relayerResponse.ResponseId)
}

func (s *RelayerUnitTestSuite) Test_Relayer_Error() {
	task := GetMockTask()
	prompts := GetMockPrompts(1, task, nil)
	prompt := prompts[0]
	relayDelay := time.Duration(100) * time.Millisecond
	relayMockData := GetRelayMockData(s.app.Logger)
	relayResponse := samples.GetErroredRelayOutput(s.app.Logger)
	mockReqRes := MockHttpReqRes{
		Route:   pocket_rpc.ClientRelayRoute,
		Method:  http.MethodPost,
		Data:    relayResponse,
		GetData: nil,
		Code:    http.StatusBadRequest,
		Delay:   &relayDelay,
	}
	_, mockServerUrl := mockReqRes.NewMockServer(s.T())
	relayMockData.Node.ServiceURL = mockServerUrl

	s.GetPocketRpcMock().On("GetHeight").Return(relayMockData.Height, nil).Times(1)

	tasksMockCollection := mongodb.MockCollection{}
	tasksMockCollection.On("Name").Return(types.TaskCollection).Times(1)
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("Aggregate", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Prompt](prompts), nil, nil)).
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
		Session:          relayMockData.Session,
		Node:             relayMockData.Node,
		App:              relayMockData.App,
		Service:          relayMockData.Service,
		SessionHeight:    relayMockData.SessionHeight,
		BlocksPerSession: relayMockData.BlocksPerSession,
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
	s.GetPocketRpcMock().AssertExpectations(s.T())

	s.NotNil(relayerResponse)
	s.NotEmpty(relayerResponse.ResponseId)
}

func (s *RelayerUnitTestSuite) Test_Relayer_Out_of_Session_Error() {
	task := GetMockTask()
	prompts := GetMockPrompts(1, task, nil)
	prompt := prompts[0]
	relayDelay := time.Duration(100) * time.Millisecond
	relayMockData := GetRelayMockData(s.app.Logger)
	relayResponse := samples.GetEvidenceSealedRelayOutput(s.app.Logger)
	mockReqRes := MockHttpReqRes{
		Route:   pocket_rpc.ClientRelayRoute,
		Method:  http.MethodPost,
		Data:    relayResponse,
		GetData: nil,
		Code:    http.StatusBadRequest,
		Delay:   &relayDelay,
	}
	_, mockServerUrl := mockReqRes.NewMockServer(s.T())
	relayMockData.Node.ServiceURL = mockServerUrl

	s.GetPocketRpcMock().On("GetHeight").Return(relayMockData.Height, nil).Times(1)

	tasksMockCollection := mongodb.MockCollection{}
	tasksMockCollection.On("Name").Return(types.TaskCollection).Times(1)
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("Aggregate", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Prompt](prompts), nil, nil)).
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
		Session:          relayMockData.Session,
		Node:             relayMockData.Node,
		App:              relayMockData.App,
		Service:          relayMockData.Service,
		SessionHeight:    relayMockData.SessionHeight,
		BlocksPerSession: relayMockData.BlocksPerSession,
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
	s.GetPocketRpcMock().AssertExpectations(s.T())

	s.NotNil(relayerResponse)
	s.NotEmpty(relayerResponse.ResponseId)
}

func (s *RelayerUnitTestSuite) Test_Relayer_Node_Error() {
	task := GetMockTask()
	prompts := GetMockPrompts(1, task, nil)
	prompt := prompts[0]
	relayDelay := time.Duration(100) * time.Millisecond
	relayMockData := GetRelayMockData(s.app.Logger)
	mockReqRes := MockHttpReqRes{
		Route:   pocket_rpc.ClientRelayRoute,
		Method:  http.MethodPost,
		Data:    nil,
		GetData: nil,
		Code:    http.StatusInternalServerError,
		Delay:   &relayDelay,
	}
	_, mockServerUrl := mockReqRes.NewMockServer(s.T())
	relayMockData.Node.ServiceURL = mockServerUrl

	s.GetPocketRpcMock().On("GetHeight").Return(relayMockData.Height, nil).Times(1)

	tasksMockCollection := mongodb.MockCollection{}
	tasksMockCollection.On("Name").Return(types.TaskCollection).Times(1)
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("Aggregate", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Prompt](prompts), nil, nil)).
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
		Session:          relayMockData.Session,
		Node:             relayMockData.Node,
		App:              relayMockData.App,
		Service:          relayMockData.Service,
		SessionHeight:    relayMockData.SessionHeight,
		BlocksPerSession: relayMockData.BlocksPerSession,
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
	s.GetPocketRpcMock().AssertExpectations(s.T())

	s.NotNil(relayerResponse)
	s.NotEmpty(relayerResponse.ResponseId)
}
