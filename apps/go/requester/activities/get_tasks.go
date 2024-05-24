package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"requester/types"
	"time"
)

type GetTasksParams struct {
	// Pass a 0 to get the latest
	Nodes []string `json:"nodes"`
	// chain (morse) service (shannon)
	Service string `json:"service"`
}

type TaskRequest struct {
	//TaskId       string  `json:"task_id" bson:"task_id"`
	//InstanceId   string  `json:"instance_id" bson:"instance_id"`
	PromptId     string  `json:"prompt_id" bson:"prompt_id"`
	Node         string  `json:"node" bson:"node"`
	RelayTimeout float64 `json:"relay_timeout" bson:"relay_timeout"`
}

type GetTaskRequestResults struct {
	TaskRequests []TaskRequest `json:"task_requests"`
}

var GetTasksName = "get_tasks"

func getTaskRequestPipeline(nodes []string, service string) mongo.Pipeline {
	nodesFilter := make(bson.A, len(nodes))
	for i, node := range nodes {
		nodesFilter[i] = bson.M{"requester_args.address": node}
	}

	return mongo.Pipeline{
		bson.D{{"$match", bson.D{
			{"$and", bson.A{
				bson.D{{"$or", nodesFilter}},
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
			{"node", "$requester_args.address"},
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
	// get tasks for the retrieved node and service that are not done yet
	taskCollection := aCtx.App.Mongodb.GetCollection(types.TaskCollection)
	pipeline := getTaskRequestPipeline(params.Nodes, params.Service)
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
