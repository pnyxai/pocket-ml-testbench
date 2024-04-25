package types

import (
	"encoding/json"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
)

type PageAndServiceParams struct {
	// this one on v0 is still called blockchain, but will be service on v1
	Service string `json:"blockchain"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

type HeightAndOptsParams struct {
	Height int64                `json:"height"`
	Opts   PageAndServiceParams `json:"opts"`
}

// QueryHeightOutput is not properly exposed on poktGoSdk
type QueryHeightOutput struct {
	Height json.Number `json:"height"`
}

type NodesPageChannelResponse struct {
	Data  *poktGoSdk.GetNodesOutput
	Error error
}
