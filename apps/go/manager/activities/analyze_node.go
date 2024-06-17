package activities

import (
	"context"
	"fmt"
	"packages/mongodb"
	"time"

	"manager/records"
	"manager/types"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var AnalyzeNodeName = "analyze_node"

func (aCtx *Ctx) AnalyzeNode(ctx context.Context, params types.AnalyzeNodeParams) (*types.AnalyzeNodeResults, error) {

	var result types.AnalyzeNodeResults
	result.Success = false
	result.IsNew = false

	// Get logger
	l := aCtx.App.Logger
	l.Debug().Str("address", params.Node.Address).Str("service", params.Node.Service).Msg("Analyzing staked node.")

	// Retrieve this node entry
	var thisNodeData records.NodeRecord
	found, err := thisNodeData.FindAndLoadNode(params.Node, aCtx.App.Mongodb, l)
	if err != nil {
		return nil, err
	}

	//--------------------------------------------------------------------------
	// Update all tasks buffers
	//--------------------------------------------------------------------------

	if !found {
		// Create entry in MongoDB
		l.Debug().Bool("found", found).Msg("Creating empty node entry.")
		thisNodeData.Init(params, aCtx.App.Config.Frameworks, aCtx.App.Mongodb, l)
		result.IsNew = true

	} else {
		// If the node entry exist we must cycle and check for pending results
		err = updateTasksNode(&thisNodeData, params.Tests, aCtx.App.Config.Frameworks, aCtx.App.Mongodb, l)
		if err != nil {
			return nil, err
		}

	}

	// TODO : Do general update of node entry if needed (for instance to track last values of buffers)

	// Push to DB the node data
	thisNodeData.UpdateNode(aCtx.App.Mongodb, l)

	//--------------------------------------------------------------------------
	// Trigger incomplete tasks
	//--------------------------------------------------------------------------

	// Loop over all tasks and frameworks
	for _, test := range params.Tests {

		for _, task := range test.Tasks {
			l.Debug().Str("address", thisNodeData.Address).Str("service", thisNodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Checking task requests.")

			// Check task dependencies
			depStatus, err := records.CheckTaskDependency(&thisNodeData, test.Framework, task, aCtx.App.Config.Frameworks, aCtx.App.Mongodb, l)
			if err != nil {
				l.Error().Msg("Could not check task dependencies.")
				return nil, err
			}
			if !depStatus {
				l.Info().Str("address", thisNodeData.Address).Str("service", thisNodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Does not meet task dependencies, ignoring for now.")
				continue
			}

			// Get task record
			taskType, err := records.GetTaskType(test.Framework, task, aCtx.App.Config.Frameworks, l)
			if err != nil {
				return nil, fmt.Errorf("cannot retrieve task type")
			}
			thisTaskRecord, found := records.GetTaskData(thisNodeData.ID, taskType, test.Framework, task, aCtx.App.Mongodb, l)
			if found != true {
				l.Error().Str("address", thisNodeData.Address).Str("service", thisNodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("not found task entry after check creation (task should be present at this point)")
				return nil, fmt.Errorf("not found task entry after check creation (task should be present at this point)")
			}

			// Check schedule restrictions
			schdStatus, err := records.CheckTaskSchedule(thisTaskRecord, params.Block, aCtx.App.Config.Frameworks, l)
			if err != nil {
				l.Error().Msg("Could not check task sqchedule.")
				return nil, err
			}
			if !schdStatus {
				l.Info().Str("address", thisNodeData.Address).Str("service", thisNodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Does not meet task schedule, ignoring for now.")
				continue
			}

			// The schedule is OK, now check minimum tasks to trigger
			minTrigger, err := records.CheckTaskTriggerMin(thisTaskRecord, params.Block, aCtx.App.Config.Frameworks, l)
			if err != nil {
				l.Error().Msg("Could not check task minimum trigger value.")
				return nil, err
			}

			// If the number of samples is less than the minimum or ther is a minimum value to trigger, proceed to request more
			numberOfSamples := thisTaskRecord.GetNumSamples()
			if numberOfSamples < thisTaskRecord.GetMinSamplesPerTask() || minTrigger > 0 {

				// Calculate the total number of request needed
				reqNeeded := thisTaskRecord.GetMinSamplesPerTask() - numberOfSamples
				// Check if this exceed the max concurrent task and limit
				maxConcurrentTasks := thisTaskRecord.GetMaxConcurrentSamplesPerTask()
				if reqNeeded > maxConcurrentTasks {
					reqNeeded = maxConcurrentTasks
				}

				// Get number of tasks in queue
				inQueue, _, blackList, _, err := CheckTaskDatabase(thisNodeData.Address, thisNodeData.Service, test.Framework, task, aCtx.App.Mongodb, l)
				if err != nil {
					return nil, err
				}

				// Only trigger if the tasks in queue are less than the maximum concurrent tasks that we allow
				if maxConcurrentTasks > inQueue {
					// Remove the number of task in queue
					reqNeeded -= inQueue

					// Apply minimum
					if reqNeeded < minTrigger {
						reqNeeded = minTrigger
					}

					if reqNeeded > 0 {

						// Add trigger
						thisTrigger := types.TaskTrigger{Address: thisNodeData.Address,
							Service:   thisNodeData.Service,
							Framework: test.Framework,
							Task:      task,
							Blacklist: blackList,
							Qty:       int(reqNeeded)}
						result.Triggers = append(result.Triggers, thisTrigger)
					}
				} else {
					l.Info().Str("address", thisNodeData.Address).Str("service", thisNodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Pending requests capped.")
				}

			} else {
				l.Info().Str("address", thisNodeData.Address).Str("service", thisNodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Buffer filled and up to date.")
			}
		}
	}

	result.Success = true

	return &result, nil
}

// Checks status of all node's tasks, drops old ones, checks for new results and re-computes.
func updateTasksNode(nodeData *records.NodeRecord, tests []types.TestsData, frameworkConfigMap map[string]types.FrameworkConfig, mongoDB mongodb.MongoDb, l *zerolog.Logger) (err error) {

	// Get results collection
	resultsCollection := mongoDB.GetCollection(types.ResultsCollection)

	//--------------------------------------------------------------------------
	// Check for each task sample date
	//--------------------------------------------------------------------------
	for _, test := range tests {

		for _, task := range test.Tasks {

			l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Updating circular buffer.")

			//------------------------------------------------------------------
			// Get stored data for this task
			//------------------------------------------------------------------
			taskType, err := records.GetTaskType(test.Framework, task, frameworkConfigMap, l)
			if err != nil {
				return nil
			}
			thisTaskRecord, found := records.GetTaskData(nodeData.ID, taskType, test.Framework, task, mongoDB, l)

			if !found {
				l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Not found, creating.")
				defaultDate := time.Now()
				thisTaskRecord = nodeData.AppendTask(nodeData.ID, test.Framework, task, defaultDate, frameworkConfigMap, mongoDB, l)
			}

			//------------------------------------------------------------------
			// Drop old samples (move indices).
			//------------------------------------------------------------------

			err = thisTaskRecord.CycleIndexes(l)
			if err != nil {
				return err
			}

			//------------------------------------------------------------------
			// Check pending and done tasks in the database
			//------------------------------------------------------------------
			_, tasksDone, _, tasksIDs, err := CheckTaskDatabase(nodeData.Address, nodeData.Service, test.Framework, task, mongoDB, l)
			if err != nil {
				l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", test.Framework).Str("task", task).Msg("Cannot check Tasks Database.")
				return err
			}

			//------------------------------------------------------------------
			// Read new results from MongoDB Results and add to buffer
			//------------------------------------------------------------------
			if tasksDone > 0 {

				// loop over all tasksIDs
				for _, taskID := range tasksIDs {
					thisTaskResults := thisTaskRecord.GetResultStruct()
					found = false
					found, err = thisTaskResults.FindAndLoadResults(taskID,
						resultsCollection,
						l)
					if err != nil {
						return err
					}
					if found == true {

						l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", test.Framework).Str("task", task).Str("task_id", taskID.String()).Msg("Processing found results.")

						// If nothing is wrong with the result calculation
						if thisTaskResults.GetStatus() == 0 {
							l.Debug().Int("NumSamples", int(thisTaskResults.GetNumSamples())).Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", test.Framework).Str("task", task).Str("task_id", taskID.String()).Msg("Inserting results into buffers.")
							// Add results to current task record
							for i := 0; i < int(thisTaskResults.GetNumSamples()); i++ {
								thisTaskRecord.InsertSample(time.Now(), thisTaskResults.GetSample(i), l)
							}
							// Update the last seen fields
							thisTaskRecord.UpdateLastHeight(thisTaskResults.GetResultHeight())
							thisTaskRecord.UpdateLastSeen(thisTaskResults.GetResultTime())
						} else {
							// TODO: handle status!=0
							l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", test.Framework).Str("task", task).Str("task_id", taskID.String()).Msg("Status not zero.")
						}

						// Delete all MongoDB entries associated with this task ID
						errDel := RemoveTaskID(taskID, mongoDB, l)
						if errDel != nil {
							l.Debug().Str("delete_error", errDel.Error()).Str("task_id", taskID.String()).Msg("Deletion error.")
						}

					}
				}

			}

			//------------------------------------------------------------------
			// Calculate new metrics for this task
			//------------------------------------------------------------------
			thisTaskRecord.ProcessData(l)

			//------------------------------------------------------------------
			// Update task in DB
			//------------------------------------------------------------------
			_, err = thisTaskRecord.UpdateTask(nodeData.ID, test.Framework, task, mongoDB, l)
			if err != nil {
				return err
			}

		}

	}

	return err
}

// Looks for a framework-task-node in the TaskDB and retreives all the IDs adn tasks status
func CheckTaskDatabase(address string, service string, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (tasksInQueue uint32, tasksDone uint32, blackList []int, tasksIDs []primitive.ObjectID, err error) {
	// define blacklist as length zero
	blackList = make([]int, 0)

	// Get tasks collection
	tasksCollection := mongoDB.GetCollection(types.TaskCollection)
	// Get tasks instances
	instancesCollection := mongoDB.GetCollection(types.InstanceCollection)

	// Set filtering for this node-service pair data
	task_request_filter := bson.D{{Key: "requester_args.address", Value: address},
		{Key: "requester_args.service", Value: service},
		{Key: "framework", Value: framework},
		{Key: "tasks", Value: task}}

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Now retrieve all node task requests entries
	cursor, err := tasksCollection.Find(ctxM, task_request_filter)
	if err != nil {
		l.Error().Msg("Could not retrieve task request data from MongoDB.")
		return tasksInQueue, tasksDone, blackList, tasksIDs, err
	}
	defer cursor.Close(ctxM)
	for cursor.Next(ctxM) {
		var taskReq types.TaskRequestRecord
		if err := cursor.Decode(&taskReq); err != nil {
			l.Error().Msg("Could not decode task request data from MongoDB.")
			return tasksInQueue, tasksDone, blackList, tasksIDs, err
		}
		// Save id
		tasksIDs = append(tasksIDs, taskReq.Id)
		// If not already done
		if !taskReq.Done {
			l.Debug().Str("address", address).Str("service", service).Str("framework", framework).Str("task", task).Msg("Found pending task.")
			// Count pending
			tasksInQueue += uint32(taskReq.Qty)

			// Search for associated instances to retrieve ids
			instances_filter := bson.D{{"task_id", taskReq.Id}}
			cursor2, err := instancesCollection.Find(ctxM, instances_filter)
			if err != nil {
				l.Error().Msg("Could not retrieve instances data from MongoDB.")
				return tasksInQueue, tasksDone, blackList, tasksIDs, err
			}
			defer cursor2.Close(ctxM)
			// Get all ids
			for cursor2.Next(ctxM) {
				var thisInstance types.InstanceRecord
				if err := cursor.Decode(&thisInstance); err != nil {
					l.Error().Msg("Could not decode task request data from MongoDB.")
					return tasksInQueue, tasksDone, blackList, tasksIDs, err
				}
				for _, docId := range thisInstance.DocIds {
					blackList = append(blackList, docId)
				}
			}
		} else {
			l.Debug().Str("address", address).Str("service", service).Str("framework", framework).Str("task", task).Msg("Found done task.")
			tasksDone += 1
		}

	}

	l.Debug().Str("address", address).Str("service", service).Str("framework", framework).Str("task", task).Int32("tasksDone", int32(tasksDone)).Int32("tasksInQueue", int32(tasksInQueue)).Int("tasksIDsLen", len(tasksIDs)).Msg("Pending tasks analized.")

	return tasksInQueue, tasksDone, blackList, tasksIDs, err

}

// Given a TaskID from MongoDB, deletes all associated entries from the "tasks", "instances", "prompts", "responses" and "results" collections.
func RemoveTaskID(taskID primitive.ObjectID, mongoDB mongodb.MongoDb, l *zerolog.Logger) (err error) {

	//--------------------------------------------------------------------------
	//-------------------------- Instances -------------------------------------
	//--------------------------------------------------------------------------
	instancesCollection := mongoDB.GetCollection(types.InstanceCollection)
	// Set filtering for this node-service pair data
	task_request_filter := bson.D{{Key: "task_id", Value: taskID}}
	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Now retrieve all node task requests entries
	response, err := instancesCollection.DeleteMany(ctxM, task_request_filter)
	if err != nil {
		l.Warn().Msg("Could not delete instances data from MongoDB.")
		return err
	}

	l.Debug().Int("deleted_count", int(response.DeletedCount)).Str("TaskID", taskID.String()).Msg("deleted instances data from MongoDB")

	//--------------------------------------------------------------------------
	//-------------------------- Prompts ---------------------------------------
	//--------------------------------------------------------------------------
	promptsCollection := mongoDB.GetCollection(types.PromptsCollection)
	// Set filtering for this node-service pair data
	task_request_filter = bson.D{{Key: "task_id", Value: taskID}}
	// Set mongo context
	ctxM, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Now retrieve all node task requests entries
	response, err = promptsCollection.DeleteMany(ctxM, task_request_filter)
	if err != nil {
		l.Warn().Msg("Could not delete prompts data from MongoDB.")
		return err
	}

	l.Debug().Int("deleted", int(response.DeletedCount)).Str("TaskID", taskID.String()).Msg("deleted prompts data from MongoDB")

	//--------------------------------------------------------------------------
	//-------------------------- Responses -------------------------------------
	//--------------------------------------------------------------------------
	responsesCollection := mongoDB.GetCollection(types.ResponsesCollection)
	// Set filtering for this node-service pair data
	task_request_filter = bson.D{{Key: "task_id", Value: taskID}}
	// Set mongo context
	ctxM, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Now retrieve all node task requests entries
	response, err = responsesCollection.DeleteMany(ctxM, task_request_filter)
	if err != nil {
		l.Warn().Msg("Could not delete responses data from MongoDB.")
		return err
	}

	l.Debug().Int("deleted_count", int(response.DeletedCount)).Str("TaskID", taskID.String()).Msg("deleted responses data from MongoDB")

	//--------------------------------------------------------------------------
	//-------------------------- Results ---------------------------------------
	//--------------------------------------------------------------------------
	resultsCollection := mongoDB.GetCollection(types.ResultsCollection)
	// Set filtering for this node-service pair data
	task_request_filter = bson.D{{Key: "result_data.task_id", Value: taskID}}
	// Set mongo context
	ctxM, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Now retrieve all node task requests entries
	response, err = resultsCollection.DeleteMany(ctxM, task_request_filter)
	if err != nil {
		l.Warn().Msg("Could not delete results data from MongoDB.")
		return err
	}

	l.Debug().Int("deleted_count", int(response.DeletedCount)).Str("TaskID", taskID.String()).Msg("deleted results data from MongoDB")

	//--------------------------------------------------------------------------
	//-------------------------- Task ------------------------------------------
	//--------------------------------------------------------------------------
	tasksCollection := mongoDB.GetCollection(types.TaskCollection)
	// Set filtering for this node-service pair data
	task_request_filter = bson.D{{Key: "_id", Value: taskID}}
	// Set mongo context
	ctxM, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Now retrieve all node task requests entries
	response, err = tasksCollection.DeleteMany(ctxM, task_request_filter)
	if err != nil {
		l.Warn().Msg("Could not delete task data from MongoDB.")
		return err
	}

	l.Debug().Int("deleted_count", int(response.DeletedCount)).Str("TaskID", taskID.String()).Msg("deleted task data from MongoDB")

	return nil

}
