package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"requester/types"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.temporal.io/sdk/temporal"
)

type GetTasksParams struct {
	// Pass a 0 to get the latest
	Suppliers []string `json:"suppliers"`
	// Service (aka chain in Morse network)
	Service string `json:"service"`
	// current session height
	CurrentSession int64 `json:"current_session"`
}

type SetPromptTriggerSessionParams struct {
	PromptId       string `json:"prompt_id" bson:"prompt_id"`
	TriggerSession int64  `json:"trigger_session"`
}

type TaskRequest struct {
	//TaskId       string  `json:"task_id" bson:"task_id"`
	//InstanceId   string  `json:"instance_id" bson:"instance_id"`
	PromptId     string  `json:"prompt_id" bson:"prompt_id"`
	Supplier     string  `json:"supplier" bson:"supplier"`
	RelayTimeout float64 `json:"relay_timeout" bson:"relay_timeout"`
}

type GetTaskRequestResults struct {
	TaskRequests []TaskRequest `json:"task_requests"`
}

var GetTasksName = "get_tasks"
var SetPromptTriggerSessionName = "set_prompt_trigger_session"

func getTaskRequestPipeline(suppliers []string, service string, currentSession int64) mongo.Pipeline {
	suppliersFilter := make(bson.A, len(suppliers))
	for i, supplier := range suppliers {
		suppliersFilter[i] = bson.M{"requester_args.address": supplier}
	}

	return mongo.Pipeline{
		bson.D{{"$match", bson.D{
			{"$and", bson.A{
				bson.D{{"$or", suppliersFilter}},
				bson.D{
					{"requester_args.service", service},
					{"done", false},
				},
			}},
		}}},
		bson.D{{"$lookup", bson.D{
			{"from", "instances"},
			{"let", bson.D{{"task", "$_id"}}},
			{"pipeline", bson.A{
				bson.D{{"$match", bson.D{
					{"$expr", bson.D{
						{"$and", bson.A{
							bson.D{{"$eq", bson.A{"$task_id", "$$task"}}},
							bson.D{{"$eq", bson.A{"$done", false}}},
						}},
					}},
				}}}}},
			{"as", "instance"},
		}}},
		bson.D{{"$unwind", bson.D{
			{"path", "$instance"},
			{"preserveNullAndEmptyArrays", false},
		}}},
		bson.D{{"$lookup", bson.D{
			{"from", "prompts"},
			{"let", bson.D{
				{"task", "$_id"},
				{"instance", "$instance._id"},
			}},
			{"pipeline", bson.A{
				bson.D{{"$match", bson.D{
					{"$expr", bson.D{
						{"$and", bson.A{
							bson.D{{"$eq", bson.A{"$task_id", "$$task"}}},
							bson.D{{"$eq", bson.A{"$instance_id", "$$instance"}}},
							bson.D{{"$eq", bson.A{"$done", false}}},
							bson.D{{"$lt", bson.A{"$trigger_session", currentSession}}},
						}},
					}},
				}}},
			}},
			{"as", "prompt"},
		}}},
		bson.D{{"$unwind", bson.D{
			{"path", "$prompt"},
			{"preserveNullAndEmptyArrays", false},
		}}},
		bson.D{{"$project", bson.D{
			{"_id", 0},
			//{"task_id", "$_id"},
			//{"instance_id", "$instance._id"},
			{"prompt_id", "$prompt._id"},
			{"supplier", "$requester_args.address"},
			{"relay_timeout", "$prompt.timeout"},
		}}},
		// 15k of pending task request is a crazy amount, larger than this will throw an error on Temporal due to the
		// size of the JSON payload.
		bson.D{{"$limit", 15000}},
	}
}

func PrintPipeline(queryName string, pipeline mongo.Pipeline) error {
	var prettyDocs []bson.M

	for _, doc := range pipeline {
		bsonDoc, err := bson.Marshal(doc)
		if err != nil {
			return err
		}
		var prettyDoc bson.M
		err = bson.Unmarshal(bsonDoc, &prettyDoc)
		if err != nil {
			return err
		}
		prettyDocs = append(prettyDocs, prettyDoc)
	}

	prettyJSON, err := json.Marshal(prettyDocs)
	if err != nil {
		return err
	}

	fmt.Printf("Query=%s Pipeline=%s", queryName, string(prettyJSON))

	return nil
}

func (aCtx *Ctx) GetTasks(ctx context.Context, params GetTasksParams) (result *GetTaskRequestResults, e error) {
	result = &GetTaskRequestResults{TaskRequests: make([]TaskRequest, 0)}
	tasksCtx, taskCancelFn := context.WithTimeout(ctx, 5*time.Minute)
	defer taskCancelFn()
	// get tasks for the retrieved supplier and service that are not done yet
	taskCollection := aCtx.App.Mongodb.GetCollection(types.TaskCollection)
	pipeline := getTaskRequestPipeline(params.Suppliers, params.Service, params.CurrentSession)
	// PrintPipeline("tasksInstancePrompts", pipeline)
	opts := options.Aggregate().SetAllowDiskUse(true)
	cursor, aggErr := taskCollection.Aggregate(tasksCtx, pipeline, opts)
	if aggErr != nil {
		return nil, aggErr
	}
	decodeErr := cursor.All(tasksCtx, &result.TaskRequests)
	if decodeErr != nil {
		return nil, aggErr
	}
	return
}

func SplitByUniqueAddress(input []TaskRequest) map[string][]TaskRequest {
	// Create a map to store requests with the same address
	nameToStructs := make(map[string][]TaskRequest)

	// Iterate through the input list
	for _, s := range input {
		// Check if this supplier name is already in the list
		_, ok := nameToStructs[s.Supplier]
		if !ok {
			// This is a new address, create an empty list of tasks
			nameToStructs[s.Supplier] = make([]TaskRequest, 0)
		}
		// Append this task to this supplier list
		nameToStructs[s.Supplier] = append(nameToStructs[s.Supplier], s)
	}

	return nameToStructs
}

func (aCtx *Ctx) SetPromptTriggerSession(ctx context.Context, params SetPromptTriggerSessionParams) (err error) {
	tasksCtx, taskCancelFn := context.WithTimeout(ctx, 5*time.Minute)
	defer taskCancelFn()

	// get prompts collection
	promptsCollection := aCtx.App.Mongodb.GetCollection(types.PromptsCollection)
	// Set the find options using the prompt id
	promptId, objIdErr := primitive.ObjectIDFromHex(params.PromptId)
	if objIdErr != nil {
		err = temporal.NewApplicationErrorWithCause("invalid prompt id", "BadParams", objIdErr, params)
		return
	}
	prompt_filter := bson.M{"_id": promptId}

	// Update given struct
	update := bson.M{"$set": bson.M{"trigger_session": params.TriggerSession}}

	// Get collection and update
	result, err := promptsCollection.UpdateOne(tasksCtx, prompt_filter, update)
	// Check if any document was modified
	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments // No document found with this ID
	}

	return err

}
