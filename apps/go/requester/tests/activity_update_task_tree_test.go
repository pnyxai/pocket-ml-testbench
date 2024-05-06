package tests

import (
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"packages/mongodb"
	"packages/utils"
	"requester/activities"
	"requester/types"
)

// define a test suite struct
type UpdateTaskTreeUnitTestSuite struct {
	BaseSuite
}

func (s *UpdateTaskTreeUnitTestSuite) Test_UpdateTaskTree_AllGood() {
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
	instanceOne := types.Instance{
		Id:     primitive.NewObjectID(),
		Done:   true,
		TaskId: task.Id,
		Task:   &task,
	}
	instanceTwo := types.Instance{
		Id:     primitive.NewObjectID(),
		Done:   true,
		TaskId: task.Id,
		Task:   &task,
	}
	promptOne := types.Prompt{
		Id:         primitive.NewObjectID(),
		Data:       "fake",
		Timeout:    10000,
		Done:       true,
		TaskId:     task.Id,
		Task:       &task,
		InstanceId: instanceOne.Id,
		Instance:   &instanceOne,
	}
	promptTwo := types.Prompt{
		Id:         primitive.NewObjectID(),
		Data:       "fake",
		Timeout:    10000,
		Done:       true,
		TaskId:     task.Id,
		Task:       &task,
		InstanceId: instanceOne.Id,
		Instance:   &instanceOne,
	}
	instances := []*types.Instance{&instanceOne, &instanceTwo}
	prompts := []*types.Prompt{&promptOne, &promptTwo}
	params := activities.UpdateTaskTreeRequest{
		PromptId: promptOne.Id.Hex(),
	}

	// mock mongo session
	mockMongoSession := mongodb.MockSession{}
	mockMongoSession.On("WithTransaction", mock.Anything, mock.Anything, mock.Anything).
		Return(activities.UpdateTaskTreeSessionWrapper(activities.Activities, &params)).
		Times(1)

	// mock collections
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("FindOne", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewSingleResultFromDocument(&promptOne, nil, nil), nil, nil).
		Times(1)
	promptMockCollection.On(
		"UpdateOne", mock.Anything, mock.Anything,
		mock.MatchedBy(func(update interface{}) bool {
			doc, err := mongodb.GetDocFromBsonSetUpdateOperation[types.Prompt](update)
			if err != nil {
				s.T().Error(err)
				return false
			}
			s.True(doc.Done)
			return true
		}),
		mock.Anything,
	).
		Return(&mongo.UpdateResult{
			MatchedCount:  1,
			ModifiedCount: 1,
			UpsertedCount: 0,
			UpsertedID:    nil,
		}, nil).
		Times(1)
	promptMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Prompt](prompts), nil, nil))

	instanceMockCollection := mongodb.MockCollection{}
	instanceMockCollection.On("FindOne", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewSingleResultFromDocument(&instanceOne, nil, nil), nil, nil).
		Times(1)
	instanceMockCollection.On(
		"UpdateOne", mock.Anything, mock.Anything,
		mock.MatchedBy(func(update interface{}) bool {
			doc, err := mongodb.GetDocFromBsonSetUpdateOperation[types.Instance](update)
			if err != nil {
				s.T().Error(err)
				return false
			}
			s.True(doc.Done)
			return true
		}),
		mock.Anything,
	).
		Return(&mongo.UpdateResult{
			MatchedCount:  1,
			ModifiedCount: 1,
			UpsertedCount: 0,
			UpsertedID:    nil,
		}, nil).
		Times(1)
	instanceMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Instance](instances), nil, nil))

	taskMockCollection := mongodb.MockCollection{}
	taskMockCollection.On("FindOne", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewSingleResultFromDocument(&task, nil, nil), nil, nil).
		Times(1)
	taskMockCollection.On(
		"UpdateOne", mock.Anything, mock.Anything,
		mock.MatchedBy(func(update interface{}) bool {
			doc, err := mongodb.GetDocFromBsonSetUpdateOperation[types.Task](update)
			if err != nil {
				s.T().Error(err)
				return false
			}
			s.True(doc.Done)
			return true
		}),
		mock.Anything,
	).
		Return(&mongo.UpdateResult{
			MatchedCount:  1,
			ModifiedCount: 1,
			UpsertedCount: 0,
			UpsertedID:    nil,
		}, nil).
		Times(1)

	// mock mongodb client
	mongoMockClient := s.GetMongoClientMock()
	mongoMockClient.On("GetCollection", types.TaskCollection).Return(&taskMockCollection).Times(1)
	mongoMockClient.On("GetCollection", types.InstanceCollection).Return(&instanceMockCollection).Times(1)
	mongoMockClient.On("GetCollection", types.PromptsCollection).Return(&promptMockCollection).Times(1)
	mongoMockClient.On("StartSession", mock.Anything).Return(&mockMongoSession, nil).Times(1)

	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.UpdateTaskTree, params)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)
	result := activities.UpdateTaskTreeResponse{}
	s.NoError(future.Get(&result))
	s.True(result.IsDone)
	s.Equal(task.Id.Hex(), result.TaskId)
	mongoMockClient.AssertExpectations(s.T())
	mockMongoSession.AssertExpectations(s.T())
	promptMockCollection.AssertExpectations(s.T())
	instanceMockCollection.AssertExpectations(s.T())
	taskMockCollection.AssertExpectations(s.T())
}

