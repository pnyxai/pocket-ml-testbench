package pocket

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"strings"
	"time"

	"github.com/pokt-network/pocket-ap/domain"
	"github.com/pokt-network/pocket-ap/relay"
)

// RelayStage names the step of the pocket-ap relay flow a relay failed at.
//
// It replaces the numeric error codes this package used to carry, which were
// inherited from the old Pocket SDK and had no meaning in terms of the flow that
// actually runs now. A stage is directly actionable: it says whether the fault
// was ours (we could not sign), the network's (we could not reach the supplier),
// or the supplier's (it answered with something that does not verify).
type RelayStage int

const (
	// StageNone is the zero value, used when a relay succeeded.
	StageNone RelayStage = iota
	// StageApp : no signing key is configured for the app the relay is billed
	// to. Ours. Maps from domain.ErrNoApp.
	StageApp
	// StageSession : the session for (app, service) could not be fetched.
	StageSession
	// StageEndpoint : the named supplier is not in the session, or serves no
	// endpoint for the transport this service is configured to use. Maps from
	// domain.ErrNoEndpoint.
	StageEndpoint
	// StageSigning : building or ring-signing the relay request failed. Ours.
	StageSigning
	// StageSending : the relay never got an answer — timeout, refused
	// connection, transport error.
	StageSending
	// StageValidation : the supplier answered, but the response failed
	// signature verification or could not be unwrapped. Theirs.
	StageValidation
)

func (s RelayStage) String() string {
	switch s {
	case StageNone:
		return "none"
	case StageApp:
		return "app"
	case StageSession:
		return "session"
	case StageEndpoint:
		return "endpoint"
	case StageSigning:
		return "signing"
	case StageSending:
		return "sending"
	case StageValidation:
		return "validation"
	default:
		return "unknown"
	}
}

// RelayError is a failed relay, carrying the stage it failed at and the
// underlying pocket-ap error.
//
// Unwrap is implemented so callers can still reach for the sentinel errors
// pocket-ap defines (domain.ErrNoEndpoint, domain.ErrNoApp) with errors.Is.
type RelayError struct {
	Stage RelayStage
	Err   error
}

func (e *RelayError) Error() string {
	return fmt.Sprintf("relay failed at the %s stage: %s", e.Stage, e.Err)
}

func (e *RelayError) Unwrap() error { return e.Err }

// SendRelay runs one relay against the named supplier and returns the backend's
// response.
//
// The supplier is pinned, not selected. pocket-ap's own mechanism for that is a
// request-scoped supplier allow list on the context, which selector.Filter
// intersects with the (empty) operator policy — so this takes the supported path
// rather than reaching around the relay flow. An allow list of one also removes
// failover by construction, which is what the testbench wants: a supplier that
// cannot answer is a result to record, not something to route around.
//
// The returned Response carries the round trip time whenever we got far enough
// to send, including on failure. A non-nil error is always a *RelayError.
func (c *Client) SendRelay(
	ctx context.Context,
	appAddress string,
	serviceID ServiceID,
	supplierAddress string,
	payload Payload,
) (Response, error) {
	var response Response

	relayCtx, cancelFn := c.relayContext(ctx, appAddress, supplierAddress, payload)
	defer cancelFn()

	// A Relayer per relay, so the probe below is private to it. The struct only
	// holds shared pointers, so this costs an allocation, and it is the same
	// shape `pocket-ap call` uses to report on a single relay.
	probe := c.newProbe()
	relayer := relay.Relayer{
		Sessions:    probe,
		Signer:      probe,
		Validator:   probe,
		Sender:      probe,
		Selector:    c.selector,
		MaxAttempts: 1,
	}

	result, err := relayer.Relay(relayCtx, domain.ServiceID(serviceID), c.rpcTypeFor(serviceID), relayInput(payload))
	response.Ms = probe.latency.Milliseconds()

	if err != nil {
		return response, &RelayError{Stage: stageOf(probe, err), Err: err}
	}

	response.Bytes = result.Body
	response.HTTPStatusCode = result.StatusCode
	response.Receipt = c.receiptFrom(probe, supplierAddress, result.Body)

	return response, nil
}

