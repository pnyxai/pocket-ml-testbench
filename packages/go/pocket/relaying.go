package pocket

import (
	"context"
	"errors"
	"fmt"
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

	relayCtx := withApp(ctx, appAddress)
	relayCtx = domain.WithSupplierPolicy(relayCtx, domain.SupplierPolicy{
		Allow: []domain.EndpointAddr{domain.EndpointAddr(supplierAddress)},
	})
	if payload.Timeout > 0 {
		var cancelFn context.CancelFunc
		relayCtx, cancelFn = context.WithTimeout(relayCtx, payload.Timeout)
		defer cancelFn()
	}

	// A Relayer per relay, so the probe below is private to it. The struct only
	// holds shared pointers, so this costs an allocation, and it is the same
	// shape `pocket-ap call` uses to report on a single relay.
	probe := &relayProbe{
		sessions:  c.sessions,
		signer:    c.signer,
		sender:    c.sender,
		validator: c.validator,
	}
	relayer := relay.Relayer{
		Sessions:    probe,
		Signer:      probe,
		Validator:   probe,
		Sender:      probe,
		Selector:    c.selector,
		MaxAttempts: 1,
	}

	result, err := relayer.Relay(relayCtx, domain.ServiceID(serviceID), c.rpcTypeFor(serviceID), domain.RelayInput{
		Method: payload.Method,
		Path:   payload.Path,
		Body:   []byte(payload.Data),
	})
	response.Ms = probe.latency.Milliseconds()

	if err != nil {
		return response, &RelayError{Stage: stageOf(probe, err), Err: err}
	}

	response.Bytes = result.Body
	response.HTTPStatusCode = result.StatusCode

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
	sessions  relay.SessionSource
	signer    relay.Signer
	sender    relay.Sender
	validator relay.Validator

	stage   RelayStage
	latency time.Duration
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
	return relayReqBz, err
}

func (p *relayProbe) Send(ctx context.Context, url string, relayReqBz []byte, rpcType domain.RPCType) ([]byte, error) {
	start := time.Now()
	respBz, err := p.sender.Send(ctx, url, relayReqBz, rpcType)
	p.latency = time.Since(start)
	if err != nil {
		p.stage = StageSending
	}
	return respBz, err
}

func (p *relayProbe) ValidateResponse(supplier domain.EndpointAddr, respBz []byte) (*domain.RelayResult, error) {
	result, err := p.validator.ValidateResponse(supplier, respBz)
	if err != nil {
		p.stage = StageValidation
	}
	return result, err
}

// Compile-time assertions: the probe really does stand in for every seam.
var (
	_ relay.SessionSource = (*relayProbe)(nil)
	_ relay.Signer        = (*relayProbe)(nil)
	_ relay.Sender        = (*relayProbe)(nil)
	_ relay.Validator     = (*relayProbe)(nil)
)
