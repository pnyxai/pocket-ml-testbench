package pocket_shannon

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"packages/pocket_shannon/types"

	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sdk "github.com/pokt-network/shannon-sdk"
	sdktypes "github.com/pokt-network/shannon-sdk/types"
)

// RelayErrorCode is enum of possible relay error codes
type RelayErrorCode int

const (
	// AppNotFoundError error when app is not found
	// AppNotFoundError = 45
	// DuplicateProofError error when proof is used multiple times
	// DuplicateProofError RelayErrorCode = 37
	// EmptyPayloadDataError error when sent payload is empty
	// EmptyPayloadDataError RelayErrorCode = 25
	// EvidencedSealedError error when evidence is sealed, either max relays reached or claim already submitted
	// EvidencedSealedError RelayErrorCode = 90
	// HTTPExecutionError error when http request failed
	HTTPExecutionError RelayErrorCode = 28
	// InvalidBlockHeightError error when sent block height is invalid
	// InvalidBlockHeightError RelayErrorCode = 60
	// InvalidSessionError error when session is invalid
	InvalidSessionError RelayErrorCode = 14
	// OutOfSyncRequestError error when request is not on sync
	// OutOfSyncRequestError RelayErrorCode = 75
	// OverServiceError error when request exceeds service capacity
	// OverServiceError RelayErrorCode = 71
	// RequestHashError error when hash is not correct
	// RequestHashError RelayErrorCode = 74
	// UnsupportedBlockchainError error when sent blockchain is not supported yet
	// UnsupportedBlockchainError RelayErrorCode = 76

	// New ones, defined here
	UnsignedRequestBuildError RelayErrorCode = 20
	RequestSigningError       RelayErrorCode = 21
	InvalidRelayError         RelayErrorCode = 22
)

// RPCError represents error output from RPC request
type RPCError struct {
	Code    RelayErrorCode `json:"code"`
	Message string         `json:"message"`
}

// endpoint is used to fulfill a protocol package Endpoint using a Shannon SupplierEndpoint.
// An endpoint is identified by combining its supplier address and its URL, because
// in Shannon a supplier can have multiple endpoints for a service.
type Endpoint struct {
	Supplier string
	Url      string

	// TODO_IMPROVE: If the same endpoint is in the session of multiple apps at the same time,
	// the first app will be chosen. A randomization among the apps in this (unlikely) scenario
	// may be needed.
	// session is the active session corresponding to the app, of which the endpoint is a member.
	Session sessiontypes.Session
}

// TODO_MVP(@adshmh): replace EndpointAddr with a URL; a single URL should be treated the same regardless of the app to which it is attached.
// For protocol-level concerns: the (app/session, URL) should be taken into account; e.g. a healthy endpoint may have been maxed out for a particular app.
// For QoS-level concerns: only the URL of the endpoint matters; e.g. an unhealthy endpoint should be skipped regardless of the app/session to which it is attached.
func (e Endpoint) Addr() types.EndpointAddr {
	return types.EndpointAddr(fmt.Sprintf("%s-%s", e.Supplier, e.Url))
}

func (e Endpoint) PublicURL() string {
	return e.Url
}

func (e Endpoint) GetSupplier() string {
	return e.Supplier
}

func (e Endpoint) GetSession() sessiontypes.Session {
	return e.Session
}

type RelayRequestSigner struct {
	AccountClient sdk.AccountClient
	PrivateKeyHex string
}

func (s *RelayRequestSigner) SignRelayRequest(req *servicetypes.RelayRequest, app apptypes.Application) (*servicetypes.RelayRequest, error) {
	ring := sdk.ApplicationRing{
		Application:      app,
		PublicKeyFetcher: &s.AccountClient,
	}

	sdkSigner := sdk.Signer{PrivateKeyHex: s.PrivateKeyHex}
	req, err := sdkSigner.Sign(context.Background(), req, ring)
	if err != nil {
		return nil, fmt.Errorf("SignRequest: error signing relay request: %w", err)
	}

	return req, nil
}

func SendRelay(payload types.Payload, selectedEndpoint Endpoint, serviceID types.ServiceID, fullNode LazyFullNode, relayRequestSigner RelayRequestSigner) (*servicetypes.RelayResponse, RPCError) {

	session := selectedEndpoint.GetSession()
	if session.Application == nil {
		errOut := RPCError{
			Code:    InvalidSessionError,
			Message: fmt.Sprintf("sendRelay: nil app on session %s for service %s", session.SessionId, serviceID),
		}
		return nil, errOut
	}
	app := *session.Application

	relayRequest, err := buildUnsignedRelayRequest(selectedEndpoint, session, []byte(payload.Data), payload.Path, payload.Method)
	if err != nil {
		errOut := RPCError{
			Code:    UnsignedRequestBuildError,
			Message: fmt.Sprintf("sendRelay: error building unsigned relay request for app %s: %w", app.Address, err),
		}
		return nil, errOut
	}

	signedRelayReq, err := signRelayRequest(relayRequest, app, relayRequestSigner)
	if err != nil {
		errOut := RPCError{
			Code:    RequestSigningError,
			Message: fmt.Sprintf("sendRelay: error signing the relay request for app %s: %w", app.Address, err),
		}
		return nil, errOut
	}

	ctxWithTimeout, cancelFn := context.WithTimeout(context.Background(), payload.Timeout)
	defer cancelFn()

	responseBz, err := sendHttpRelay(ctxWithTimeout, selectedEndpoint.PublicURL(), signedRelayReq)
	if err != nil {
		errOut := RPCError{
			Code:    HTTPExecutionError,
			Message: fmt.Sprintf("relay: error sending request to endpoint %s: %w", selectedEndpoint.PublicURL(), err),
		}
		return nil, errOut
	}

	// Validate the response
	response, err := fullNode.ValidateRelayResponse(sdk.SupplierAddress(selectedEndpoint.GetSupplier()), responseBz)
	if err != nil {

		errOut := RPCError{
			Code:    InvalidRelayError,
			Message: fmt.Sprintf("relay: error verifying the relay response for app %s, endpoint %s: %w", app.Address, selectedEndpoint.PublicURL(), err),
		}
		return nil, errOut
	}

	return response, RPCError{Code: 0, Message: "OK"}
}

