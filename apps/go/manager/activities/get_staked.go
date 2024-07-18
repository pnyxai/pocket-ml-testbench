package activities

import (
	"context"
	"manager/types"
	"strconv"
)

var GetStakedName = "get_staked"

func (aCtx *Ctx) GetStaked(ctx context.Context, params types.GetStakedParams) (*types.GetStakedResults, error) {

	l := aCtx.App.Logger
	l.Debug().Msg("Collecting staked nodes from network.")

	result := types.GetStakedResults{}

	// Get all nodes in given chain
	l.Debug().Str("service", params.Service).Msg("Querying service...")
	nodes, err := aCtx.App.PocketRpc.GetNodes(params.Service)
	if err != nil {
		l.Error().Str("service", params.Service).Msg("Could not retrieve staked nodes.")
		return nil, err
	}
	if len(nodes) == 0 {
		l.Warn().Str("service", params.Service).Msg("No nodes found staked.")
	}
	for _, node := range nodes {
		if !node.Jailed {
			this_node := types.NodeData{
				Address: node.Address,
				Service: params.Service,
			}
			result.Nodes = append(result.Nodes, this_node)
		}
	}

	if len(result.Nodes) == 0 {
		l.Warn().Msg("No nodes were found on any of the given services")
	} else {
		l.Info().Int("nodes_staked", len(result.Nodes)).Msg("Successfully pulled staked node-services.")
	}

	// Get block data
	currHeight, err := aCtx.App.PocketRpc.GetHeight()
	if err != nil {
		l.Error().Str("service", params.Service).Msg("Could not retrieve latest block hieght.")
		return nil, err
	}
	blockParams, err := aCtx.App.PocketRpc.GetAllParams(currHeight)
	if err != nil {
		l.Error().Str("service", params.Service).Msg("Could not retrieve block params.")
		return nil, err
	}
	blocksPerSession, ok := blockParams.NodeParams.Get("pos/BlocksPerSession")
	if !ok {
		l.Error().Str("service", params.Service).Msg("Cannot get blocks per session parameter.")
		return nil, err
	}
	i64, err := strconv.ParseInt(blocksPerSession, 10, 64)
	if err != nil {
		l.Error().Str("service", params.Service).Msg("Could convert parameter to number.")
		return nil, err
	}

	// Assign
	result.Block.BlocksPerSession = i64
	result.Block.Height = currHeight

	return &result, nil
}
