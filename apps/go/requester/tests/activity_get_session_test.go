package tests

import (
	"errors"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"go.temporal.io/sdk/temporal"
	"packages/pocket_rpc/samples"
	"reflect"
	"requester/activities"
	"testing"
)

// define a test suite struct
type GetSessionUnitTestSuite struct {
	BaseSuite
}

func (s *GetSessionUnitTestSuite) Test_GetSession_Activity() {
	getSessionParams := activities.GetSessionParams{
		App:     "f3abbe313689a603a1a6d6a43330d0440a552288",
		Service: "0001",
	}
	signer, _ := s.app.SignerByAddress.Load(getSessionParams.App)

	getSessionOutput := samples.GetSessionMock(s.app.Logger)
	s.GetPocketRpcMock().
		On("GetSession", signer.GetPublicKey(), getSessionParams.Service).
		Return(getSessionOutput, nil).
		Times(1)

	// Run the Activity in the test environment
	future, err := s.activityEnv.ExecuteActivity(activities.Activities.GetSession, getSessionParams)
	// Check there was no error on the call to execute the Activity
	s.NoError(err)
	s.GetPocketRpcMock().AssertExpectations(s.T())
	// Check that there was no error returned from the Activity
	result := poktGoSdk.DispatchOutput{}
	s.NoError(future.Get(&result))
	// check not nil returned
	s.NotNil(result)
	s.True(reflect.DeepEqual(&result, getSessionOutput))
}

func (s *GetSessionUnitTestSuite) Test_GetSession_Rpc_Errored_Activity() {
	getSessionParams := activities.GetSessionParams{
		App:     "f3abbe313689a603a1a6d6a43330d0440a552288",
		Service: "0001",
	}

	s.GetPocketRpcMock().
		On("GetSession", getSessionParams.App, getSessionParams.Service).
		Return(nil, errors.New("not found")).
		Times(1)

	// Run the Activity in the test environment
	_, err := s.activityEnv.ExecuteActivity(activities.Activities.GetSession, getSessionParams)
	// Check there was no error on the call to execute the Activity
	s.Error(err)
	// GetSession should be called at least once
	s.GetPocketRpcMock().AssertExpectations(s.T())
}

func (s *GetSessionUnitTestSuite) Test_GetSession_Params_Errored_Activity() {
	type fields struct {
		app     string
		service string
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name: "empty_params",
			fields: fields{
				app:     "",
				service: "",
			},
		},
		{
			name: "wrong_pub_key",
			fields: fields{
				app:     "1802f4116b9d3798e2766a2452fbe",
				service: "0001",
			},
		},
		{
			name: "wrong_service",
			fields: fields{
				app:     "1802f4116b9d3798e2766a2452fbeb4d280fa99e77e61193df146ca4d88b38af",
				service: "00001",
			},
		},
	}

	for _, tt := range tests {
		suiteT := s.T()
		suiteT.Run(tt.name, func(t *testing.T) {
			// call the activity with a bad application public key
			_, err := s.activityEnv.ExecuteActivity(
				activities.Activities.GetSession,
				activities.GetSessionParams{
					App:     tt.fields.app,
					Service: tt.fields.service,
				},
			)
			s.GetPocketRpcMock().AssertNotCalled(t, "GetSession")
			// Check there was no error on the call to execute the Activity
			s.Error(err)
			isAppError := temporal.IsApplicationError(err)
			s.Equal(isAppError, true)
		})
	}
}
