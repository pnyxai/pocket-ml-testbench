package activities

import (
	"context"
	"fmt"
	"manager/records"
	"manager/types"
	"packages/mongodb"
	"time"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.temporal.io/sdk/temporal"
)

var AnalyzeResultName = "analyze_result"

func (aCtx *Ctx) AnalyzeResult(ctx context.Context, params types.AnalyzeResultParams) (*types.AnalyzeResultResults, error) {

	var result types.AnalyzeResultResults
	result.Success = false

	// Get logger
	l := aCtx.App.Logger
	l.Debug().
		Str("task_id", params.TaskID.String()).
		Msg("Analyzing task.")

	// Get results collection
	resultsCollection := aCtx.App.Mongodb.GetCollection(types.ResultsCollection)

	//------------------------------------------------------------------
	// Get Task data
	//------------------------------------------------------------------
	taskData, err := retrieveTaskData(params.TaskID, aCtx.App.Mongodb, l)
	if err != nil {
		err = temporal.NewApplicationErrorWithCause("unable to get task data", "retrieveTaskData", fmt.Errorf("Task %s not found", params.TaskID.String()))
		return nil, err
	}
	if taskData.Drop {
		// The task has failed for some reason (result of mark_task_to_drop ),
		// we cannot proceed and we must delete the task data
		if !aCtx.App.Config.DevelopCfg.DoNotRemoveTasksFromDB {
			RemoveTaskID(params.TaskID, aCtx.App.Mongodb, l)
		}
		// this was successfully analyzed
		result.Success = true
		return &result, nil
	}
	// Extract data
	Supplier := types.SupplierData{
		Address: taskData.RequesterArgs.Address,
		Service: taskData.RequesterArgs.Service,
	}

	l.Debug().
		Str("task_id", params.TaskID.String()).
		Str("address", Supplier.Address).
		Str("service", Supplier.Service).
		Str("framework", taskData.Framework).
		Str("task", taskData.Task).
		Msg("Analyzing result.")

	//------------------------------------------------------------------
	// Get stored data for this supplier
	//------------------------------------------------------------------
	var supplierData records.SupplierRecord
	found, err := supplierData.FindAndLoadSupplier(Supplier, aCtx.App.Mongodb, l)
	if err != nil {
		return nil, err
	}

	if !found {
		err = temporal.NewApplicationErrorWithCause("unable to get supplier data", "FindAndLoadSupplier", fmt.Errorf("Supplier %s not found", Supplier.Address))
		l.Error().
			Str("address", Supplier.Address).
			Msg("Cannot retrieve supplier data")
		return nil, err
	}

	//------------------------------------------------------------------
	// Get stored data for this task
	//------------------------------------------------------------------
	taskType, err := records.GetTaskType(taskData.Framework, taskData.Task, aCtx.App.Config.Frameworks, l)
	if err != nil {
		return nil, err
	}
	thisTaskRecord, found := records.GetTaskData(supplierData.ID, taskType, taskData.Framework, taskData.Task, true, aCtx.App.Mongodb, l)
	if !found {
		// Data should be found because we are creating it in the last
		err = temporal.NewApplicationErrorWithCause("unable to get task buffer data", "GetTaskData", fmt.Errorf("Task %s not found", taskData.Task))
		l.Error().
			Str("address", supplierData.Address).
			Str("service", supplierData.Service).
			Str("framework", taskData.Framework).
			Str("task", taskData.Task).
			Msg("Requested task was not found.")
		return nil, err
	}

	thisTaskResults := thisTaskRecord.GetResultStruct()
	found = false
	found, err = thisTaskResults.FindAndLoadResults(params.TaskID,
		resultsCollection,
		l)
	if err != nil {
		return nil, err
	}
	if !found {
		l.Error().
			Str("address", supplierData.Address).
			Str("service", supplierData.Service).
			Str("framework", taskData.Framework).
			Str("task", taskData.Task).
			Msg("Requested result was not found.")
	}

	l.Debug().
		Str("address", supplierData.Address).
		Str("service", supplierData.Service).
		Str("framework", taskData.Framework).
		Str("task", taskData.Task).
		Str("task_id", params.TaskID.String()).
		Msg("Processing found results.")

	// If nothing is wrong with the result calculation
	// (this does not mean that the RPC error codes were checked or not,
	// only that the calculation was successful, even when the calculation
	// itself used no sample )
	if thisTaskResults.GetStatus() == 0 {
		if thisTaskResults.GetNumSamples() == 0 {
			l.Warn().
				Str("address", supplierData.Address).
				Str("service", supplierData.Service).
				Str("framework", taskData.Framework).
				Str("task", taskData.Task).
				Str("task_id", params.TaskID.String()).
				Msg("Has status 0 but no samples/results to insert, the tasks will be consumed with no effect on the score.")
		} else {
			l.Debug().
				Int("NumSamples", int(thisTaskResults.GetNumSamples())).
				Str("address", supplierData.Address).
				Str("service", supplierData.Service).
				Str("framework", taskData.Framework).
				Str("task", taskData.Task).
				Str("task_id", params.TaskID.String()).
				Msg("Inserting results into buffers.")
			// Add results to current task record
			// This inclusion is conditional on the status of the RPC.
			total_ok := 0
			for i := 0; i < int(thisTaskResults.GetNumSamples()); i++ {
				ok, err := thisTaskRecord.InsertSample(time.Now(), thisTaskResults.GetSample(i), l)
				if err != nil {
					l.Error().
						Err(err).
						Str("address", supplierData.Address).
						Str("service", supplierData.Service).
						Str("framework", taskData.Framework).
						Str("task", taskData.Task).
						Msg("Wrong buffer class (really weird...).")
					return nil, err
				}
				if ok {
					total_ok += 1
				}
			}
			if total_ok > 0 {
				// Update the last OK fields, because we have seen the supplier
				// responding to a call successfully at least once.
				thisTaskRecord.UpdateLastOkHeight(thisTaskResults.GetResultHeight())
				thisTaskRecord.UpdateLastOk(thisTaskResults.GetResultTime())
			}
			thisTaskRecord.UpdateLastHeight(thisTaskResults.GetResultHeight())
			thisTaskRecord.UpdateLastSeen(thisTaskResults.GetResultTime())

		}

	} else {
		// TODO: handle status!=0
		l.Debug().
			Str("address", supplierData.Address).
			Str("service", supplierData.Service).
			Str("framework", taskData.Framework).
			Str("task", taskData.Task).
			Str("task_id", params.TaskID.String()).
			Msg("Status not zero.")
	}

	// Delete all MongoDB entries associated with this task ID
	if !aCtx.App.Config.DevelopCfg.DoNotRemoveTasksFromDB {
		RemoveTaskID(params.TaskID, aCtx.App.Mongodb, l)
	}

	//------------------------------------------------------------------
	// Calculate new metrics for this task
	//------------------------------------------------------------------
	thisTaskRecord.ProcessData(l)

	//------------------------------------------------------------------
	// Update task in DB
	//------------------------------------------------------------------

	_, err = thisTaskRecord.UpdateTask(supplierData.ID, taskData.Framework, taskData.Task, aCtx.App.Mongodb, l)
	if err != nil {
		return nil, err
	}

	result.Success = true

	return &result, nil
}

