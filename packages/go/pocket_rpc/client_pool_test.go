package pocket_rpc

import (
	"context"
	"encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/http/httptest"
	"os"
	"packages/utils"
	"path"
	"pocket_rpc/samples"
	"pocket_rpc/types"
	"reflect"
	"testing"
	"time"
)

// define a test suite struct
type ClientPoolTestSuite struct {
	suite.Suite
	logger *zerolog.Logger
}

func (s *ClientPoolTestSuite) SetupTest() {
	l := zerolog.New(
		zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		},
	).Level(zerolog.DebugLevel).With().Caller().Timestamp().Logger()
	samples.SetBasePath(path.Join("samples"))
	s.logger = &l
}

func (s *ClientPoolTestSuite) Test_NewClientPool() {
	servers := []string{"localhost:12345", "localhost:12346", "localhost:12347"}
	opts := &ClientPoolOptions{
		MaxRetries: 2,
		ReqPerSec:  10,
		MinBackoff: time.Duration(10),
		MaxBackoff: time.Duration(10),
	}
	cp := NewClientPool(servers, opts, s.logger)

	s.Equal(len(servers), cp.clients.Size())
	s.Equal(2, cp.opts.MaxRetries)
	s.True(reflect.DeepEqual(opts, cp.opts), "ClientPoolOptions are different instance got = %v, want %v", cp.opts, opts)

	cp.clients.Range(func(url string, client *Client) bool {
		s.NotNil(client)
		s.True(utils.StringInSlice(url, servers))
		s.Equal(url, client.url)
		return true
	})
}

func (s *ClientPoolTestSuite) Test_getClientQueue() {
	servers := []string{"localhost:12345"}
	cp := NewClientPool(servers, nil, s.logger)
	cq := cp.getClientQueue()
	s.Equal(len(servers), len(cq))
	s.Equal(len(servers), cap(cq))
}

func (s *ClientPoolTestSuite) Test_addServer() {
	server := "localhost:12345"
	cp := NewClientPool([]string{}, nil, s.logger)
	s.False(cp.hasServers())
	cp.addServer(server)
	s.True(cp.hasServers())
	s.True(cp.hasServer(server))
	s.Equal(len(cp.getClientQueue()), 1)
}

func (s *ClientPoolTestSuite) Test_do() {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte{})
		if err != nil {
			http.Error(w, "Unable to write response", http.StatusInternalServerError)
			return
		}
	}))
	cp := NewClientPool([]string{server.URL}, nil, s.logger)
	cq := cp.getClientQueue()
	client := <-cq
	req, err := http.NewRequest("GET", "/test", nil)
	s.NoError(err)
	reqCtx, cancelFunc := context.WithTimeout(req.Context(), 1*time.Second)
	defer cancelFunc()
	resp, err := cp.do(client, req, reqCtx)
	s.NoError(err)
	s.NotNil(resp)
	s.Equal(calls, 1)
}

func (s *ClientPoolTestSuite) Test_DoRRLoadBalanced_HitAtFirst() {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte{})
		if err != nil {
			http.Error(w, "Unable to write response", http.StatusInternalServerError)
			return
		}
	}))
	cp := NewClientPool([]string{server.URL}, nil, s.logger)
	req, err := http.NewRequest("GET", "/test", nil)
	s.NoError(err)
	resp, retries, err := cp.DoRRLoadBalanced(req, context.Background())
	s.NoError(err)
	s.NotNil(resp)
	s.Equal(0, retries)
	s.Equal(1, calls)
}

func (s *ClientPoolTestSuite) Test_DoRRLoadBalanced_HitAtSecond() {
	calls := 0
	alreadyFail := false
	handlerFn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		code := http.StatusInternalServerError
		// after fail first request, fail the next one
		if alreadyFail {
			code = http.StatusOK
		} else {
			alreadyFail = true
		}
		w.WriteHeader(code)
		_, err := w.Write([]byte{})
		if err != nil {
			http.Error(w, "Unable to write response", http.StatusInternalServerError)
			return
		}
	})
	serverOne := httptest.NewServer(handlerFn)
	serverTwo := httptest.NewServer(handlerFn)
	cp := NewClientPool([]string{serverOne.URL, serverTwo.URL}, nil, s.logger)
	req, err := http.NewRequest("GET", "/test", nil)
	s.NoError(err)
	resp, retries, err := cp.DoRRLoadBalanced(req, context.Background())
	s.NoError(err)
	s.NotNil(resp)
	s.Equal(1, retries, "retries should be 1")
	s.Equal(2, calls, "servers must be call 2 times in total")
}

