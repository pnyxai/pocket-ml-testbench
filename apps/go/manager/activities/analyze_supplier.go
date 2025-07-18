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

var AnalyzeSupplierName = "analyze_supplier"

func (aCtx *Ctx) AnalyzeSupplier(ctx context.Context, params types.AnalyzeSupplierParams) (*types.AnalyzeSupplierResults, error) {

	var result types.AnalyzeSupplierResults
	result.Success = false
	result.IsNew = false

	// Get logger
	l := aCtx.App.Logger
	l.Debug().
		Str("address", params.Supplier.Address).
		Str("service", params.Supplier.Service).
		Msg("Analyzing staked supplier.")

	// Get current height and time
	currHeight, err := aCtx.App.PocketFullNode.GetLatestBlockHeight()
	if err != nil {
		l.Error().
			Str("supplier", params.Supplier.Address).
			Str("service", params.Supplier.Service).
			Msg("Could not retrieve latest block height.")
		return nil, err
	}
	currTime := time.Now()

	// Retrieve this supplier entry
	var thisSupplierData records.SupplierRecord
	found, err := thisSupplierData.FindAndLoadSupplier(params.Supplier, aCtx.App.Mongodb, l)
	if err != nil {
		l.Error().
			Err(err).
			Str("address", params.Supplier.Address).
			Str("service", params.Supplier.Service).
			Msg("Failed to load supplier data.")
		return nil, err
	}

	//--------------------------------------------------------------------------
	// Update all tasks buffers
	//--------------------------------------------------------------------------
	var LastSeenHeight int64
	var LastSeenTime time.Time
	if !found {
		// Create entry in MongoDB
		l.Debug().Bool("found", found).Msg("Creating empty supplier entry.")
		err = thisSupplierData.Init(params, aCtx.App.Config.Frameworks, aCtx.App.Mongodb, l)
		if err != nil {
			l.Error().Err(err).
				Str("address", params.Supplier.Address).
				Str("service", params.Supplier.Service).
				Msg("Failed to create supplier entry.")
			return nil, err
		}
		result.IsNew = true
		LastSeenHeight = currHeight
		LastSeenTime = currTime

	} else {
		// If the supplier entry exist we must cycle and check for pending results
		LastSeenHeight, LastSeenTime, err = updateTasksSupplier(&thisSupplierData, params.Tests, aCtx.App.Config.Frameworks, aCtx.App.Mongodb, l)
		if err != nil {
			l.Error().
				Err(err).
				Str("address", params.Supplier.Address).
				Str("service", params.Supplier.Service).
				Msg("Failed to create update supplier.")
			return nil, err
		}
	}

	// Do general update of supplier
	thisSupplierData.LastProcessHeight = currHeight
	thisSupplierData.LastProcessTime = currTime
	thisSupplierData.LastSeenHeight = LastSeenHeight
	thisSupplierData.LastSeenTime = LastSeenTime

	// Push to DB the supplier data
	l.Debug().Msg("Uploading supplier changes to DB.")
	_, err = thisSupplierData.UpdateSupplier(aCtx.App.Mongodb, l)
	if err != nil {
		l.Error().Err(err).Str("address", params.Supplier.Address).Str("service", params.Supplier.Service).Msg("Failed upload supplier to MongoDB.")
		return nil, err
	}

	//--------------------------------------------------------------------------
	// Trigger incomplete tasks
	//--------------------------------------------------------------------------

	// Loop over all tasks and frameworks
	for _, test := range params.Tests {

		for _, task := range test.Tasks {
			l.Debug().
				Str("address", thisSupplierData.Address).
				Str("service", thisSupplierData.Service).
				Str("framework", test.Framework).
				Str("task", task).
				Msg("Checking task requests.")

			// Check taxonomy dependencies
			depStatus, err := records.CheckTaxonomyDependency(&thisSupplierData, test.Framework, task, aCtx.App.Config.Frameworks, aCtx.App.Mongodb, l)
			if err != nil {
				l.Error().Err(err).
					Msg("Could not check taxonomy dependencies.")
				return nil, err
			}
			if !depStatus {
				l.Debug().
					Str("address", thisSupplierData.Address).
					Str("service", thisSupplierData.Service).
					Str("framework", test.Framework).
					Str("task", task).
					Msg("Does not meet taxonomy dependencies, ignoring for now.")
				continue
			}

			// Check task dependencies
			depStatus, err = records.CheckTaskDependency(&thisSupplierData, test.Framework, task, aCtx.App.Config.Frameworks, aCtx.App.Mongodb, l)
			if err != nil {
				l.Error().Err(err).
					Msg("Could not check task dependencies.")
				return nil, err
			}
			if !depStatus {
				l.Debug().
					Str("address", thisSupplierData.Address).
					Str("service", thisSupplierData.Service).
					Str("framework", test.Framework).
					Str("task", task).
					Msg("Does not meet task dependencies, ignoring for now.")
				continue
			}

			// Get task record
			taskType, err := records.GetTaskType(test.Framework, task, aCtx.App.Config.Frameworks, l)
			if err != nil {
				l.Error().Err(err).Msg("cannot retrieve task type")
				return nil, fmt.Errorf("cannot retrieve task type")
			}
			thisTaskRecord, found := records.GetTaskData(thisSupplierData.ID, taskType, test.Framework, task, true, aCtx.App.Mongodb, l)
			if found != true {
				l.Error().
					Str("address", thisSupplierData.Address).
					Str("service", thisSupplierData.Service).
					Str("framework", test.Framework).
					Str("task", task).
					Msg("not found task entry after check creation (task should have been created)")
				return nil, fmt.Errorf("not found task entry after check creation (task should have been created)")
			}

			// Check schedule restrictions
			schdStatus, err := records.CheckTaskSchedule(thisTaskRecord, params.Block, aCtx.App.Config.Frameworks, l)
			if err != nil {
				l.Error().Err(err).Msg("Could not check task schedule.")
				return nil, err
			}
			if !schdStatus {
				l.Debug().Str("address", thisSupplierData.Address).Str("service", thisSupplierData.Service).Str("framework", test.Framework).Str("task", task).Msg("Does not meet task schedule, ignoring for now.")
				continue
			}

			// The schedule is OK, now check minimum tasks to trigger
			minTrigger, err := records.CheckTaskTriggerMin(thisTaskRecord, params.Block, aCtx.App.Config.Frameworks, l)
			if err != nil {
				l.Error().Err(err).Msg("Could not check task minimum trigger value.")
				return nil, err
			}

			// If the number of samples is less than the minimum or there is a minimum value to trigger, proceed to request more
			numberOfSamples := thisTaskRecord.GetNumOkSamples()
			l.Debug().Str("address", thisSupplierData.Address).Str("service", thisSupplierData.Service).Str("framework", test.Framework).Str("task", task).Uint32("numberOfSamples", numberOfSamples).Msg("Ok sample count.")
			if numberOfSamples < thisTaskRecord.GetMinSamplesPerTask() || minTrigger > 0 {

				// Calculate the total number of request needed
				reqNeeded := thisTaskRecord.GetMinSamplesPerTask() - numberOfSamples
				// Check if this exceed the max concurrent task and limit
				maxConcurrentTasks := thisTaskRecord.GetMaxConcurrentSamplesPerTask()
				if reqNeeded > maxConcurrentTasks {
					reqNeeded = maxConcurrentTasks
				}

				// Get number of tasks in queue
				inQueue, _, blackList, _, err := checkTaskDatabase(thisSupplierData.Address, thisSupplierData.Service, test.Framework, task, aCtx.App.Mongodb, l)
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
						thisTrigger := types.TaskTrigger{Address: thisSupplierData.Address,
							Service:   thisSupplierData.Service,
							Framework: test.Framework,
							Task:      task,
							Blacklist: blackList,
							Qty:       int(reqNeeded)}
						result.Triggers = append(result.Triggers, thisTrigger)
					}
				} else {
					l.Debug().Str("address", thisSupplierData.Address).Str("service", thisSupplierData.Service).Str("framework", test.Framework).Str("task", task).Msg("Pending requests capped.")
				}

			} else {
				l.Debug().Str("address", thisSupplierData.Address).Str("service", thisSupplierData.Service).Str("framework", test.Framework).Str("task", task).Msg("Buffer filled and up to date.")
			}
		}
	}

	result.Success = true

	return &result, nil
}