// Looks for an specific task in the TaskDB and retrieves all data
func retrieveTaskData(taskID primitive.ObjectID,
	mongoDB mongodb.MongoDb,
	l *zerolog.Logger) (tasksData types.TaskRequestRecord,
	err error) {

	// Get tasks collection
	tasksCollection := mongoDB.GetCollection(types.TaskCollection)

	// Set filtering for this task
	task_request_filter := bson.D{{Key: "_id", Value: taskID}}
	opts := options.FindOne()
	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Now retrieve all supplier's task requests entries
	cursor := tasksCollection.FindOne(ctxM, task_request_filter, opts)
	var taskReq types.TaskRequestRecord
	if err := cursor.Decode(&taskReq); err != nil {
		l.Debug().Str("taskID", taskID.String()).Msg("Could not decode task request data from MongoDB.")
		return taskReq, err
	}

	return taskReq, nil

}

// Given a TaskID from MongoDB, deletes all associated entries from the "tasks", "instances", "prompts", "responses" and "results" collections.
func RemoveTaskID(taskID primitive.ObjectID, mongoDB mongodb.MongoDb, l *zerolog.Logger) {

	//--------------------------------------------------------------------------
	//-------------------------- Instances -------------------------------------
	//--------------------------------------------------------------------------
	instancesCollection := mongoDB.GetCollection(types.InstanceCollection)
	// Set filtering for this supplier-service pair data
	task_request_filter := bson.D{{Key: "task_id", Value: taskID}}
	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Now retrieve all supplier's task requests entries
	response, err := instancesCollection.DeleteMany(ctxM, task_request_filter)
	if err != nil {
		l.Warn().Err(err).Msg("Could not delete instances data from MongoDB.")
	} else {
		l.Debug().Int("deleted_count", int(response.DeletedCount)).Str("TaskID", taskID.String()).Msg("deleted instances data from MongoDB")
	}

	//--------------------------------------------------------------------------
	//-------------------------- Prompts ---------------------------------------
	//--------------------------------------------------------------------------
	promptsCollection := mongoDB.GetCollection(types.PromptsCollection)
	// Set filtering for this supplier-service pair data
	task_request_filter = bson.D{{Key: "task_id", Value: taskID}}
	// Set mongo context
	ctxM, cancel = context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Now retrieve all supplier task requests entries
	response, err = promptsCollection.DeleteMany(ctxM, task_request_filter)
	if err != nil {
		l.Warn().Err(err).Msg("Could not delete prompts data from MongoDB.")
	} else {
		l.Debug().Int("deleted", int(response.DeletedCount)).Str("TaskID", taskID.String()).Msg("deleted prompts data from MongoDB")
	}

	//--------------------------------------------------------------------------
	//-------------------------- Responses -------------------------------------
	//--------------------------------------------------------------------------
	responsesCollection := mongoDB.GetCollection(types.ResponsesCollection)
	// Set filtering for this supplier-service pair data
	task_request_filter = bson.D{{Key: "task_id", Value: taskID}}
	// Set mongo context
	ctxM, cancel = context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Now retrieve all supplier task requests entries
	response, err = responsesCollection.DeleteMany(ctxM, task_request_filter)
	if err != nil {
		l.Warn().Err(err).Msg("Could not delete responses data from MongoDB.")
	} else {
		l.Debug().Int("deleted_count", int(response.DeletedCount)).Str("TaskID", taskID.String()).Msg("deleted responses data from MongoDB")
	}

	//--------------------------------------------------------------------------
	//-------------------------- Results ---------------------------------------
	//--------------------------------------------------------------------------
	resultsCollection := mongoDB.GetCollection(types.ResultsCollection)
	// Set filtering for this supplier-service pair data
	task_request_filter = bson.D{{Key: "result_data.task_id", Value: taskID}}
	// Set mongo context
	ctxM, cancel = context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Now retrieve all supplier task requests entries
	response, err = resultsCollection.DeleteMany(ctxM, task_request_filter)
	if err != nil {
		l.Warn().Err(err).Msg("Could not delete results data from MongoDB.")
	} else {
		l.Debug().Int("deleted_count", int(response.DeletedCount)).Str("TaskID", taskID.String()).Msg("deleted results data from MongoDB")
	}

	//--------------------------------------------------------------------------
	//-------------------------- Task ------------------------------------------
	//--------------------------------------------------------------------------
	tasksCollection := mongoDB.GetCollection(types.TaskCollection)
	// Set filtering for this supplier-service pair data
	task_request_filter = bson.D{{Key: "_id", Value: taskID}}
	// Set mongo context
	ctxM, cancel = context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	// Now retrieve all supplier task requests entries
	response, err = tasksCollection.DeleteMany(ctxM, task_request_filter)
	if err != nil {
		l.Warn().Err(err).Msg("Could not delete task data from MongoDB.")
	} else {
		l.Debug().Int("deleted_count", int(response.DeletedCount)).Str("TaskID", taskID.String()).Msg("deleted task data from MongoDB")
	}

}
