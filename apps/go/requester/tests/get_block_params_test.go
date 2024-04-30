package tests

import (
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"packages/pocket_rpc/samples"
	"requester/activities"
)

// define a test suite struct
type GetBlockParamsUnitTestSuite struct {
	BaseSuite
}

func (s *GetBlockParamsUnitTestSuite) Test_GetBlockParams_Activity() {
	height := int64(0)
	getAllParamsOutput := samples.GetAllParamsMock(s.app.Logger)
	s.mockRpc.
		On("GetAllParams", height).
		Return(getAllParamsOutput, nil)

	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.GetBlockParams, height)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)
	// Check that there was no error returned from the Activity
	allParams := poktGoSdk.AllParams{}
	s.NoError(future.Get(&allParams))
	// check not nil returned for params
	s.NotNil(allParams)
}

func (s *GetBlockParamsUnitTestSuite) Test_GetBlockParams_Error_Activity() {
	height := int64(0)
	s.mockRpc.
		On("GetAllParams", height).
		Return(nil, errors.New("not found"))

	// Run the Activity in the test environment
	_, err := s.activityEnv.ExecuteActivity(activities.Activities.GetBlockParams, height)
	// Check there was no error on the call to execute the Activity
	s.Error(err)
}