func (s *ClientPoolTestSuite) Test_DoRRLoadBalanced_HitMaxRetries() {
	calls := 0
	handlerFn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte{})
		if err != nil {
			s.logger.Error().Err(err).Msg("failed to write response")
			return
		}
	})
	serverOne := httptest.NewServer(handlerFn)
	serverTwo := httptest.NewServer(handlerFn)
	opts := NewDefaultClientPoolOptions()
	opts.MaxRetries = 4
	cp := NewClientPool([]string{serverOne.URL, serverTwo.URL}, opts, s.logger)
	req, err := http.NewRequest("GET", "/test", nil)
	s.Nil(err)
	resp, retries, err := cp.DoRRLoadBalanced(req, context.Background())
	s.Nil(resp)
	s.Equal(opts.MaxRetries, retries, "maximum number of retries is not respected")
	s.Equal(opts.MaxRetries, calls-1, "servers must be called same amount of retries")
}

func (s *ClientPoolTestSuite) Test_DoRRLoadBalanced_HitBackoff() {
	calls := 0
	handlerFn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte{})
		if err != nil {
			http.Error(w, "Unable to write response", http.StatusInternalServerError)
			return
		}
	})
	serverOne := httptest.NewServer(handlerFn)
	serverTwo := httptest.NewServer(handlerFn)
	opts := NewDefaultClientPoolOptions()
	opts.MaxRetries = 3
	cp := NewClientPool([]string{serverOne.URL, serverTwo.URL}, opts, s.logger)
	ms := &MockClientBackoff{}
	// it should be called twice
	ms.On("Duration", 0).Times(1)
	cp.sleeper = ms
	req, err := http.NewRequest("GET", "/test", nil)
	s.Nil(err)
	resp, retries, err := cp.DoRRLoadBalanced(req, context.Background())
	s.Nil(resp)
	s.Equal(opts.MaxRetries, retries, "maximum number of retries is not respected")
	// addition 1 because the first call is always done and is not count as a retry
	s.Equal(opts.MaxRetries, calls-1, "servers must be called one more time than retries, because first is not a retry")
	ms.AssertExpectations(s.T())
}

func (s *ClientPoolTestSuite) Test_ReplicateRequest_GetExpectedAmountOfResponsesOnly() {
	calls := 0
	expectedReplicas := 3
	handlerFn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(&types.QueryHeightOutput{Height: "10"})
		if err != nil {
			http.Error(w, "Unable to write response to JSON", http.StatusInternalServerError)
			return
		}
	})

	serverOne := httptest.NewServer(handlerFn)
	serverTwo := httptest.NewServer(handlerFn)
	serverThree := httptest.NewServer(handlerFn)
	// add one more server to check it will not call more than 3 times that is the expected number of successful responses
	serverFour := httptest.NewServer(handlerFn)

	opts := NewDefaultClientPoolOptions()
	opts.MaxRetries = 1

	cp := NewClientPool([]string{serverOne.URL, serverTwo.URL, serverThree.URL, serverFour.URL}, opts, s.logger)
	req, err := http.NewRequest("GET", "/test", nil)
	s.Nil(err)
	resp, _, err := cp.ReplicateRequest(req, context.Background(), expectedReplicas)
	// if no err, then we can read the responses, that in this test should be 3
	s.Nil(err)
	s.NotNil(resp)
	s.GreaterOrEqual(len(resp), expectedReplicas, "expected number of success responses is 3")
}

