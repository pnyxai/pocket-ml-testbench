package tests

import (
	"context"
	"encoding/json"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.temporal.io/sdk/client"
	"io"
	"net/http"
	"net/http/httptest"
	"packages/pocket_rpc/samples"
	"packages/utils"
	"requester/common"
	"requester/types"
	"testing"
	"time"
)

type MockHttpReqRes struct {
	Route   string
	Method  string
	Data    interface{}
	GetData func(body []byte) (interface{}, error)
	Code    int
	Delay   *time.Duration
}

type MockRelayData struct {
	Height           int64
	BlockParams      *poktGoSdk.AllParams
	Session          *poktGoSdk.Session
	App              *poktGoSdk.App
	Node             *poktGoSdk.Node
	Service          string
	SessionHeight    int64
	BlocksPerSession int64
	RelayResponse    *poktGoSdk.RelayOutput
}

func (mockResponse *MockHttpReqRes) NewMockServer(t *testing.T) (server *httptest.Server, url string) {
	server = httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				// Check if the path is "/test"
				if r.URL.Path != mockResponse.Route {
					http.Error(w, "Not found", http.StatusNotFound)
					return
				}
				// Check if the method is GET
				if r.Method != mockResponse.Method {
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
					return
				}

				var data interface{}

				if mockResponse.GetData != nil {
					body, err := io.ReadAll(r.Body)
					if err != nil || len(body) == 0 {
						http.Error(w, "Wrong Payload", http.StatusBadRequest)
						return
					}
					// is just a test we do not need to take care about error on Body.Close()
					defer r.Body.Close()

					// implemented to do paginated responses
					data, err = mockResponse.GetData(body)

					if err != nil {
						http.Error(w, "Wrong data resolution", http.StatusInternalServerError)
						return
					}
				} else if mockResponse.Data != nil {
					data = mockResponse.Data
				}

				if mockResponse.Delay != nil {
					time.Sleep(*mockResponse.Delay)
				}

				// write a json response with the proper response header and status code 200
				w.WriteHeader(mockResponse.Code)
				var err error

				if data != nil {
					err = json.NewEncoder(w).Encode(data)
				} else {
					_, err = w.Write([]byte{})
				}

				if err != nil {
					http.Error(w, "Unable to write response to JSON", http.StatusInternalServerError)
					return
				}
			},
		),
	)
	url = server.URL
	t.Cleanup(func() {
		server.Close()
	})
	return
}

type TemporalClientMock struct {
	client.Client
	mock.Mock
}

func (m *TemporalClientMock) ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow interface{}, args ...interface{}) (client.WorkflowRun, error) {
	argsOut := m.Called(ctx, options, workflow, args)
	return argsOut.Get(0).(client.WorkflowRun), argsOut.Error(1)
}

type FakeWorkflowRun struct {
	mock.Mock
	client.WorkflowRun
}

func (w *FakeWorkflowRun) GetID() string {
	return w.Called().String(0)
}

func (w *FakeWorkflowRun) GetRunID() string {
	return w.Called().String(0)
}

func GetMockTask() *types.Task {
	return &types.Task{
		Id: primitive.NewObjectID(),
		RequesterArgs: types.RequesterArgs{
			Address: "1234",
			Service: "0001",
			Method:  "GET",
			Path:    "/test",
		},
		Done: false,
	}
}

func GetMockTasks(count int) []*types.Task {
	docs := make([]*types.Task, count)
	for i := 0; i < count; i++ {
		docs[i] = GetMockTask()
	}
	return docs
}

func GetMockInstance(task *types.Task) *types.Instance {
	instance := types.Instance{
		Id:   primitive.NewObjectID(),
		Done: false,
	}
	if task != nil {
		instance.TaskId = task.Id
		instance.Task = task
	}
	return &instance
}

func GetMockInstances(count int, task *types.Task) []*types.Instance {
	docs := make([]*types.Instance, count)
	for i := 0; i < count; i++ {
		docs[i] = GetMockInstance(task)
	}
	return docs
}

func GetMockPrompt(task *types.Task, instance *types.Instance) *types.Prompt {
	prompt := types.Prompt{
		Id:      primitive.NewObjectID(),
		Data:    "{\"data\":\"test\"}",
		Timeout: 10000,
		Done:    false,
	}
	if task != nil {
		prompt.Task = task
		prompt.TaskId = task.Id
	}
	if instance != nil {
		prompt.Instance = instance
		prompt.InstanceId = instance.Id
	}
	return &prompt
}

func GetMockPrompts(count int, task *types.Task, instance *types.Instance) []*types.Prompt {
	prompts := make([]*types.Prompt, count)
	for i := 0; i < count; i++ {
		prompts[i] = GetMockPrompt(task, instance)
	}
	return prompts
}

func GetRelayMockData(l *zerolog.Logger) *MockRelayData {
	height, _ := samples.GetHeightMock(l).Height.Int64()
	blockParams := samples.GetAllParamsMock(l)
	session := samples.GetSessionMock(l).Session
	app := samples.GetAppMock(l)
	node := utils.GetRandomFromSlice[poktGoSdk.Node](session.Nodes)
	service := *utils.GetRandomFromSlice[string](app.Chains)
	sessionHeight := int64(session.Header.SessionHeight)
	blocksPerSession, _ := common.GetBlocksPerSession(blockParams)
	relayResponse := samples.GetSuccessRelayOutput(l)

	return &MockRelayData{
		Height:           height,
		BlockParams:      blockParams,
		Session:          session,
		App:              app,
		Node:             node,
		Service:          service,
		SessionHeight:    sessionHeight,
		BlocksPerSession: blocksPerSession,
		RelayResponse:    relayResponse,
	}

}
