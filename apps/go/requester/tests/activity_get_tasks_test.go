package tests

import (
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"packages/mongodb"
	"packages/utils"
	"requester/activities"
	"requester/types"
)

// define a test suite struct
type GetTasksUnitTestSuite struct {
	BaseSuite
}

func (s *GetTasksUnitTestSuite) Test_GetTasks_AllGood() {
	tasks := GetMockTasks(1)
	task := tasks[0]
	instance := GetMockInstance(task)
	instances := GetMockInstances(1, task)
	prompts := GetMockPrompts(10, task, instance)
	getTasksParams := activities.GetTasksParams{
		Node:    "1234",
		Service: "0001",
	}

	taskMockCollection := mongodb.MockCollection{}
	taskMockCollection.On("Find", mock.Anything, mock.MatchedBy(func(f interface{}) bool {
		if filter, ok := f.(bson.M); !ok {
			return false
		} else if filter["requester_args.address"] == nil || filter["requester_args.address"] != getTasksParams.Node {
			return false
		} else if filter["requester_args.service"] == nil || filter["requester_args.service"] != getTasksParams.Service {
			return false
		} else if filter["done"] == nil || filter["done"] != false {
			return false
		}
		return true
	}), mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Task](tasks), nil, nil)).
		Times(1)

	instanceMockCollection := mongodb.MockCollection{}
	instanceMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Instance](instances), nil, nil)).
		Times(1)

	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Prompt](prompts), nil, nil)).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&taskMockCollection).Times(1)
	mockClient.On("GetCollection", types.InstanceCollection).Return(&instanceMockCollection).Times(1)
	mockClient.On("GetCollection", types.PromptsCollection).Return(&promptMockCollection).Times(1)

	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.GetTasks, getTasksParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)

	// check the expectations after get the result
	mockClient.AssertExpectations(s.T())
	taskMockCollection.AssertExpectations(s.T())
	instanceMockCollection.AssertExpectations(s.T())
	promptMockCollection.AssertExpectations(s.T())

	// Check that there was no error returned from the Activity
	taskRequestsResults := activities.GetTaskRequestResults{}
	s.NoError(future.Get(&taskRequestsResults))
	s.Equal(len(prompts), len(taskRequestsResults.TaskRequests), "tasks requests must be same number of prompts returned on prompts collection find cursor")

	for _, taskRequest := range taskRequestsResults.TaskRequests {
		id, e := primitive.ObjectIDFromHex(taskRequest.TaskId)
		s.NoError(e)
		s.NotNil(id)
		s.Equal(task.Id.Hex(), taskRequest.TaskId)

		id, e = primitive.ObjectIDFromHex(taskRequest.InstanceId)
		s.NoError(e)
		s.NotNil(id)
		s.Equal(instance.Id.Hex(), taskRequest.InstanceId)

		id, e = primitive.ObjectIDFromHex(taskRequest.PromptId)
		s.NoError(e)
		s.NotNil(id)
	}
}

func (s *GetTasksUnitTestSuite) Test_GetTasks_NoTasks() {
	taskMockCollection := mongodb.MockCollection{}
	taskMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(make([]interface{}, 0), nil, nil)).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&taskMockCollection).Times(1)

	getTasksParams := activities.GetTasksParams{
		Node:    "1234",
		Service: "0001",
	}
	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.GetTasks, getTasksParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)

	// check the expectations after get the result
	mockClient.AssertExpectations(s.T())
	taskMockCollection.AssertExpectations(s.T())

	// Check that there was no error returned from the Activity
	taskRequestsResults := activities.GetTaskRequestResults{}
	s.NoError(future.Get(&taskRequestsResults))
	s.Equal(0, len(taskRequestsResults.TaskRequests), "tasks requests must be 0")
}

func (s *GetTasksUnitTestSuite) Test_GetTasks_MissingInstance() {
	tasks := GetMockTasks(1)
	instances := GetMockInstances(0, nil)

	taskMockCollection := mongodb.MockCollection{}
	taskMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Task](tasks), nil, nil)).
		Times(1)

	instancesMockCollection := mongodb.MockCollection{}
	instancesMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Instance](instances), nil, nil)).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&taskMockCollection).Times(1)
	mockClient.On("GetCollection", types.InstanceCollection).Return(&instancesMockCollection).Times(1)

	getTasksParams := activities.GetTasksParams{
		Node:    "1234",
		Service: "0001",
	}
	// Run the Activity in the test environment
	_, err := s.activityEnv.ExecuteActivity(activities.Activities.GetTasks, getTasksParams)
	// Check there was no error on the call to execute the Activity
	s.Error(err)
	// check the expectations after get the result
	mockClient.AssertExpectations(s.T())
	taskMockCollection.AssertExpectations(s.T())
	instancesMockCollection.AssertExpectations(s.T())
}

func (s *GetTasksUnitTestSuite) Test_GetTasks_MissingPrompts() {
	tasks := GetMockTasks(1)
	task := tasks[0]
	instances := GetMockInstances(1, task)
	prompts := GetMockPrompts(0, nil, nil)

	taskMockCollection := mongodb.MockCollection{}
	taskMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Task](tasks), nil, nil)).
		Times(1)

	instancesMockCollection := mongodb.MockCollection{}
	instancesMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Instance](instances), nil, nil)).
		Times(1)

	promptsMockCollection := mongodb.MockCollection{}
	promptsMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*types.Prompt](prompts), nil, nil)).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&taskMockCollection).Times(1)
	mockClient.On("GetCollection", types.InstanceCollection).Return(&instancesMockCollection).Times(1)
	mockClient.On("GetCollection", types.PromptsCollection).Return(&promptsMockCollection).Times(1)

	getTasksParams := activities.GetTasksParams{
		Node:    "1234",
		Service: "0001",
	}
	// Run the Activity in the test environment
	_, err := s.activityEnv.ExecuteActivity(activities.Activities.GetTasks, getTasksParams)
	// Check there was no error on the call to execute the Activity
	s.Error(err)
	// check the expectations after get the result
	mockClient.AssertExpectations(s.T())
	taskMockCollection.AssertExpectations(s.T())
	instancesMockCollection.AssertExpectations(s.T())
	promptsMockCollection.AssertExpectations(s.T())
}