// Checks for suppliers's tasks records and drops old ones.
func updateTasksSupplier(supplierData *records.SupplierRecord,
	tests []types.TestsData,
	frameworkConfigMap map[string]types.FrameworkConfig,
	mongoDB mongodb.MongoDb,
	l *zerolog.Logger) (LastSeenHeight int64, LastSeenTime time.Time, err error) {

	//--------------------------------------------------------------------------
	// Check for each task sample date
	//--------------------------------------------------------------------------
	for _, test := range tests {

		for _, task := range test.Tasks {

			l.Debug().
				Str("address", supplierData.Address).
				Str("service", supplierData.Service).
				Str("framework", test.Framework).
				Str("task", task).
				Msg("Updating circular buffer.")

			//------------------------------------------------------------------
			// Get stored data for this task
			//------------------------------------------------------------------
			taskType, err := records.GetTaskType(test.Framework, task, frameworkConfigMap, l)
			if err != nil {
				return LastSeenHeight, LastSeenTime, err
			}
			thisTaskRecord, found := records.GetTaskData(supplierData.ID, taskType, test.Framework, task, false, mongoDB, l)

			if !found {
				l.Debug().
					Str("address", supplierData.Address).
					Str("service", supplierData.Service).
					Str("framework", test.Framework).
					Str("task", task).
					Msg("Not found, skipping.")
				continue
			}

			//------------------------------------------------------------------
			// Drop old samples (move indices).
			//------------------------------------------------------------------

			l.Debug().
				Str("address", supplierData.Address).
				Str("service", supplierData.Service).
				Str("framework", test.Framework).
				Str("task", task).
				Msg("Cycling indexes.")
			cycled, err := thisTaskRecord.CycleIndexes(l)
			if err != nil {
				return LastSeenHeight, LastSeenTime, err
			}

			//------------------------------------------------------------------
			// Update task in DB
			//------------------------------------------------------------------
			if cycled || found {
				l.Debug().
					Str("address", supplierData.Address).
					Str("service", supplierData.Service).
					Str("framework", test.Framework).
					Str("task", task).
					Msg("Updating task entry.")
				_, err = thisTaskRecord.UpdateTask(supplierData.ID, test.Framework, task, mongoDB, l)
				if err != nil {
					return LastSeenHeight, LastSeenTime, err
				}
			}

			//------------------------------------------------------------------
			// Track Last Seen
			//------------------------------------------------------------------
			if LastSeenHeight < thisTaskRecord.GetLastHeight() {
				LastSeenHeight = thisTaskRecord.GetLastHeight()
				LastSeenTime = thisTaskRecord.GetLastSeen()
			}

		}

	}

	// If the tracked last seen height is lower than the last record we have
	// it means that the supplier was seen by another framework after the
	// framework that we are analyzing here, so we must keep the largest value.
	if LastSeenHeight < supplierData.LastSeenHeight {
		LastSeenHeight = supplierData.LastSeenHeight
		LastSeenTime = supplierData.LastSeenTime
	}

	return LastSeenHeight, LastSeenTime, err
}