func (s *ClientPoolTestSuite) Test_ReplicateRequest_GetClientsLenResponses() {
	calls := 0
	replicas := 3
	handlerFn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(&types.QueryHeightOutput{Height: "10"})
		if err != nil {
			http.Error(w, "Unable to write response to JSON", http.StatusInternalServerError)
			return
		}
	})

	serverOne := httptest.NewServer(handlerFn)
	serverTwo := httptest.NewServer(handlerFn)
	// we ask for 3 responses but provide only 2 servers, so it should return only 2 responses
	// does not make sense call again a server that already answer
	servers := []string{serverOne.URL, serverTwo.URL}

	opts := NewDefaultClientPoolOptions()
	opts.MaxRetries = 1

	cp := NewClientPool(servers, opts, s.logger)
	req, err := http.NewRequest("GET", "/test", nil)
	s.Nil(err)
	resp, respError, err := cp.ReplicateRequest(req, context.Background(), replicas)
	s.Nil(err)
	s.Nil(respError)
	s.NotNil(resp)
	s.Equal(0, len(respError))
	s.Equal(len(resp), len(servers), "expected number of success responses should be same of servers")
	s.Equal(len(servers), calls, "servers must be called once each")
}

// Test_ReplicateRequest_GetAvailableAnswersOnly ask for a number of responses that will not be able to be reach
// because many of the servers return an error, so it will provide the amount of response that it could get
func (s *ClientPoolTestSuite) Test_ReplicateRequest_GetAvailableAnswersOnly() {
	calls := 0
	erroredCalls := 0
	successCalls := 0
	replicas := 3
	handlerFn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		successCalls++
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(&types.QueryHeightOutput{Height: "10"})
		if err != nil {
			http.Error(w, "Unable to write response to JSON", http.StatusInternalServerError)
			return
		}
	})
	errHandlerFn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		erroredCalls++
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte{})
		if err != nil {
			http.Error(w, "Unable to write response", http.StatusInternalServerError)
			return
		}
	})

	serverOne := httptest.NewServer(errHandlerFn)
	serverTwo := httptest.NewServer(errHandlerFn)
	serverThree := httptest.NewServer(handlerFn)
	serverFour := httptest.NewServer(handlerFn)
	servers := []string{serverOne.URL, serverTwo.URL, serverThree.URL, serverFour.URL}

	opts := NewDefaultClientPoolOptions()
	opts.MaxRetries = 1

	cp := NewClientPool(servers, opts, s.logger)
	req, err := http.NewRequest("GET", "/test", nil)
	s.Nil(err)
	resp, respError, err := cp.ReplicateRequest(req, context.Background(), replicas)
	s.Nil(err)
	s.NotNil(resp)
	s.NotNil(respError)
	s.Equal(erroredCalls, len(respError))
	s.Equal(successCalls, len(resp), "expected number of response should be the same amount of successCalls because we force 2 servers to fail")
	s.Equal(len(servers), calls, "servers must be called once each")
}

// Test_ReplicateRequest_EveryClientReturnsErrors call to every server and only get errors from them that are 5xx or Unknown
func (s *ClientPoolTestSuite) Test_ReplicateRequest_EveryClientReturnsErrors() {
	calls := 0
	replicas := 3
	errHandlerFn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte{})
		if err != nil {
			http.Error(w, "Unable to write response", http.StatusInternalServerError)
			return
		}
	})

	serverOne := httptest.NewServer(errHandlerFn)
	serverTwo := httptest.NewServer(errHandlerFn)
	serverThree := httptest.NewServer(errHandlerFn)
	serverFour := httptest.NewServer(errHandlerFn)
	servers := []string{serverOne.URL, serverTwo.URL, serverThree.URL, serverFour.URL}

	opts := NewDefaultClientPoolOptions()
	opts.MaxRetries = 1

	cp := NewClientPool(servers, opts, s.logger)
	req, err := http.NewRequest("GET", "/test", nil)
	s.Nil(err)
	resp, respError, err := cp.ReplicateRequest(req, context.Background(), replicas)
	s.NotNil(err)
	s.ErrorIs(err, ErrUnableToGetReplicateResponse)
	s.NotNil(respError)
	s.Nil(resp)
	s.Equal(calls, len(respError))
	s.Equal(len(servers), calls, "servers must be called once each")
}

func TestClientPoolTestSuite(t *testing.T) {
	// run all the tests
	suite.Run(t, new(ClientPoolTestSuite))
}