func (s *UpdateTaskTreeUnitTestSuite) Test_UpdateTaskTree_Unmet_Prompts() {
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
	instanceOne := types.Instance{
		Id:     primitive.NewObjectID(),
		Done:   true,
		TaskId: task.Id,
		Task:   &task,
	}
	promptOne := types.Prompt{
		Id:         primitive.NewObjectID(),
		Data:       "fake",
		Timeout:    10000,
		Done:       true,
		TaskId:     task.Id,
		Task:       &task,
		InstanceId: instanceOne.Id,
		Instance:   &instanceOne,
	}
	promptTwo := types.Prompt{
		Id:         primitive.NewObjectID(),
		Data:       "fake",
		Timeout:    10000,
		Done:       false,
		TaskId:     task.Id,
		Task:       &task,
		InstanceId: instanceOne.Id,
		Instance:   &instanceOne,
	}
	prompts := []*types.Prompt{&promptOne, &promptTwo}
	params := activities.UpdateTaskTreeRequest{
		PromptId: promptOne.Id.Hex(),
	}

	// mock mongo session
	mockMongoSession := mongodb.MockSession{}
	mockMongoSession.On("WithTransaction", mock.Anything, mock.Anything, mock.Anything).
		Return(activities.UpdateTaskTreeSessionWrapper(activities.Activities, &params)).
		Times(1)

	// mock collections
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("FindOne", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewSingleResultFromDocument(&promptOne, nil, nil), nil, nil).
		Times(1)
	promptMockCollection.On(
		"UpdateOne", mock.Anything, mock.Anything,
		mock.MatchedBy(func(update interface{}) bool {
			doc, err := mongodb.GetDocFromBsonSetUpdateOperation[types.Prompt](update)
			if err != nil {
				s.T().Error(err)
				return false
			}
			s.True(doc.Done)
			return true
		}),
		mock.Anything,
	).
		Return(&mongo.UpdateResult{
			MatchedCount:  1,
			ModifiedCount: 1,
			UpsertedCount: 0,
			UpsertedID:    nil,
		}, nil).
		Times(1)
	promptMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Prompt](prompts), nil, nil))

	// mock mongodb client
	mongoMockClient := s.GetMongoClientMock()
	mongoMockClient.On("GetCollection", types.PromptsCollection).Return(&promptMockCollection).Times(1)
	mongoMockClient.On("StartSession", mock.Anything).Return(&mockMongoSession, nil).Times(1)

	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.UpdateTaskTree, params)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)
	result := activities.UpdateTaskTreeResponse{}
	s.NoError(future.Get(&result))
	s.False(result.IsDone)
	s.Equal(task.Id.Hex(), result.TaskId)
	mongoMockClient.AssertExpectations(s.T())
	mockMongoSession.AssertExpectations(s.T())
	promptMockCollection.AssertExpectations(s.T())
}

