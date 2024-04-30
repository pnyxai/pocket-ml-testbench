package tests

import (
	"errors"
	"packages/pocket_rpc/samples"
	"requester/activities"
)

// define a test suite struct
type GetHeightUnitTestSuite struct {
	BaseSuite
}

func (s *GetHeightUnitTestSuite) Test_GetHeight_Activity() {
	getHeightOutput := samples.GetHeightMock(s.app.Logger)
	mockHeight, _ := getHeightOutput.Height.Int64()
	s.mockRpc.
		On("GetHeight").
		Return(mockHeight, nil)

	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.GetHeight)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)
	// Check that there was no error returned from the Activity
	height := int64(0)
	s.NoError(future.Get(&height))
	// check not nil returned for params
	s.Equal(mockHeight, height)
}

func (s *GetHeightUnitTestSuite) Test_GetHeight_Error_Activity() {
	s.mockRpc.
		On("GetHeight", 0).
		Return(nil, errors.New("not found"))

	// Run the Activity in the test environment
	_, err := s.activityEnv.ExecuteActivity(activities.Activities.GetHeight)
	// Check there was no error on the call to execute the Activity
	s.Error(err)
}
