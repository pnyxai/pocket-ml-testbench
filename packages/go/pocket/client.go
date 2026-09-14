package pocket

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	sessiontypes "github.com/pokt-network/poktroll/x/session/types"

	"github.com/pokt-network/pocket-ap/domain"
	ap "github.com/pokt-network/pocket-ap/pocket"
	"github.com/pokt-network/pocket-ap/relay"
	"github.com/pokt-network/pocket-ap/selector"
	"github.com/rs/zerolog"
)

// Client is the testbench's single entry point to the Pocket Network protocol.
// See the package comment for what it adds over pocket-ap.
type Client struct {
	fullNode  *ap.FullNode
	signer    *ap.MultiSigner
	sender    *ap.HTTPSender
	validator *ap.Validator
	selector  relay.Selector
	sessions  *multiAppSessions

	// appServices maps an app address to the one service it is staked for,
	// read from the chain at startup (poktroll allows exactly one per app).
	appServices map[string]ServiceID

	rpcTypes map[ServiceID]domain.RPCType

	// blocksPerSession is the chain's num_blocks_per_session governance param.
	// Read once at startup and refreshed from every session fetched afterwards,
	// so a param change lands without a restart.
	blocksPerSession atomic.Int64

	logger *zerolog.Logger

	startOnce sync.Once
}

// NewClient dials the full node, derives every configured app from its key, and
// prepares one session manager per app.
//
// apps maps an app address to its hex private key. The address is checked
// against the one the key derives: a mismatch means the pairing in the config is
// wrong, which would otherwise show up much later as "session not found" for an
// app nobody owns.
//
// services carries the per-service settings (currently the RPC type). A service
// with no entry falls back to DefaultRPCType.
func NewClient(
	cfg FullNodeConfig,
	apps map[string]string,
	services map[string]ServiceConfig,
	senderTimeout time.Duration,
	l *zerolog.Logger,
) (*Client, error) {
	fullNode, err := ap.NewFullNode(cfg.GRPCConfig.HostPort, cfg.GRPCConfig.Insecure, cfg.RpcURL)
	if err != nil {
		return nil, fmt.Errorf("NewClient: %w", err)
	}

	rpcTypes, err := parseServiceRPCTypes(services)
	if err != nil {
		return nil, err
	}

	if senderTimeout <= 0 {
		senderTimeout = DefaultSenderTimeout
	}

	c := &Client{
		fullNode:  fullNode,
		sender:    ap.NewHTTPSender(senderTimeout),
		validator: ap.NewValidator(fullNode),
		// Filter is the layer that reads the per-relay supplier allow list off
		// the context; Random, which it delegates the ordering to, ignores it.
		// Policies is empty because every restriction the testbench applies is
		// per relay, never per process.
		selector:    selector.Filter{Inner: selector.Random{}},
		sessions:    &multiAppSessions{byApp: make(map[string]*ap.SessionManager, len(apps))},
		appServices: make(map[string]ServiceID, len(apps)),
		rpcTypes:    rpcTypes,
		logger:      l,
	}

	ctx := context.Background()
	signers := make([]*ap.Signer, 0, len(apps))
	for appAddress, privateKeyHex := range apps {
		signer, err := ap.NewSigner(privateKeyHex, fullNode)
		if err != nil {
			return nil, fmt.Errorf("NewClient: app %s: %w", appAddress, err)
		}
		if signer.AppAddr() != appAddress {
			return nil, fmt.Errorf(
				"NewClient: the private key configured for app %s derives app %s instead — the address/key pair in the config do not belong together",
				appAddress, signer.AppAddr(),
			)
		}

		// The app's service comes from the chain rather than from config: an
		// application is staked for exactly one, so asking the operator to also
		// type it in would only create a second source of truth to disagree with.
		serviceID, err := signer.ServiceID(ctx)
		if err != nil {
			return nil, fmt.Errorf("NewClient: app %s: %w", appAddress, err)
		}

		sessionManager, err := ap.NewSessionManager(fullNode, []ap.ServiceApp{
			{ServiceID: serviceID, AppAddr: appAddress},
		})
		if err != nil {
			return nil, fmt.Errorf("NewClient: app %s: %w", appAddress, err)
		}

		signers = append(signers, signer)
		c.sessions.byApp[appAddress] = sessionManager
		c.appServices[appAddress] = ServiceID(serviceID)

		l.Info().
			Str("appAddress", appAddress).
			Str("service", string(serviceID)).
			Str("rpc_type", c.rpcTypeFor(ServiceID(serviceID)).String()).
			Msg("Pocket Network app ready.")
	}

	c.signer = ap.NewMultiSigner(signers...)

	sharedParams, err := fullNode.GetSharedParams(ctx)
	if err != nil {
		return nil, fmt.Errorf("NewClient: reading shared params: %w", err)
	}
	blocksPerSession := int64(sharedParams.GetNumBlocksPerSession())
	if blocksPerSession <= 0 {
		return nil, fmt.Errorf("NewClient: the chain reports num_blocks_per_session=%d", blocksPerSession)
	}
	c.blocksPerSession.Store(blocksPerSession)
	l.Info().Int64("blocksPerSession", blocksPerSession).Msg("Read session length from chain.")

	return c, nil
}

