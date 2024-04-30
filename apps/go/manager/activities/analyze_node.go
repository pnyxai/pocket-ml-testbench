package activities

import (
	"context"
	"packages/mongodb"
	"time"

	"manager/types"

	"github.com/rs/zerolog"
)

var AnalyzeNodeName = "analyze_node"

func (aCtx *Ctx) AnalyzeNode(ctx context.Context, params types.AnalyzeNodeParams) (*types.AnalyzeNodeResults, error) {

	// Get logger
	l := aCtx.App.Logger
	l.Debug().Str("address", params.Node.Address).Str("service", params.Node.Service).Msg("Analyzing staked node.")

	// Get nodes collection
	nodesCollection := aCtx.App.Mongodb.GetCollection(types.NodesCollection)

	// Retrieve this node entry
	var thisNodeData NodeRecord
	found, err := thisNodeData.FindAndLoadNode(params.Node, nodesCollection, l)
	if err != nil {
		return nil, err
	}

	//--------------------------------------------------------------------------
	// Update all tasks buffers
	//--------------------------------------------------------------------------

	if !found {
		// Create entry in MongoDB
		l.Debug().Bool("found", found).Msg("Creating empty node entry.")
		thisNodeData.Init(params, l)

	} else {
		// If the node entry exist we must cycle and check for pending results
		err = updateTasksNode(&thisNodeData, params.Tests, aCtx.App.Mongodb, l)
		if err != nil {
			return nil, err
		}

	}
	// Push to DB
	thisNodeData.UpdateNode(nodesCollection, l)

	//--------------------------------------------------------------------------
	// Trigger incomplete tasks
	//--------------------------------------------------------------------------

	// Check tasks trigger list
	// for each task check if the NumSamples is equal to the buffer lenght
	// Check if already triggered and the total quantity,
	// Check the Tasks database for this node/task
	// if the total quality is less than limit, trigger new tasks, else skip

	// If this is a LM, then we must also check for tokenizer state ???

	// placeholder
	result := types.AnalyzeNodeResults{Success: true}
	return &result, nil
}

func updateTasksNode(nodeData *NodeRecord, tests []types.TestsData, mongo mongodb.MongoDb, l *zerolog.Logger) (err error) {

	nodeData.LastSeenHeight += 1

	// Get results collection
	resultsCollection := mongo.GetCollection(types.ResultsCollection)

	// Check for each task sample date
	for _, test := range tests {

		for _, task := range test.Tasks {
			l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Updating circular buffer.")

			//------------------------------------------------------------------
			// Get stored data for this task
			//------------------------------------------------------------------

			var thisTaskRecord *TaskRecord
			found := false
			for i, taskEntry := range nodeData.Tasks {
				// Check if the Name field matches the search string
				if taskEntry.Framework == test.Framework && taskEntry.Task == task {
					l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Found!")
					found = true
					thisTaskRecord = &nodeData.Tasks[i]
				}
			}
			if !found {
				l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Not found, creating.")
				defaultDate := time.Now()
				thisTaskRecord = nodeData.AppendTask(test.Framework, task, defaultDate)
			}

			//------------------------------------------------------------------
			// Drop old samples (move indices).
			//------------------------------------------------------------------

			err = thisTaskRecord.CycleIndexes(l)
			if err != nil {
				return err
			}

			//------------------------------------------------------------------
			// Read new results from MongoDB Results and add to buffer
			//------------------------------------------------------------------

			var thisTaskResults ResultRecord
			found = false
			found, err = thisTaskResults.FindAndLoadResults(nodeData.Address,
				nodeData.Service,
				thisTaskRecord.Framework,
				thisTaskRecord.Task,
				resultsCollection,
				l)
			if err != nil {
				return err
			}
			if found == true {
				// If nothing is wrong with the result calculation
				if thisTaskResults.Status == 0 {
					// Add results to current task record
					for i := 0; i < int(thisTaskResults.NumSamples); i++ {
						thisTaskRecord.IsertSample(thisTaskResults.Scores[i], time.Now())
					}
				}
				// TODO: handle status!=0
			}

			//------------------------------------------------------------------
			// Calculate new averages
			//------------------------------------------------------------------
			thisTaskRecord.CalculateStats(l)

		}

	}

	//--------------------------------------------------------------------------
	// Drop old tasks that have not been updated in a long time
	//--------------------------------------------------------------------------

	err = nodeData.PruneTasks(l)
	if err != nil {
		return err
	}

	return err
}