// Looks for a framework-task-suppliers in the TaskDB and retrieves all the IDs and tasks status
func checkTaskDatabase(address string,
	service string,
	framework string,
	task string,
	mongoDB mongodb.MongoDb,
	l *zerolog.Logger) (tasksInQueue uint32,
	tasksDone uint32,
	blackList []int,
	tasksIDs []primitive.ObjectID,
	err error) {
	// define blacklist as length zero
	blackList = make([]int, 0)

	// Get tasks collection
	tasksCollection := mongoDB.GetCollection(types.TaskCollection)
	// Get tasks instances
	instancesCollection := mongoDB.GetCollection(types.InstanceCollection)

	// Set filtering for this supplier-service pair data
	task_request_filter := bson.D{{Key: "requester_args.address", Value: address},
		{Key: "requester_args.service", Value: service},
		{Key: "framework", Value: framework},
		{Key: "tasks", Value: task}}

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Now retrieve all supplier task requests entries
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
			l.Debug().
				Str("address", address).
				Str("service", service).
				Str("framework", framework).
				Str("task", task).
				Msg("Found pending task.")
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
			l.Debug().
				Str("address", address).
				Str("service", service).
				Str("framework", framework).
				Str("task", task).
				Msg("Found done task.")
			tasksDone += 1
		}

	}

	l.Debug().
		Str("address", address).
		Str("service", service).
		Str("framework", framework).
		Str("task", task).
		Int32("tasksDone", int32(tasksDone)).
		Int32("tasksInQueue", int32(tasksInQueue)).
		Int("tasksIDsLen", len(tasksIDs)).
		Msg("Pending tasks analyzed.")

	return tasksInQueue, tasksDone, blackList, tasksIDs, err

}
