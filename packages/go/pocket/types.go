// Package pocket is the testbench's adapter to the Pocket Network protocol.
//
// It is the only package in this repository that imports pocket-ap or poktroll:
// everything protocol-shaped — sessions, ring signing, relay serialization,
// response validation — stays behind this boundary, and the apps above it deal
// only in the plain types declared here.
//
// What it adds on top of pocket-ap is the handful of things a testbench needs
// and a proxy deliberately does not:
//
//   - several apps staked for the SAME service, to widen the set of suppliers
//     seen in session (pocket-ap refuses this by design — see multiAppSessions);
//   - relays pinned to a NAMED supplier rather than a selected one;
//   - a failure classified by the stage it happened at, because "whose fault was
//     it" is the measurement (see RelayStage);
//   - flat, JSON-serializable session values that can cross a Temporal boundary.
package pocket

import (
	"time"
)

// ServiceID is a Pocket Network service identifier (e.g. "text-to-text").
type ServiceID string

// FullNodeConfig points at the Pocket full node to query. Two transports are
// needed: gRPC for session/app/account/params, CometBFT RPC for block height.
type FullNodeConfig struct {
	RpcURL     string     `json:"rpc_url"`
	GRPCConfig GRPCConfig `json:"grpc_config"`
}

type GRPCConfig struct {
	HostPort string `json:"host_port"`
	Insecure bool   `json:"insecure"`
}

// ServiceConfig holds the per-service settings the testbench needs to reach a
// Pocket Network service.
type ServiceConfig struct {
	// RPCType picks which transport we relay over for this service.
	//
	// A supplier staked on a service can advertise SEVERAL transports, each on
	// its own URL, and the relay miner routes on the `Rpc-Type` header that goes
	// out with the relay. So this single value picks both the endpoint URL we
	// talk to and the header we stamp, and a supplier that advertises nothing for
	// the chosen type is simply not reachable for this service.
	//
	// One of: "json_rpc", "rest", "comet_bft", "grpc". Empty means DefaultRPCType.
	RPCType string `json:"rpc_type"`
}

// DefaultRPCType is used for any service that has no entry in the services
// configuration. REST is what the ML services (OpenAI-style HTTP APIs) use.
const DefaultRPCType = "rest"

// DefaultSenderTimeout caps how long a single relay may take at the HTTP client
// level. Every relay also carries its own (shorter) context deadline built from
// the prompt timeout; this is only a backstop so a supplier that accepts a
// connection and never answers cannot pin a worker goroutine forever.
const DefaultSenderTimeout = 30 * time.Minute

// Payload is one request to relay to a service's backend.
type Payload struct {
	Data    string
	Method  string
	Path    string
	Timeout time.Duration
}

// Response is what the service's backend answered, unwrapped from the relay.
type Response struct {
	// Bytes is the body the backend returned.
	Bytes []byte
	// HTTPStatusCode is the status the backend returned.
	HTTPStatusCode int
	// Ms is how long the round trip to the supplier took. It is set even when
	// the relay fails, as long as we got far enough to send.
	Ms int64

	// Receipt is the signed, verifiable record of the relay — session header,
	// both signatures, and the payload hashes. Present only on success, and nil
	// if the envelopes could not be decoded. See RelayReceipt, in particular for
	// why it is not an onchain proof.
	Receipt *RelayReceipt `json:"receipt,omitempty"`
}

// Endpoint is one supplier reachable for a service, over the transport that
// service is configured to use.
//
// It is deliberately a flat pair of strings. The version this replaced carried
// a whole poktroll session so a relay could be built from it, which made it
// impossible to pass through Temporal (the session's protobuf enum does not
// round-trip through the default data converter). Sessions now live in the
// per-app caches pocket-ap manages and are looked up at relay time, so nothing
// protocol-shaped has to cross a workflow boundary.
type Endpoint struct {
	Supplier string `json:"supplier"`
	Url      string `json:"url"`
}

// SessionInfo is the flat, JSON-serializable view of a session that the
// testbench works with. See Client.GetSession for why it is not the protobuf.
type SessionInfo struct {
	SessionId           string     `json:"session_id"`
	AppAddress          string     `json:"app_address"`
	Service             string     `json:"service"`
	SessionNumber       int64      `json:"session_number"`
	NumBlocksPerSession int64      `json:"num_blocks_per_session"`
	SessionStartHeight  int64      `json:"session_start_height"`
	SessionEndHeight    int64      `json:"session_end_height"`
	Endpoints           []Endpoint `json:"endpoints"`
}

// EndpointsBySupplier indexes the session's endpoints by supplier address, which
// is how the requester looks them up when it assigns prompts to suppliers.
func (s *SessionInfo) EndpointsBySupplier() map[string]Endpoint {
	out := make(map[string]Endpoint, len(s.Endpoints))
	for _, endpoint := range s.Endpoints {
		out[endpoint.Supplier] = endpoint
	}
	return out
}