// parseServiceRPCTypes validates the configured transport of every service up
// front, so a typo is a startup failure rather than an unreachable service.
func parseServiceRPCTypes(services map[string]ServiceConfig) (map[ServiceID]domain.RPCType, error) {
	out := make(map[ServiceID]domain.RPCType, len(services))
	for serviceID, serviceCfg := range services {
		rpcTypeName := serviceCfg.RPCType
		if rpcTypeName == "" {
			rpcTypeName = DefaultRPCType
		}
		rpcType, err := domain.ParseRPCType(rpcTypeName)
		if err != nil {
			return nil, fmt.Errorf("service %s: %w", serviceID, err)
		}
		if !rpcType.Stateless() {
			// The testbench only ever sends request/response relays. A streaming
			// type has no one-shot form, so accepting it here would just fail on
			// the first relay.
			return nil, fmt.Errorf("service %s: rpc type %s is a streaming transport, which the testbench does not relay over", serviceID, rpcType)
		}
		out[ServiceID(serviceID)] = rpcType
	}
	return out, nil
}

// Start launches the background block pollers that drive session rotation.
//
// It is NOT optional for a long running worker: a session manager only refreshes
// a cached session once the polled chain head passes its end height, so without
// a poller the height stays 0, no session ever looks expired, and every relay
// after the first rollover goes to a session the chain has already retired.
//
// The pollers run until ctx is cancelled, which is the only shutdown they need —
// pass a cancellable context to stop them, or Background to tie them to the
// process.
func (c *Client) Start(ctx context.Context) error {
	var err error
	c.startOnce.Do(func() { err = c.sessions.Start(ctx) })
	return err
}

// rpcTypeFor returns the transport configured for a service, or the default.
func (c *Client) rpcTypeFor(serviceID ServiceID) domain.RPCType {
	if rpcType, ok := c.rpcTypes[serviceID]; ok {
		return rpcType
	}
	// DefaultRPCType is a validated constant, so this cannot fail.
	rpcType, _ := domain.ParseRPCType(DefaultRPCType)
	return rpcType
}

// Apps lists every configured app address, sorted for stable iteration.
func (c *Client) Apps() []string {
	out := make([]string, 0, len(c.appServices))
	for appAddress := range c.appServices {
		out = append(out, appAddress)
	}
	sort.Strings(out)
	return out
}

// ServiceForApp returns the service an app is staked for, as read from chain at
// startup.
func (c *Client) ServiceForApp(appAddress string) (ServiceID, bool) {
	serviceID, ok := c.appServices[appAddress]
	return serviceID, ok
}

// AppIsStakedForService reports whether a configured app serves a service.
func (c *Client) AppIsStakedForService(appAddress string, serviceID ServiceID) bool {
	staked, ok := c.appServices[appAddress]
	return ok && staked == serviceID
}

// GetLatestBlockHeight queries the full node for the current chain head.
func (c *Client) GetLatestBlockHeight() (int64, error) {
	return c.fullNode.GetCurrentBlockHeight(context.Background())
}

// BlocksPerSession returns the chain's session length in blocks.
//
// This used to be a config entry with a "no SDK support" TODO against it. It is
// read from the shared module's governance params at startup and refreshed from
// every session fetched since, so it is never a number someone has to keep in
// step with the network by hand.
func (c *Client) BlocksPerSession() int64 {
	return c.blocksPerSession.Load()
}

// GetSession returns the current session for an (app, service) pair as a plain,
// JSON-serializable value.
//
// The conversion is the point: a poktroll session is a protobuf carrying an enum
// (RPCType) that Temporal's default data converter cannot round-trip — it fails
// with `unknown value "JSON_RPC" for enum pocket.shared.RPCType`. Returning a
// flat struct is what lets sessions cross an activity or workflow boundary at
// all.
//
// Endpoints are filtered to the transport configured for the service, so a
// supplier that advertises other types but not this one is correctly absent.
func (c *Client) GetSession(ctx context.Context, appAddress string, serviceID ServiceID) (*SessionInfo, error) {
	session, err := c.sessions.Session(withApp(ctx, appAddress), domain.ServiceID(serviceID))
	if err != nil {
		return nil, fmt.Errorf("GetSession: service %s app %s: %w", serviceID, appAddress, err)
	}

	rpcType := c.rpcTypeFor(serviceID)
	info := &SessionInfo{
		SessionId:        session.ID,
		AppAddress:       session.AppAddr,
		Service:          string(session.ServiceID),
		SessionEndHeight: session.EndBlockHeight,
		Endpoints:        make([]Endpoint, 0, len(session.Endpoints)),
	}

	// pocket-ap keeps the raw poktroll session on the domain value precisely so
	// callers can read the fields it does not model. The testbench needs the
	// session number and length: they are what its `trigger_session` bookkeeping
	// in MongoDB is keyed on, so the arithmetic has to stay identical.
	if raw, ok := session.Raw.(*sessiontypes.Session); ok && raw != nil {
		info.SessionNumber = raw.SessionNumber
		info.NumBlocksPerSession = raw.NumBlocksPerSession
		if raw.Header != nil {
			info.SessionStartHeight = raw.Header.SessionStartBlockHeight
		}
		if raw.NumBlocksPerSession > 0 {
			c.blocksPerSession.Store(raw.NumBlocksPerSession)
		}
	}

	for _, endpoint := range session.Endpoints {
		url, ok := endpoint.URL(rpcType)
		if !ok {
			c.logger.Debug().
				Str("supplier", string(endpoint.Supplier)).
				Str("service", string(serviceID)).
				Str("rpc_type", rpcType.String()).
				Msg("Supplier advertises no endpoint for the configured transport, skipping.")
			continue
		}
		info.Endpoints = append(info.Endpoints, Endpoint{
			Supplier: string(endpoint.Supplier),
			Url:      url,
		})
	}

	return info, nil
}

