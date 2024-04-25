package tests

import (
	"errors"
	"packages/pocket_rpc/samples"
	"requester/activities"
)

// define a test suite struct
type GetBlockUnitTestSuite struct {
	BaseSuite
}

func (s *GetBlockUnitTestSuite) Test_GetBlock_Activity() {
	getBlockParams := activities.GetBlockParams{Height: 0}

	getBlockOutput := samples.GetBlockMock(s.app.Logger)
	s.mockRpc.
		On("GetBlock", getBlockParams.Height).
		Return(getBlockOutput, nil)

	getAllParamsOutput := samples.GetAllParamsMock(s.app.Logger)
	s.mockRpc.
		On("GetAllParams", getBlockParams.Height).
		Return(getAllParamsOutput, nil)

	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.GetBlock, getBlockParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)
	// Check that there was no error returned from the Activity
	result := activities.GetBlockResults{}
	s.NoError(future.Get(&result))
	// check not nil returned for params
	s.NotNil(result.Block)
	// check not nil returned for params
	s.NotNil(result.Params)
}

func (s *GetBlockUnitTestSuite) Test_GetBlock_Error_Activity() {
	getBlockParams := activities.GetBlockParams{Height: 0}

	s.mockRpc.
		On("GetBlock", getBlockParams.Height).
		Return(nil, errors.New("not found"))

	// Run the Activity in the test environment
	_, err := s.activityEnv.ExecuteActivity(activities.Activities.GetBlock, getBlockParams)
	// Check there was no error on the call to execute the Activity
	s.Error(err)
}
