package activities

import (
	"context"
	"fmt"
)

type GetStakedParams struct {
	Services []string
}

type NodeData struct {
	Address string
	Service string
}

type GetStakedResults struct {
	Nodes []NodeData
}

var GetStakedName = "get_staked"

func (aCtx *Ctx) GetStaked(ctx context.Context, params GetStakedParams) (*GetStakedResults, error) {

	l := aCtx.App.Logger
	l.Debug().Msg("Collecting staked nodes from network.")

	// Get all nodes in given chain

	// Fill json

	// Return data

	// cheap mock
	result := GetStakedResults{}

	for i := 0; i < 5; i++ {
		thisNode := NodeData{Address: fmt.Sprint(i), Service: fmt.Sprint(i * 10)}
		result.Nodes = append(result.Nodes, thisNode)
	}

	return &result, nil
}