func (s *UpdateTaskTreeUnitTestSuite) Test_UpdateTaskTree_Unmet_Instances() {
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
	instanceOne := types.Instance{
		Id:     primitive.NewObjectID(),
		Done:   true,
		TaskId: task.Id,
		Task:   &task,
	}
	instanceTwo := types.Instance{
		Id:     primitive.NewObjectID(),
		Done:   false,
		TaskId: task.Id,
		Task:   &task,
	}
	promptOne := types.Prompt{
		Id:         primitive.NewObjectID(),
		Data:       "fake",
		Timeout:    10000,
		Done:       true,
		TaskId:     task.Id,
		Task:       &task,
		InstanceId: instanceOne.Id,
		Instance:   &instanceOne,
	}
	promptTwo := types.Prompt{
		Id:         primitive.NewObjectID(),
		Data:       "fake",
		Timeout:    10000,
		Done:       true,
		TaskId:     task.Id,
		Task:       &task,
		InstanceId: instanceOne.Id,
		Instance:   &instanceOne,
	}
	instances := []*types.Instance{&instanceOne, &instanceTwo}
	prompts := []*types.Prompt{&promptOne, &promptTwo}
	params := activities.UpdateTaskTreeRequest{
		PromptId: promptOne.Id.Hex(),
	}

	// mock mongo session
	mockMongoSession := mongodb.MockSession{}
	mockMongoSession.On("WithTransaction", mock.Anything, mock.Anything, mock.Anything).
		Return(activities.UpdateTaskTreeSessionWrapper(activities.Activities, &params)).
		Times(1)

	// mock collections
	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("FindOne", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewSingleResultFromDocument(&promptOne, nil, nil), nil, nil).
		Times(1)
	promptMockCollection.On(
		"UpdateOne", mock.Anything, mock.Anything,
		mock.MatchedBy(func(update interface{}) bool {
			doc, err := mongodb.GetDocFromBsonSetUpdateOperation[types.Prompt](update)
			if err != nil {
				s.T().Error(err)
				return false
			}
			s.True(doc.Done)
			return true
		}),
		mock.Anything,
	).
		Return(&mongo.UpdateResult{
			MatchedCount:  1,
			ModifiedCount: 1,
			UpsertedCount: 0,
			UpsertedID:    nil,
		}, nil).
		Times(1)
	promptMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Prompt](prompts), nil, nil))

	instanceMockCollection := mongodb.MockCollection{}
	instanceMockCollection.On("FindOne", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewSingleResultFromDocument(&instanceOne, nil, nil), nil, nil).
		Times(1)
	instanceMockCollection.On(
		"UpdateOne", mock.Anything, mock.Anything,
		mock.MatchedBy(func(update interface{}) bool {
			doc, err := mongodb.GetDocFromBsonSetUpdateOperation[types.Instance](update)
			if err != nil {
				s.T().Error(err)
				return false
			}
			s.True(doc.Done)
			return true
		}),
		mock.Anything,
	).
		Return(&mongo.UpdateResult{
			MatchedCount:  1,
			ModifiedCount: 1,
			UpsertedCount: 0,
			UpsertedID:    nil,
		}, nil).
		Times(1)
	instanceMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Instance](instances), nil, nil))

	// mock mongodb client
	mongoMockClient := s.GetMongoClientMock()
	mongoMockClient.On("GetCollection", types.InstanceCollection).Return(&instanceMockCollection).Times(1)
	mongoMockClient.On("GetCollection", types.PromptsCollection).Return(&promptMockCollection).Times(1)
	mongoMockClient.On("StartSession", mock.Anything).Return(&mockMongoSession, nil).Times(1)

	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.UpdateTaskTree, params)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)
	result := activities.UpdateTaskTreeResponse{}
	s.NoError(future.Get(&result))
	s.False(result.IsDone)
	s.Equal(task.Id.Hex(), result.TaskId)
	mongoMockClient.AssertExpectations(s.T())
	mockMongoSession.AssertExpectations(s.T())
	promptMockCollection.AssertExpectations(s.T())
	instanceMockCollection.AssertExpectations(s.T())
}
