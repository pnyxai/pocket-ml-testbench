package pocket_rpc

import (
	"context"
	"errors"
	"fmt"
	"github.com/alitto/pond"
	"github.com/puzpuzpuz/xsync"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"golang.org/x/time/rate"
	"math"
	"net/http"
	"net/url"
	"packages/utils"
	"sync"
	"sync/atomic"
	"time"
)

type ReplicatedResponse struct {
	Response *http.Response
	Error    error
}

type Backoff interface {
	Duration(n int) time.Duration
}

type ClientBackoff struct {
	min time.Duration
	max time.Duration
}

func (b *ClientBackoff) Duration(n int) time.Duration {
	if n == 0 {
		return b.min
	}
	duration := time.Duration(math.Pow(2, float64(n))) * b.min
	if duration > b.max {
		return b.max
	}
	return duration
}

func NewBackoff(min, max time.Duration) *ClientBackoff {
	return &ClientBackoff{min: min, max: max}
}

type MockClientBackoff struct {
	mock.Mock
}

func (ms *MockClientBackoff) Duration(n int) time.Duration {
	ms.Called(n)
	return time.Duration(n)
}

type Client struct {
	url    string
	client *http.Client
}

func (c Client) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

type RotatingClientQueue chan *Client

func (rc RotatingClientQueue) Get() *Client {
	return <-rc
}

func (rc RotatingClientQueue) Set(client *Client) {
	rc <- client
}

type ClientPoolOptions struct {
	MaxRetries    int
	ReqPerSec     int
	MinBackoff    time.Duration
	MaxBackoff    time.Duration
	RetryOnUnkErr bool
	RetryOn4xx    bool
	RetryOn5xx    bool
}

type ClientPool struct {
	clients *xsync.MapOf[string, *Client]
	sleeper Backoff
	logger  *zerolog.Logger
	opts    *ClientPoolOptions
}

func NewDefaultClientPoolOptions() *ClientPoolOptions {
	return &ClientPoolOptions{
		MaxRetries:    1,
		ReqPerSec:     10,
		MinBackoff:    1 * time.Second,
		MaxBackoff:    10 * time.Second,
		RetryOnUnkErr: true,
		RetryOn4xx:    true,
		RetryOn5xx:    true,
	}
}

func NewClientPool(servers []string, opts *ClientPoolOptions, logger *zerolog.Logger) *ClientPool {
	if opts == nil {
		opts = NewDefaultClientPoolOptions()
	}

	cp := ClientPool{
		clients: xsync.NewMapOf[*Client](),
		sleeper: NewBackoff(opts.MinBackoff, opts.MaxBackoff),
		logger:  logger,
		opts:    opts,
	}

	for _, server := range servers {
		cp.addServer(server)
	}

	return &cp
}

func (cp *ClientPool) DoRRLoadBalanced(req *http.Request, ctx context.Context) (resp *http.Response, retries int, err error) {
	rc := cp.getClientQueue()
	clients := len(rc)
	backoffCounter := 0

	for {
		if len(rc) == 0 {
			// on certain errors the clients are not set back because they will not be able to be used, so
			// this channel could be empty at some point
			return nil, retries, fmt.Errorf("there is no more clients to get a response")
		}

		if retries > clients {
			// apply backoff only when the retries are more than the available clients
			time.Sleep(cp.sleeper.Duration(backoffCounter))
			backoffCounter++
		}

		client := rc.Get()

		newURL, parseError := url.Parse(client.url)
		if parseError != nil {
			// do not add the client back because try again will not give a different result for the error
			cp.logger.Error().Err(parseError).Str("server", client.url).Msg("failed to parse server url")
			continue
		}

		// Keep the same path and other components
		newURL.Path = req.URL.Path
		newURL.RawQuery = req.URL.RawQuery
		newURL.Fragment = req.URL.Fragment

		req.URL = newURL

		resp, err = cp.do(client, req, ctx)

		if err != nil {
			if cp.opts.RetryOnUnkErr {
				if retries >= cp.opts.MaxRetries {
					err = fmt.Errorf("maximum retries exceeded")
					resp = nil
					break
				}
				// mark retry and add a client back to the rotated client queue to retry with it or another one
				retries++
				rc.Set(client)
				continue
			}
			break
		}

		if string(resp.Status[0]) == "4" {
			if cp.opts.RetryOn4xx {
				if retries >= cp.opts.MaxRetries {
					err = fmt.Errorf("maximum retries exceeded")
					resp = nil
					break
				}
				// mark retry and add a client back to the rotated client queue to retry with it or another one
				retries++
				rc.Set(client)
				continue
			}
			break
		}

		if string(resp.Status[0]) == "5" {
			if cp.opts.RetryOn5xx {
				if retries >= cp.opts.MaxRetries {
					err = fmt.Errorf("maximum retries exceeded")
					resp = nil
					break
				}
				// mark retry and add a client back to the rotated client queue to retry with it or another one
				retries++
				rc.Set(client)
				continue
			}
		}
		break
	}

	return
}