// stageOf works out where a relay died.
//
// The probe knows every stage it actually entered. Endpoint selection is the one
// step it does not wrap — that is the Selector's, and the Relayer runs it before
// handing anything to a seam — so it is recognised from the sentinel pocket-ap
// returns for it instead.
func stageOf(probe *relayProbe, err error) RelayStage {
	switch {
	case errors.Is(err, domain.ErrNoApp):
		return StageApp
	case probe.stage != StageNone:
		return probe.stage
	case errors.Is(err, domain.ErrNoEndpoint), errors.Is(err, domain.ErrUnsupportedType):
		return StageEndpoint
	default:
		return StageSending
	}
}

// relayProbe implements all four relay seams by delegating to the real ones,
// recording which stage failed and how long the supplier took to answer.
//
// It exists because relay.Relayer reports one wrapped error and no timing, and
// the testbench's whole job is to say which of "we could not sign", "we could
// not reach them" and "they answered with garbage" happened. relay.Observer
// carries some of that, but it is attached to the Selector, which is shared by
// every concurrent relay — there is no way to attribute a callback to one of
// them. Wrapping the seams of a per-relay Relayer has no such ambiguity.
type relayProbe struct {
	sessions     relay.SessionSource
	signer       relay.Signer
	sender       relay.Sender
	streamSender relay.StreamSender
	validator    relay.Validator

	stage   RelayStage
	latency time.Duration
	sentAt  time.Time

	// The wire bytes the relay actually put on and took off the network. They
	// are captured here because relay.Relayer hands back only the unwrapped
	// backend response — the signatures and session header that make a relay
	// verifiable live in the protobuf envelopes around it, and these two seams
	// are the last place they are still whole. See Client.receiptFrom.
	signedRequestBz []byte
	responseBz      []byte
}

// elapsed is how long it is since the relay went out. For a streaming relay it
// is read once per batch, which is what makes the first batch's value the
// time-to-first-token.
func (p *relayProbe) elapsed() time.Duration {
	if p.sentAt.IsZero() {
		return p.latency
	}
	return time.Since(p.sentAt)
}

func (p *relayProbe) Session(ctx context.Context, serviceID domain.ServiceID) (*domain.Session, error) {
	session, err := p.sessions.Session(ctx, serviceID)
	if err != nil {
		p.stage = StageSession
	}
	return session, err
}

func (p *relayProbe) Start(ctx context.Context) error { return p.sessions.Start(ctx) }

func (p *relayProbe) SignRelay(ctx context.Context, session *domain.Session, endpoint domain.Endpoint, rpcType domain.RPCType, in domain.RelayInput) ([]byte, error) {
	relayReqBz, err := p.signer.SignRelay(ctx, session, endpoint, rpcType, in)
	if err != nil {
		p.stage = StageSigning
	}
	p.signedRequestBz = relayReqBz
	return relayReqBz, err
}

func (p *relayProbe) Send(ctx context.Context, url string, relayReqBz []byte, rpcType domain.RPCType) ([]byte, error) {
	p.sentAt = time.Now()
	respBz, err := p.sender.Send(ctx, url, relayReqBz, rpcType)
	p.latency = time.Since(p.sentAt)
	if err != nil {
		p.stage = StageSending
	}
	return respBz, err
}

func (p *relayProbe) ValidateResponse(supplier domain.EndpointAddr, respBz []byte) (*domain.RelayResult, error) {
	// Held for the receipt. On a streaming relay this is overwritten per batch,
	// which is correct: every batch carries its own supplier signature.
	p.responseBz = respBz
	result, err := p.validator.ValidateResponse(supplier, respBz)
	if err != nil {
		p.stage = StageValidation
	}
	return result, err
}

