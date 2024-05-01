package tests

import (
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"packages/mongodb"
	"requester/activities"
	"requester/types"
)

// define a test suite struct
type GetTasksUnitTestSuite struct {
	BaseSuite
}

func interfaceSlice[T interface{}](elements []T) []interface{} {
	out := make([]interface{}, len(elements))
	for i, v := range elements {
		out[i] = v
	}
	return out
}

func (s *GetTasksUnitTestSuite) Test_GetTasks_AllGood() {
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
	instance := types.Instance{
		Id:     primitive.NewObjectID(),
		Done:   false,
		TaskId: task.Id,
		Task:   &task,
	}
	tasks := []*types.Task{&task}
	instances := []*types.Instance{&instance}
	prompts := make([]*types.Prompt, 0)
	for i := 0; i < 10; i++ {
		prompts = append(prompts, &types.Prompt{
			Id:         primitive.NewObjectID(),
			Data:       "{\"data\":\"test\"}",
			Timeout:    10000,
			Done:       false,
			TaskId:     task.Id,
			Task:       &task,
			InstanceId: instance.Id,
			Instance:   &instance,
		})
	}
	// end of fake data

	taskMockCollection := mongodb.MockCollection{}
	taskMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Task](tasks), nil, nil)).
		Times(1)

	instanceMockCollection := mongodb.MockCollection{}
	instanceMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Instance](instances), nil, nil)).
		Times(1)

	promptMockCollection := mongodb.MockCollection{}
	promptMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Prompt](prompts), nil, nil)).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&taskMockCollection).Times(1)
	mockClient.On("GetCollection", types.InstanceCollection).Return(&instanceMockCollection).Times(1)
	mockClient.On("GetCollection", types.PromptsCollection).Return(&promptMockCollection).Times(1)

	getTasksParams := activities.GetTasksParams{
		Node:    "1234",
		Service: "0001",
	}
	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.GetTasks, getTasksParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)

	// Check that there was no error returned from the Activity
	taskRequestsResults := activities.GetTaskRequestResults{}
	s.NoError(future.Get(&taskRequestsResults))
	s.Equal(len(prompts), len(taskRequestsResults.TaskRequests), "tasks requests must be same number of prompts returned on prompts collection find cursor")

	// check the expectations after get the result
	mockClient.AssertExpectations(s.T())
	taskMockCollection.AssertExpectations(s.T())
	instanceMockCollection.AssertExpectations(s.T())
	promptMockCollection.AssertExpectations(s.T())

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

	// Check that there was no error returned from the Activity
	taskRequestsResults := activities.GetTaskRequestResults{}
	s.NoError(future.Get(&taskRequestsResults))
	s.Equal(0, len(taskRequestsResults.TaskRequests), "tasks requests must be 0")

	// check the expectations after get the result
	mockClient.AssertExpectations(s.T())
	taskMockCollection.AssertExpectations(s.T())
}

func (s *GetTasksUnitTestSuite) Test_GetTasks_MissingInstance() {
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
	tasks := []*types.Task{&task}
	instances := make([]*types.Instance, 0)
	// end of fake data

	taskMockCollection := mongodb.MockCollection{}
	taskMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Task](tasks), nil, nil)).
		Times(1)

	instancesMockCollection := mongodb.MockCollection{}
	instancesMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Instance](instances), nil, nil)).
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
	instance := types.Instance{
		Id:     primitive.NewObjectID(),
		Done:   false,
		TaskId: task.Id,
		Task:   &task,
	}
	tasks := []*types.Task{&task}
	instances := []*types.Instance{&instance}
	prompts := make([]*types.Prompt, 0)
	// end of fake data

	taskMockCollection := mongodb.MockCollection{}
	taskMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Task](tasks), nil, nil)).
		Times(1)

	instancesMockCollection := mongodb.MockCollection{}
	instancesMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Instance](instances), nil, nil)).
		Times(1)

	promptsMockCollection := mongodb.MockCollection{}
	promptsMockCollection.On("Find", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(interfaceSlice[*types.Prompt](prompts), nil, nil)).
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