// SuppliersInSession returns, per service, the unique supplier addresses that
// are in session for the given apps and reachable over the service's configured
// transport.
//
// An app is staked for exactly one service, so a pair whose service is not the
// app's own has no session to fetch and is skipped rather than attempted: doing
// otherwise turns a perfectly normal multi-service configuration into a fetch
// that always fails.
//
// TODO : It would be nice to change all this into an SDK version of
// `pocketd query supplier list-suppliers --chain-id <chainID>`
func (c *Client) SuppliersInSession(ctx context.Context, apps []string, serviceIDs []string, l *zerolog.Logger) (map[string][]string, error) {
	supplierSeen := make(map[string]map[string]bool)
	uniqueSuppliers := make(map[string][]string)

	for _, appAddress := range apps {
		for _, serviceID := range serviceIDs {
			if !c.AppIsStakedForService(appAddress, ServiceID(serviceID)) {
				l.Debug().
					Str("thisApp", appAddress).
					Str("thisService", serviceID).
					Msg("App is not staked for service, skipping.")
				continue
			}

			session, err := c.GetSession(ctx, appAddress, ServiceID(serviceID))
			if err != nil {
				l.Debug().Err(err).Str("thisApp", appAddress).Str("thisService", serviceID).Msg("Failed to get session.")
				return nil, err
			}

			for _, endpoint := range session.Endpoints {
				thisSupplier := endpoint.Supplier
				if _, ok := supplierSeen[serviceID]; !ok {
					supplierSeen[serviceID] = make(map[string]bool)
				}
				if !supplierSeen[serviceID][thisSupplier] {
					supplierSeen[serviceID][thisSupplier] = true
					uniqueSuppliers[serviceID] = append(uniqueSuppliers[serviceID], thisSupplier)
				} else {
					l.Debug().Str("thisService", serviceID).Str("thisSupplier", thisSupplier).Msg("Duplicate supplier found.")
				}
			}
		}
	}

	return uniqueSuppliers, nil
}

// --- session source ---------------------------------------------------------

// appContextKey types the context key so nothing else can collide with it.
type appContextKey struct{}

// withApp names the app a relay is billed to.
//
// The context is the carrier because relay.SessionSource is keyed by service
// alone — a proxy has one app per service and nothing to choose. The testbench
// deliberately runs several apps against the same service to widen the supplier
// set it sees, so the app has to travel with the request, and pocket-ap already
// uses the context for exactly this kind of per-relay caller decision (see
// domain.WithSupplierPolicy).
func withApp(ctx context.Context, appAddress string) context.Context {
	return context.WithValue(ctx, appContextKey{}, appAddress)
}

func appFromContext(ctx context.Context) string {
	appAddress, _ := ctx.Value(appContextKey{}).(string)
	return appAddress
}

// multiAppSessions is a relay.SessionSource over one pocket-ap session manager
// per app. Each manager keeps its own session cache and rotation poller.
type multiAppSessions struct {
	byApp map[string]*ap.SessionManager
}

var _ relay.SessionSource = (*multiAppSessions)(nil)

func (m *multiAppSessions) Session(ctx context.Context, serviceID domain.ServiceID) (*domain.Session, error) {
	appAddress := appFromContext(ctx)
	sessionManager, ok := m.byApp[appAddress]
	if !ok {
		// Wrapping the pocket-ap sentinel keeps errors.Is working for callers
		// that only want to know "we hold no key for this app".
		return nil, fmt.Errorf("no signing key configured for app %q: %w", appAddress, domain.ErrNoApp)
	}
	return sessionManager.Session(ctx, serviceID)
}

func (m *multiAppSessions) Start(ctx context.Context) error {
	for appAddress, sessionManager := range m.byApp {
		if err := sessionManager.Start(ctx); err != nil {
			return fmt.Errorf("app %s: %w", appAddress, err)
		}
	}
	return nil
}
