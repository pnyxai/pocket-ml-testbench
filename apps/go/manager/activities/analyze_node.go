package activities

import (
	"context"
	"packages/mongodb"

	"github.com/rs/zerolog"
)

type AnalyzeNodeParams struct {
	Node  NodeData `json:"node"`
	Tasks []string `json:"tasks"`
}

type AnalyzeNodeResults struct {
	Success bool
}

var AnalyzeNodeName = "analyze_node"

func (aCtx *Ctx) AnalyzeNode(ctx context.Context, params AnalyzeNodeParams) (*AnalyzeNodeResults, error) {

	// Get logger
	l := aCtx.App.Logger
	l.Debug().Str("address", params.Node.Address).Str("service", params.Node.Service).Msg("Analyzing staked node.")

	// Get nodes collection
	nodesCollection := aCtx.App.Mongodb.GetCollection("nodes")

	// Retrieve this node entry
	var thisNodeData NodeRecord
	found, err := thisNodeData.LoadNode(params.Node, nodesCollection, l)
	if err != nil {
		return nil, err
	}

	if !found {
		// Create entry in MongoDB
		l.Debug().Bool("found", found).Msg("Creating empty node entry.")
		thisNodeData.Init(params, l)

	} else {
		// If the node entry exist we must check for pending results
		err = updateTasksNode(&thisNodeData, params.Tasks, aCtx.App.Mongodb, l)
		if err != nil {
			return nil, err
		}

	}

	// Push to DB
	thisNodeData.UpdateNode(nodesCollection, l)

	// Check tasks trigger list
	// Check if already triggered and the total quantity,
	// if the total qtity is less than limit, trigger new tasks, else skip

	// placeholder
	result := AnalyzeNodeResults{Success: true}
	return &result, nil
}

func updateTasksNode(nodeData *NodeRecord, tasks []string, mongo *mongodb.MongoDb, l *zerolog.Logger) error {

	nodeData.LastSeenHeight += 1

	// Get results collection
	// resultsCollection := mongo.GetCollection("results")

	// Check for each task sample date
	for _, task := range tasks {
		l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("task", task).Msg("Updating circular buffer.")
		// Drop old samples (move indices).
		// Read the results DB
		// Update circular buffers (replace oldest sample, move indices)
		// Re-calculate task metrics rolling averages.
	}

	return nil
}