// sendHttpRelay sends the relay request to the supplier at the given URL using an HTTP Post request.
func sendHttpRelay(
	ctx context.Context,
	supplierUrlStr string,
	relayRequest *servicetypes.RelayRequest,
) (relayResponseBz []byte, err error) {
	_, err = url.Parse(supplierUrlStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing relay url: %w", err)
	}

	relayRequestBz, err := relayRequest.Marshal()
	if err != nil {
		return nil, fmt.Errorf("error marshaling relay request: %w", err)
	}

	relayHTTPRequest, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost, // This is the method to the POKT node
		supplierUrlStr,
		io.NopCloser(bytes.NewReader(relayRequestBz)),
	)
	if err != nil {
		return nil, fmt.Errorf("error creating relay with context: %w", err)
	}

	relayHTTPRequest.Header.Add("Content-Type", "application/json")

	httpClient := &http.Client{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 0 * time.Second,
			TLSHandshakeTimeout:   0 * time.Second,
			IdleConnTimeout:       0 * time.Second,
		},
	}
	relayHTTPResponse, err := httpClient.Do(relayHTTPRequest)

	// relayHTTPResponse, err := http.DefaultClient.Do(relayHTTPRequest)

	if err != nil {
		return nil, fmt.Errorf("error sending relay: %w", err)
	}
	defer relayHTTPResponse.Body.Close()

	// buf := bufio.NewReader(relayHTTPResponse.Body)
	// relayResponseBz, err = io.ReadAll(buf)

	var buf bytes.Buffer
	_, err = io.Copy(&buf, relayHTTPResponse.Body)
	if err != nil {
		return nil, fmt.Errorf("error copying response: %w", err)
	}
	relayResponseBz = buf.Bytes()

	return

	// return io.ReadAll(relayHTTPResponse.Body)
}

func signRelayRequest(unsignedRelayReq *servicetypes.RelayRequest, app apptypes.Application, relayRequestSigner RelayRequestSigner) (*servicetypes.RelayRequest, error) {
	// Verify the relay request's metadata, specifically the session header.
	// Note: cannot use the RelayRequest's ValidateBasic() method here, as it looks for a signature in the struct, which has not been added yet at this point.
	meta := unsignedRelayReq.GetMeta()

	if meta.GetSessionHeader() == nil {
		return nil, errors.New("signRelayRequest: relay request is missing session header")
	}

	sessionHeader := meta.GetSessionHeader()
	if err := sessionHeader.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("signRelayRequest: relay request session header is invalid: %w", err)
	}

	// Sign the relay request using the selected app's private key
	return relayRequestSigner.SignRelayRequest(unsignedRelayReq, app)
}

// buildUnsignedRelayRequest builds a ready-to-sign RelayRequest struct using the supplied endpoint, session, and payload.
// The returned RelayRequest can be signed and sent to the endpoint to receive the endpoint's response.
func buildUnsignedRelayRequest(endpoint Endpoint, session sessiontypes.Session, payload []byte, path string, method string) (*servicetypes.RelayRequest, error) {
	// If the path is not empty (ie. for a REST service request), append it to the endpoint's URL
	url := endpoint.PublicURL()
	if path != "" {
		url = fmt.Sprintf("%s%s", url, path)
	}
	// TODO_TECHDEBT: need to select the correct underlying request (HTTP, etc.) based on the selected service.
	jsonRpcHttpReq, err := shannonJsonRpcHttpRequest(payload, url, method)
	if err != nil {
		return nil, fmt.Errorf("error building a JSONRPC HTTP request for url %s: %w", url, err)
	}

	relayRequest, err := embedHttpRequest(jsonRpcHttpReq)
	if err != nil {
		return nil, fmt.Errorf("error embedding a JSONRPC HTTP request for url %s: %w", url, err)
	}

	// TODO_MVP(@adshmh): use the new `FilteredSession` struct provided by the Shannon SDK to get the session and the endpoint.
	relayRequest.Meta = servicetypes.RelayRequestMetadata{
		SessionHeader:           session.Header,
		SupplierOperatorAddress: string(endpoint.GetSupplier()),
	}

	return relayRequest, nil
}

// serviceRequestPayload is the contents of the request received by the underlying service's API server.
func shannonJsonRpcHttpRequest(serviceRequestPayload []byte, url string, method string) (*http.Request, error) {
	jsonRpcServiceReq, err := http.NewRequest(method, url, io.NopCloser(bytes.NewReader(serviceRequestPayload)))
	if err != nil {
		return nil, fmt.Errorf("shannonJsonRpcHttpRequest: failed to create a new HTTP request for url %s: %w", url, err)
	}

	jsonRpcServiceReq.Header.Set("Content-Type", "application/json")
	return jsonRpcServiceReq, nil
}

func embedHttpRequest(reqToEmbed *http.Request) (*servicetypes.RelayRequest, error) {
	_, reqToEmbedBz, err := sdktypes.SerializeHTTPRequest(reqToEmbed)
	if err != nil {
		return nil, fmt.Errorf("embedHttpRequest: failed to Serialize HTTP Request for URL %s: %w", reqToEmbed.URL, err)
	}

	return &servicetypes.RelayRequest{
		Payload: reqToEmbedBz,
	}, nil
}
