package types

import (
	"time"
)

type FullNodeConfig struct {
	RpcURL     string     `json:"rpc_url"`
	GRPCConfig GRPCConfig `json:"grpc_config"`
}

type GRPCConfig struct {
	HostPort string `json:"host_port"`
	Insecure bool   `json:"insecure"`
}

type Payload struct {
	Data    string
	Method  string
	Path    string
	Timeout time.Duration
}

type EndpointAddr string

type ServiceID string

type Response struct {
	// Bytes is the response to a relay received from an endpoint.
	// An endpoint is the backend server servicing an onchain service.
	// This can be the serialized response to any type of RPC (gRPC, HTTP, etc.)
	Bytes []byte
	// HTTPStatusCode is the HTTP status returned by an endpoint in response to a relay request.
	HTTPStatusCode int

	// EndpointAddr is the address of the endpoint which returned the response.
	EndpointAddr
}
