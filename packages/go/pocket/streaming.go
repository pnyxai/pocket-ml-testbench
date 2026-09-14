package pocket

import (
	"context"
	"fmt"
	"io"

	"github.com/pokt-network/pocket-ap/domain"
	"github.com/pokt-network/pocket-ap/relay"
)

// StreamDelimiter separates the signed batches inside a streaming relay
// response. It is a relay-miner wire constant, re-exported here so a caller
// driving its own transport can split a body without importing pocket-ap.
//
// You only need it on the BuildSignedRequest path. SendRelayStream handles the
// framing for you, and handling it yourself is easy to get subtly wrong — see
// the truncation note on ValidateResponseChunk.
const StreamDelimiter = relay.StreamDelimiter

// --- high level --------------------------------------------------------------

// SendRelayStream runs one relay against the named supplier and hands each
// validated batch to onChunk as it arrives.
//
// It is a superset of SendRelay, because whether a response streams is not the
// caller's decision and not knowable in advance: the BACKEND says so with its
// Content-Type (text/event-stream or application/x-ndjson), the relay miner
// copies that onto its reply and batch-signs the body instead of buffering it.
// A non-streaming response simply yields exactly one chunk. So anything that
// might receive an SSE answer — an inference backend asked for stream:true —
// must call this rather than SendRelay, which would hand the validator a body
// containing several concatenated RelayResponses and fail on it.
//
// Each chunk's Ms is the time from sending the relay to that chunk arriving, so
// the first one measures time-to-first-token.
//
// An error returned by onChunk is passed back to the caller unchanged. Any other
// failure is a *RelayError.
func (c *Client) SendRelayStream(
	ctx context.Context,
	appAddress string,
	serviceID ServiceID,
	supplierAddress string,
	payload Payload,
	onChunk func(Response) error,
) error {
	relayCtx, cancelFn := c.relayContext(ctx, appAddress, supplierAddress, payload)
	defer cancelFn()

	probe := c.newProbe()
	relayer := relay.Relayer{
		Sessions:     probe,
		Signer:       probe,
		Validator:    probe,
		Sender:       probe,
		StreamSender: probe,
		Selector:     c.selector,
		MaxAttempts:  1,
	}

	// Kept apart from the relay's own failures so a caller's error is not
	// reported back to it as though a supplier had caused it.
	var callbackErr error

	err := relayer.RelayStream(relayCtx, domain.ServiceID(serviceID), c.rpcTypeFor(serviceID), relayInput(payload),
		func(result *domain.RelayResult) error {
			chunk := Response{
				Bytes:          result.Body,
				HTTPStatusCode: result.StatusCode,
				Ms:             probe.elapsed().Milliseconds(),
				// Per batch, not per relay: each one is separately signed by the
				// supplier, so each one has its own receipt.
				Receipt: c.receiptFrom(probe, supplierAddress),
			}
			if cbErr := onChunk(chunk); cbErr != nil {
				callbackErr = cbErr
				return cbErr
			}
			return nil
		})

	if callbackErr != nil {
		return callbackErr
	}
	if err != nil {
		return &RelayError{Stage: stageOf(probe, err), Err: err}
	}
	return nil
}

// --- primitives --------------------------------------------------------------

// SignedRelayRequest is one relay, ring-signed for one named supplier and ready
// to put on the wire.
//
// It exists so the three steps of a relay — sign once, send, validate each
// answer — can be driven separately. That is what a chunked consumer needs: the
// request is signed a single time, and every batch that comes back is verified
// on its own as it arrives.
type SignedRelayRequest struct {
	// Supplier is the operator address this relay is signed for. Pass it back to
	// ValidateResponseChunk — a signature is only meaningful against the
	// supplier that produced it.
	Supplier string
	// Url is where the signed bytes must be POSTed.
	Url string
	// Bytes is the marshaled, signed RelayRequest.
	Bytes []byte

	// rpcType is carried so SendSignedRequest can stamp the header the relay
	// miner routes on, without the caller having to know that header exists.
	rpcType domain.RPCType
}