// SendStream is the streaming counterpart of Send. It returns as soon as the
// response headers land, so latency here is time-to-first-byte rather than the
// whole round trip — the body is still arriving.
func (p *relayProbe) SendStream(ctx context.Context, url string, relayReqBz []byte, rpcType domain.RPCType) (io.ReadCloser, map[string][]string, int, error) {
	p.sentAt = time.Now()
	body, header, statusCode, err := p.streamSender.SendStream(ctx, url, relayReqBz, rpcType)
	p.latency = time.Since(p.sentAt)
	if err != nil {
		p.stage = StageSending
	}
	return body, header, statusCode, err
}

// Compile-time assertions: the probe really does stand in for every seam.
var (
	_ relay.SessionSource = (*relayProbe)(nil)
	_ relay.Signer        = (*relayProbe)(nil)
	_ relay.Sender        = (*relayProbe)(nil)
	_ relay.StreamSender  = (*relayProbe)(nil)
	_ relay.Validator     = (*relayProbe)(nil)
)

// --- shared relay setup ------------------------------------------------------

// newProbe builds a probe over this client's real seams.
func (c *Client) newProbe() *relayProbe {
	return &relayProbe{
		sessions:     c.sessions,
		signer:       c.signer,
		sender:       c.sender,
		streamSender: c.sender,
		validator:    c.validator,
	}
}

// relayContext names the app the relay is billed to, pins it to one supplier,
// and applies the payload deadline.
//
// ⚠️ On the streaming path that deadline bounds the WHOLE stream, not the wait
// for the first byte — a long answer needs a timeout sized for the long answer,
// or it is cut off mid-stream. The returned cancel is always safe to call.
func (c *Client) relayContext(ctx context.Context, appAddress, supplierAddress string, payload Payload) (context.Context, context.CancelFunc) {
	relayCtx := withApp(ctx, appAddress)
	relayCtx = domain.WithSupplierPolicy(relayCtx, domain.SupplierPolicy{
		Allow: []domain.EndpointAddr{domain.EndpointAddr(supplierAddress)},
	})
	if payload.Timeout <= 0 {
		return relayCtx, func() {}
	}
	return context.WithTimeout(relayCtx, payload.Timeout)
}

// relayInput converts a testbench payload into the pocket-ap request shape.
func relayInput(payload Payload) domain.RelayInput {
	return domain.RelayInput{
		Method: payload.Method,
		Path:   payload.Path,
		Body:   []byte(payload.Data),
	}
}

// isNoApp reports whether an error means we hold no signing key for the app.
func isNoApp(err error) bool { return errors.Is(err, domain.ErrNoApp) }

// endpointForSupplier finds the named supplier in a session and the URL it
// advertises for the requested transport.
func endpointForSupplier(session *domain.Session, supplierAddress string, rpcType domain.RPCType) (domain.Endpoint, string, bool) {
	for _, endpoint := range session.Endpoints {
		if string(endpoint.Supplier) != supplierAddress {
			continue
		}
		url, ok := endpoint.URL(rpcType)
		return endpoint, url, ok
	}
	return domain.Endpoint{}, "", false
}

// isStreamingContentType reports whether a relay-miner reply holds
// delimiter-separated signed batches, by the media type the backend declared.
//
// ⚠️ This mirrors pocket-ap's own isStreamingResponse (relay/stream.go), which
// is unexported, and the media types are the relay miner's list. If pocket-ap
// starts batch-signing another type, this has to follow. Only the manual
// BuildSignedRequest path needs it — SendRelayStream uses pocket-ap's copy.
func isStreamingContentType(header map[string][]string) bool {
	var contentType string
	for name, values := range header {
		if strings.EqualFold(name, "Content-Type") && len(values) > 0 {
			contentType = values[0]
			break
		}
	}
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	switch strings.ToLower(mediaType) {
	case "text/event-stream", "application/x-ndjson":
		return true
	default:
		return false
	}
}