func (cp *ClientPool) ReplicateRequest(r *http.Request, ctx context.Context, maxReplicas int) (responses []*http.Response, _errors []error, e error) {
	rc := cp.getClientQueue()
	clients := len(rc)
	replicas := utils.MinInt(clients, maxReplicas)
	responseCounter := int32(0)
	responseChan := make(chan *ReplicatedResponse, clients)
	// so we try to hit them asap
	wPool := pond.New(clients, clients, pond.MinWorkers(replicas), pond.Strategy(pond.Eager()))
	cancellableCtx, cancel := context.WithCancel(context.Background())
	group, groupCtx := wPool.GroupContext(cancellableCtx)
	replicasReach := errors.New("replicas reached")
	var once sync.Once
	worker := func(client *Client) func() error {
		return func() error {
			req := *r
			rr := ReplicatedResponse{}

			newURL, parseError := url.Parse(client.url)
			if parseError != nil {
				// we do not add the client back because no matter how much time we try parse again the result will be the same
				cp.logger.Error().Err(parseError).Str("server", client.url).Msg("failed to parse server url")
				rr.Error = parseError
				responseChan <- &rr
				return nil
			}

			// Keep the same path and other components
			newURL.Path = r.URL.Path
			newURL.RawQuery = r.URL.RawQuery
			newURL.Fragment = r.URL.Fragment

			req.URL = newURL

			resp, err := cp.do(client, &req, groupCtx)

			select {
			case <-ctx.Done():
				return replicasReach
			default:
				if err != nil {
					if ctx.Err() != nil {
						return replicasReach
					}
					rr.Error = err
					responseChan <- &rr
					return nil
				}

				if resp.StatusCode >= http.StatusBadRequest {
					rr.Error = fmt.Errorf(resp.Status)
					responseChan <- &rr
					return nil
				}

				responseChan <- &rr

				var c int32
				if rr.Response != nil && rr.Error == nil {
					c = atomic.AddInt32(&responseCounter, 1)
				}

				if c >= int32(replicas) {
					once.Do(func() {
						cancel()
					})
					return replicasReach
				}

				rr.Response = resp
				return nil
			}
		}
	}

	cp.clients.Range(func(_ string, c *Client) bool {
		group.Submit(worker(c))
		return true
	})

	err := group.Wait()
	if err != nil && !errors.Is(err, replicasReach) {
		e = err
		return
	}

	wPool.StopAndWait()

	close(responseChan)

	for rr := range responseChan {
		if rr.Error != nil {
			_errors = append(_errors, rr.Error)
		}
		if rr.Response != nil {
			responses = append(responses, rr.Response)
		}
	}

	if (len(_errors) > 0 && len(responses) == 0) || len(responses) == 0 {
		e = ErrUnableToGetReplicateResponse
		return
	}

	return
}

func (cp *ClientPool) do(client *Client, req *http.Request, ctx context.Context) (*http.Response, error) {
	lim := rate.NewLimiter(rate.Every(time.Second/time.Duration(cp.opts.ReqPerSec)), 1)

	// Wait for the rate limiter
	err := lim.Wait(ctx)
	if err != nil {
		return nil, err
	}

	newURL, err := url.Parse(client.url)
	if err != nil {
		return nil, err
	}

	// Keep the same path and other components
	newURL.Path = req.URL.Path
	newURL.RawQuery = req.URL.RawQuery
	newURL.Fragment = req.URL.Fragment
	req.URL = newURL

	// Make the request, passing in the new context
	return client.Do(req.WithContext(ctx))
}

func (cp *ClientPool) getClientQueue() RotatingClientQueue {
	rc := make(RotatingClientQueue, cp.clients.Size())
	cp.clients.Range(func(_ string, client *Client) bool {
		rc.Set(client)
		return true
	})
	return rc
}

func (cp *ClientPool) addServer(url string) {
	// if already there, omit
	if _, ok := cp.clients.Load(url); ok {
		return
	}

	cp.clients.Store(url, &Client{
		url: url,
		client: &http.Client{
			Timeout: 1 * time.Minute,
		},
	})
}

func (cp *ClientPool) hasServer(url string) bool {
	_, ok := cp.clients.Load(url)
	return ok
}

func (cp *ClientPool) hasServers() bool {
	return cp.clients.Size() > 0
}
