package activities

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
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
	TaskId       string  `json:"task_id" bson:"task_id"`
	InstanceId   string  `json:"instance_id" bson:"instance_id"`
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
			{"task_id", "$_id"},
			{"instance_id", "$instance._id"},
			{"prompt_id", "$prompt._id"},
			{"node", "$requester_args.address"},
			{"relay_timeout", "$prompt.timeout"},
		}}},
	}
}

func (aCtx *Ctx) GetTasks(ctx context.Context, params GetTasksParams) (result *GetTaskRequestResults, e error) {
	result = &GetTaskRequestResults{TaskRequests: make([]TaskRequest, 0)}
	tasksCtx, taskCancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer taskCancelFn()
	// get tasks for the retrieved node and service that are not done yet
	taskCollection := aCtx.App.Mongodb.GetCollection(types.TaskCollection)
	pipeline := getTaskRequestPipeline(params.Nodes, params.Service)
	cursor, AggErr := taskCollection.Aggregate(tasksCtx, pipeline)
	if AggErr != nil {
		return nil, AggErr
	}
	decodeErr := cursor.All(tasksCtx, &result.TaskRequests)
	if decodeErr != nil {
		return nil, AggErr
	}
	return
}
