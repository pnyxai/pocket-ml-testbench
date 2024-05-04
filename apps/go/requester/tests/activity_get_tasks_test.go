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
type GetTasksUnitTestSuite struct {
	BaseSuite
}

func (s *GetTasksUnitTestSuite) Test_GetTasks_AllGood() {
	nodeAddress := "1234"
	service := "0001"
	relayTimeout := float64(1)
	tasks := GetMockTasks(1)
	task := tasks[0]
	instance := GetMockInstance(task)
	prompts := GetMockPrompts(10, task, instance)
	taskRequests := make([]*activities.TaskRequest, len(prompts))
	for i, prompt := range prompts {
		taskRequests[i] = &activities.TaskRequest{
			TaskId:       prompt.TaskId.Hex(),
			InstanceId:   prompt.InstanceId.Hex(),
			PromptId:     prompt.Id.Hex(),
			Node:         nodeAddress,
			RelayTimeout: relayTimeout,
		}
	}
	getTasksParams := activities.GetTasksParams{
		Nodes:   []string{nodeAddress},
		Service: service,
	}

	taskMockCollection := mongodb.MockCollection{}
	taskMockCollection.On("Aggregate", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(utils.InterfaceSlice[*activities.TaskRequest](taskRequests), nil, nil)).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&taskMockCollection).Times(1)

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

		s.Equal(nodeAddress, taskRequest.Node)
		s.Equal(relayTimeout, taskRequest.RelayTimeout)
	}
}

func (s *GetTasksUnitTestSuite) Test_GetTasks_NoTasks() {
	taskMockCollection := mongodb.MockCollection{}
	taskMockCollection.On("Aggregate", mock.Anything, mock.Anything, mock.Anything).
		Return(mongo.NewCursorFromDocuments(make([]interface{}, 0), nil, nil)).
		Times(1)

	mockClient := s.app.Mongodb.(*mongodb.MockClient)
	mockClient.On("GetCollection", types.TaskCollection).Return(&taskMockCollection).Times(1)

	getTasksParams := activities.GetTasksParams{
		Nodes:   []string{"1234"},
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
