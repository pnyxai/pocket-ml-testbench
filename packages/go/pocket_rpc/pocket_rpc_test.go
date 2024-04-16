package pocket_rpc

import (
	"encoding/json"
	"fmt"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"pocket_rpc/samples"
	"reflect"
	"testing"
	"time"
)

// define a test suite struct
type UnitTestSuite struct {
	suite.Suite
	logger *zerolog.Logger
}

type MockResponse struct {
	Route   string
	Method  string
	Data    interface{}
	GetData func(body []byte) (interface{}, error)
	Code    int
}

type PagedOutput[T any] struct {
	Result     []T `json:"result"`
	Page       int `json:"page"`
	TotalPages int `json:"total_pages"`
}

func GetPagedEntity[T interface{}](items []T) func([]byte) (interface{}, error) {
	return func(body []byte) (interface{}, error) {
		params := HeightAndOptsParams{}
		if e := json.Unmarshal(body, &params); e != nil {
			return nil, e
		}

		start := (params.Opts.Page - 1) * params.Opts.PerPage
		end := start + params.Opts.PerPage

		if start > len(items) {
			return nil, fmt.Errorf("page number is out of range")
		}

		if end > len(items) {
			end = len(items)
		}

		totalItems := float64(len(items))

		return &PagedOutput[T]{
			Result:     items[start:end],
			Page:       params.Opts.Page,
			TotalPages: int(math.Ceil(totalItems / float64(params.Opts.PerPage))),
		}, nil
	}
}

func (s *UnitTestSuite) NewMockClientPoolServer(mockResponse MockResponse) *ClientPool {
	server := httptest.NewServer(
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
					defer func() {
						if e := r.Body.Close(); e != nil {
							s.logger.Error().Err(e).Msg("error closing body")
						}
					}()

					// implemented to do paginated responses
					data, err = mockResponse.GetData(body)

					if err != nil {
						http.Error(w, "Wrong data resolution", http.StatusInternalServerError)
						return
					}
				} else {
					data = mockResponse.Data
				}

				// write a json response with the proper response header and status code 200
				w.WriteHeader(mockResponse.Code)
				err := json.NewEncoder(w).Encode(data)
				if err != nil {
					http.Error(w, "Unable to write response to JSON", http.StatusInternalServerError)
					return
				}
			},
		),
	)
	return NewClientPool([]string{server.URL}, nil, s.logger)
}

func (s *UnitTestSuite) Test_PocketRpc_GetApp() {
	type fields struct {
		clientPool *ClientPool
		pageSize   int
	}
	type args struct {
		address string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *poktGoSdk.App
		wantErr bool
	}{
		{
			name: "all_good",
			fields: fields{
				clientPool: s.NewMockClientPoolServer(MockResponse{
					Route:  QueryAppRoute,
					Method: http.MethodPost,
					Data: &poktGoSdk.App{
						Address:       "f3abbe313689a603a1a6d6a43330d0440a552288",
						PublicKey:     "1802f4116b9d3798e2766a2452fbeb4d280fa99e77e61193df146ca4d88b38af",
						Jailed:        false,
						Status:        2,
						Chains:        []string{"0001"},
						StakedTokens:  "15000000000",
						MaxRelays:     "10000",
						UnstakingTime: time.Time{},
					},
					Code: http.StatusOK,
				}),
				pageSize: 0,
			},
			args: args{
				address: "f3abbe313689a603a1a6d6a43330d0440a552288",
			},
			want: &poktGoSdk.App{
				Address:       "f3abbe313689a603a1a6d6a43330d0440a552288",
				PublicKey:     "1802f4116b9d3798e2766a2452fbeb4d280fa99e77e61193df146ca4d88b38af",
				Jailed:        false,
				Status:        2,
				Chains:        []string{"0001"},
				StakedTokens:  "15000000000",
				MaxRelays:     "10000",
				UnstakingTime: time.Time{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		suiteT := s.T()
		suiteT.Run(tt.name, func(t *testing.T) {
			rpc := &PocketRpc{
				clientPool: tt.fields.clientPool,
				pageSize:   tt.fields.pageSize,
			}
			got, err := rpc.GetApp(tt.args.address)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetApp() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetApp() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func (s *UnitTestSuite) Test_PocketRpc_GetNodes() {
	type fields struct {
		clientPool *ClientPool
		pageSize   int
	}
	type args struct {
		service string
	}

	nodesSample := samples.GetNodesMock(s.logger)
	if nodesSample == nil {
		s.Fail("missing sample: GetNodesMock")
		return
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []*poktGoSdk.Node
		wantErr bool
	}{
		{
			name: "all_good",
			fields: fields{
				clientPool: s.NewMockClientPoolServer(MockResponse{
					Route:   QueryNodesRoute,
					Method:  http.MethodPost,
					GetData: GetPagedEntity[*poktGoSdk.Node](nodesSample.Result),
					Code:    http.StatusOK,
				}),
				pageSize: 2,
			},
			args: args{
				service: "0001",
			},
			want:    nodesSample.Result,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		suiteT := s.T()
		suiteT.Run(tt.name, func(t *testing.T) {
			rpc := &PocketRpc{
				clientPool: tt.fields.clientPool,
				pageSize:   tt.fields.pageSize,
			}
			got, err := rpc.GetNodes(tt.args.service)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetNodes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetNodes() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func (s *UnitTestSuite) SetupTest() {
	l := zerolog.New(
		zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		},
	).Level(zerolog.DebugLevel).With().Caller().Timestamp().Logger()
	s.logger = &l
}

func TestUnitTestSuite(t *testing.T) {
	// run all the tests
	suite.Run(t, new(UnitTestSuite))
}