// BuildSignedRequest signs a relay for the named supplier without sending it.
//
// payload.Timeout is ignored here — there is nothing yet to time out. Apply the
// deadline to the context you pass to SendSignedRequest, or to your own
// transport.
//
// Use this when you need to own the transport: a chunked consumer that reads the
// body itself, a test that replays the same signed bytes, a caller measuring the
// send separately. If you just want the batches, SendRelayStream is the whole
// job done correctly.
func (c *Client) BuildSignedRequest(
	ctx context.Context,
	appAddress string,
	serviceID ServiceID,
	supplierAddress string,
	payload Payload,
) (*SignedRelayRequest, error) {
	session, err := c.sessions.Session(withApp(ctx, appAddress), domain.ServiceID(serviceID))
	if err != nil {
		stage := StageSession
		if isNoApp(err) {
			stage = StageApp
		}
		return nil, &RelayError{Stage: stage, Err: err}
	}

	rpcType := c.rpcTypeFor(serviceID)
	endpoint, url, found := endpointForSupplier(session, supplierAddress, rpcType)
	if !found {
		return nil, &RelayError{Stage: StageEndpoint, Err: fmt.Errorf(
			"supplier %s serves no %s endpoint in session %s (service %s, app %s): %w",
			supplierAddress, rpcType, session.ID, serviceID, appAddress, domain.ErrNoEndpoint,
		)}
	}

	relayReqBz, err := c.signer.SignRelay(ctx, session, endpoint, rpcType, relayInput(payload))
	if err != nil {
		return nil, &RelayError{Stage: StageSigning, Err: err}
	}

	return &SignedRelayRequest{
		Supplier: supplierAddress,
		Url:      url,
		Bytes:    relayReqBz,
		rpcType:  rpcType,
	}, nil
}

// StreamResponse is a relay response still arriving.
//
// The caller MUST close Body. Header is what decides whether Body holds one
// RelayResponse or several StreamDelimiter-separated batches — see IsStreaming.
type StreamResponse struct {
	Body           io.ReadCloser
	Header         map[string][]string
	HTTPStatusCode int
}

// IsStreaming reports whether the body holds delimiter-separated signed batches
// rather than a single RelayResponse.
//
// The decision belongs to the backend, which declares it by Content-Type; the
// relay miner copies that onto its reply. A caller can neither ask for streaming
// nor opt out, so this has to be read off the response rather than assumed.
func (s *StreamResponse) IsStreaming() bool {
	return isStreamingContentType(s.Header)
}

// SendSignedRequest POSTs a signed relay and returns the response with its body
// still open, so it can be consumed as it arrives.
//
// It goes through the same HTTP client as SendRelay — pooled connections, and
// the header the relay miner routes on stamped for you. Unlike SendRelay's
// sender it carries no whole-request deadline, because that would sever a
// working token stream mid-answer; time-to-first-byte is still bounded, and the
// context is yours to give a deadline to.
func (c *Client) SendSignedRequest(ctx context.Context, req *SignedRelayRequest) (*StreamResponse, error) {
	body, header, statusCode, err := c.sender.SendStream(ctx, req.Url, req.Bytes, req.rpcType)
	if err != nil {
		return nil, &RelayError{Stage: StageSending, Err: err}
	}
	return &StreamResponse{Body: body, Header: header, HTTPStatusCode: statusCode}, nil
}

// ValidateResponseChunk verifies one signed batch from a supplier and unwraps
// the backend response inside it.
//
// A streaming body is a sequence of complete, independently signed
// RelayResponses separated by StreamDelimiter, so validating one batch is the
// same operation as validating a whole non-streaming response — which is why
// this one method serves both.
//
// ⚠️ Only hand it batches you know are complete. A batch followed by a
// delimiter is complete; a trailing fragment is complete only if the stream
// ended cleanly, and validating a truncated protobuf fails. If you are splitting
// a body yourself, hold the last unterminated fragment back until you have seen
// the reader finish without error. SendRelayStream already does this.
func (c *Client) ValidateResponseChunk(supplierAddress string, chunk []byte) (Response, error) {
	result, err := c.validator.ValidateResponse(domain.EndpointAddr(supplierAddress), chunk)
	if err != nil {
		return Response{}, &RelayError{Stage: StageValidation, Err: err}
	}
	return Response{
		Bytes:          result.Body,
		HTTPStatusCode: result.StatusCode,
		// Only the response half is available here: the caller signed the
		// request itself with BuildSignedRequest and holds those bytes, so the
		// application signature and session header are theirs to add.
		Receipt: c.receiptFrom(&relayProbe{responseBz: chunk}, supplierAddress),
	}, nil
}
