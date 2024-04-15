package pocket_rpc

import (
	"context"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
	"math"
	"net/http"
	"net/url"
	"time"
)

type Backoff struct {
	min time.Duration
	max time.Duration
}

func NewBackoff(min, max time.Duration) *Backoff {
	return &Backoff{min: min, max: max}
}

func (b *Backoff) Duration(n int) time.Duration {
	if n == 0 {
		return b.min
	}
	duration := time.Duration(math.Pow(2, float64(n))) * b.min
	if duration > b.max {
		return b.max
	}
	return duration
}

type Client struct {
	url    string
	client *http.Client
}

func (c Client) Do(req *http.Request) (*http.Response, error) {
	return c.client.Do(req)
}

type ClientPoolOptions struct {
	MaxRetries int
	ReqPerSec  int
	MinBackoff time.Duration
	MaxBackoff time.Duration
}

type ClientPool struct {
	clients chan *Client
	logger  *zerolog.Logger
	opts    *ClientPoolOptions
}

func NewDefaultClientPoolOptions() *ClientPoolOptions {
	return &ClientPoolOptions{
		MaxRetries: 3,
		ReqPerSec:  10,
		MinBackoff: 10 * time.Second,
		MaxBackoff: 60 * time.Second,
	}
}

func NewClientPool(servers []string, opts *ClientPoolOptions, logger *zerolog.Logger) *ClientPool {
	if opts == nil {
		opts = NewDefaultClientPoolOptions()
	}

	cp := ClientPool{
		clients: make(chan *Client, len(servers)),
		logger:  logger,
		opts:    opts,
	}

	for _, server := range servers {
		cp.setClient(&Client{url: server, client: &http.Client{Timeout: 1 * time.Minute}})
	}

	return &cp
}

func (cp *ClientPool) DoRRLoadBalanced(req *http.Request, timeout time.Duration) (*http.Response, int, error) {
	retries := 0
	backoff := NewBackoff(cp.opts.MinBackoff, cp.opts.MaxBackoff) // adjust min and max duration as necessary
	clients := len(cp.clients)
	allowBackoff := clients >= cp.opts.MaxRetries
	backoffCounter := 0

	for {
		if retries >= cp.opts.MaxRetries {
			return nil, retries, fmt.Errorf("maximum retries exceeded")
		}

		if allowBackoff && retries > clients {
			// apply backoff only when the retries are more than the available clients
			time.Sleep(backoff.Duration(backoffCounter))
			backoffCounter++
		}

		client := cp.getClient()

		newURL, parseError := url.Parse(client.url)
		if parseError != nil {
			cp.setClient(client)
			cp.logger.Error().Err(parseError).Str("server", client.url).Msg("failed to parse server url")
			continue
		}

		// Keep the same path and other components
		newURL.Path = req.URL.Path
		newURL.RawQuery = req.URL.RawQuery
		newURL.Fragment = req.URL.Fragment

		req.URL = newURL

		resp, doError := cp.do(client, req, timeout)

		if doError != nil || resp.StatusCode >= 500 {
			if doError != nil && !errors.Is(doError, context.DeadlineExceeded) {
				// if no timeout and no response, then we break and return
				cp.setClient(client)
				return resp, retries, doError
			}

			retries++
			cp.setClient(client)

			continue
		}

		cp.setClient(client)
		return resp, retries, nil
	}
}

func (cp *ClientPool) do(client *Client, req *http.Request, timeout time.Duration) (*http.Response, error) {
	lim := rate.NewLimiter(rate.Every(time.Second/time.Duration(cp.opts.ReqPerSec)), 1)

	// Create a new context with a timeout
	reqCtx, cancelFunc := context.WithTimeout(req.Context(), timeout)
	defer cancelFunc() // Ensure the context is canceled even if the http.Client.Do function panics

	// Wait for the rate limiter
	err := lim.Wait(reqCtx)
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
	return client.Do(req.WithContext(reqCtx))
}

func (cp *ClientPool) getClient() *Client {
	return <-cp.clients
}

func (cp *ClientPool) setClient(client *Client) {
	cp.clients <- client
}
