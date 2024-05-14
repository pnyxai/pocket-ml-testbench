package activities

import (
	"context"
	"fmt"
	"packages/mongodb"
	"time"

	"manager/types"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
)

var AnalyzeNodeName = "analyze_node"

func (aCtx *Ctx) AnalyzeNode(ctx context.Context, params types.AnalyzeNodeParams) (*types.AnalyzeNodeResults, error) {

	var result types.AnalyzeNodeResults
	result.Success = false

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

	// Get tasks collection
	tasksCollection := aCtx.App.Mongodb.GetCollection(types.TaskCollection)
	// Get tasks instances
	instancesCollection := aCtx.App.Mongodb.GetCollection(types.InstanceCollection)

	// Loop over all tasks and frameworks
	for _, test := range params.Tests {

		for _, task := range test.Tasks {
			l.Debug().Str("address", thisNodeData.Address).Str("service", thisNodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Checking task requests.")

			// Get task record
			thisTaskRecord, found := getTaskData(&thisNodeData, test.Framework, task, l)
			if found != true {
				l.Error().Str("address", thisNodeData.Address).Str("service", thisNodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("not found task entry after check creation (task should be present at this point)")
				return nil, fmt.Errorf("not found task entry after check creation (task should be present at this point)")
			}

			// If the number of samples is less than the minimum, proceed to request more
			if thisTaskRecord.NumSamples <= MinSamplesPerTask {

				// Calculate the total number of request needed
				reqNeeded := MinSamplesPerTask - thisTaskRecord.NumSamples
				// Check if this exceed the max concurrent task and limit
				if reqNeeded > MaxConcurrentSamplesPerTask {
					reqNeeded = MaxConcurrentSamplesPerTask
				}

				// Set filtering for this node-service pair data
				task_request_filter := bson.D{{Key: "requester_args.address", Value: thisNodeData.Address},
					{Key: "requester_args.service", Value: thisNodeData.Service},
					{Key: "framework", Value: test.Framework},
					{Key: "task", Value: task}}

				// Set mongo context
				ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()

				// Now retrieve all node task requests entries
				cursor, err := tasksCollection.Find(ctxM, task_request_filter)
				if err != nil {
					l.Error().Msg("Could not retrieve task request data from MongoDB.")
					return nil, err
				}
				defer cursor.Close(ctxM)
				var inQueue uint32 = 0
				var blackList []int
				for cursor.Next(ctxM) {
					var taskReq types.TaskRequestRecord
					if err := cursor.Decode(&taskReq); err != nil {
						l.Error().Msg("Could not decode task request data from MongoDB.")
						return nil, err
					}
					// If not already done
					if !taskReq.Done {
						// Count pending
						inQueue += uint32(taskReq.Qty)

						// Search for associated instances to retrieve ids
						instances_filter := bson.D{{"task_id", taskReq.Id}}
						cursor2, err := instancesCollection.Find(ctxM, instances_filter)
						if err != nil {
							l.Error().Msg("Could not retrieve instances data from MongoDB.")
							return nil, err
						}
						defer cursor2.Close(ctxM)
						// Get all ids
						for cursor2.Next(ctxM) {
							var thisInstance types.InstanceRecord
							if err := cursor.Decode(&thisInstance); err != nil {
								l.Error().Msg("Could not decode task request data from MongoDB.")
								return nil, err
							}
							for _, docId := range thisInstance.DocIds {
								blackList = append(blackList, docId)
							}
						}
					}

				}

				// Remove the number of task in queue
				reqNeeded -= inQueue
				if reqNeeded > 0 {

					// Add trigger
					thisTrigger := types.TaskTrigger{Address: thisNodeData.Address,
						Service:   thisNodeData.Service,
						Framework: test.Framework,
						Task:      task,
						Blacklist: blackList,
						Qty:       int(reqNeeded)}
					result.Triggers = append(result.Triggers, thisTrigger)
				} else {
					l.Info().Str("address", thisNodeData.Address).Str("service", thisNodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Pending requests capped.")
				}

			}
		}
	}

	result.Success = true

	return &result, nil
}

// Get specific task data from a node record
func getTaskData(nodeData *NodeRecord, framework string, task string, l *zerolog.Logger) (*TaskRecord, bool) {

	for i, taskEntry := range nodeData.Tasks {
		// Check if the Name field matches the search string
		if taskEntry.Framework == framework && taskEntry.Task == task {
			l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", framework).Str("task", task).Msg("Found!")
			return &nodeData.Tasks[i], true
		}
	}
	return nil, false
}

func updateTasksNode(nodeData *NodeRecord, tests []types.TestsData, mongo mongodb.MongoDb, l *zerolog.Logger) (err error) {

	nodeData.LastSeenHeight += 1

	// Get results collection
	resultsCollection := mongo.GetCollection(types.ResultsCollection)

	//--------------------------------------------------------------------------
	// Drop old tasks that have not been updated in a long time
	//--------------------------------------------------------------------------

	err = nodeData.PruneTasks(l)
	if err != nil {
		return err
	}

	//--------------------------------------------------------------------------
	// Check for each task sample date
	//--------------------------------------------------------------------------
	for _, test := range tests {

		for _, task := range test.Tasks {
			l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Updating circular buffer.")

			//------------------------------------------------------------------
			// Get stored data for this task
			//------------------------------------------------------------------
			thisTaskRecord, found := getTaskData(nodeData, test.Framework, task, l)

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
						thisTaskRecord.IsertSample(thisTaskResults.Scores[i], time.Now(), thisTaskResults.SampleIds[i])
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

	return err
}
