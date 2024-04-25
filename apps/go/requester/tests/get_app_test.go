package tests

import (
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"packages/pocket_rpc/samples"
	"requester/activities"
)

// define a test suite struct
type GetAppUnitTestSuite struct {
	BaseSuite
}

func (s *GetAppUnitTestSuite) Test_GetApp_Activity() {
	getAppParams := activities.GetAppParams{
		Address: "f3abbe313689a603a1a6d6a43330d0440a552288",
		Service: "0001",
	}

	s.mockRpc.
		On("GetApp", getAppParams.Address).
		Return(samples.GetAppMock(s.app.Logger), nil).
		Times(1)

	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.GetApp, getAppParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)
	// rpc must be called once
	s.mockRpc.AssertExpectations(s.T())
	// Check that there was no error returned from the Activity
	result := poktGoSdk.App{}
	s.NoError(future.Get(&result))
	// Check for the expected return value.
	s.Equal(getAppParams.Address, result.Address)
}

func (s *GetAppUnitTestSuite) Test_GetApp_Error_Activity() {
	getAppParams := activities.GetAppParams{
		Address: "f3abbe313689a603a1a6d6a43330d0440a552288",
		Service: "0001",
	}

	s.mockRpc.
		On("GetApp", getAppParams.Address).
		Return(nil, errors.New("not found"))

	// Run the Activity in the test environment
	_, err := s.activityEnv.ExecuteActivity(activities.Activities.GetApp, getAppParams)
	// Check there was no error on the call to execute the Activity
	s.Error(err)
}
