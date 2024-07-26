package activities

import (
	"context"
	"encoding/json"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"requester/types"
	"time"
)

type GetTasksParams struct {
	Nodes       []string `json:"nodes"`
	Application string   `json:"application"`
	// chain (morse) service (shannon)
	Service       string `json:"service"`
	SessionHeight int64  `json:"session_height"`
}

type TaskRequest struct {
	PromptId        string  `json:"prompt_id" bson:"prompt_id"`
	Node            string  `json:"node" bson:"node"`
	RelayTimeout    float64 `json:"relay_timeout" bson:"relay_timeout"`
	RemainingRelays int64   `json:"remaining_relays" bson:"remaining_relays"`
}

type GetTaskRequestResults struct {
	TaskRequests []TaskRequest `json:"task_requests"`
}

var GetTasksName = "get_tasks"

func getTaskRequestPipeline(nodes []string, application, service string, sessionHeight, maxRelaysPerSession int64) mongo.Pipeline {
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
			{"prompt_id", "$prompt._id"},
			{"node", "$requester_args.address"},
			{"relay_timeout", "$prompt.timeout"},
		}}},
		bson.D{{"$lookup", bson.D{
			{"from", "relays_by_session"},
			{"let", bson.D{
				{"servicer", "$node"},
				{"application", application},
				{"service", service},
				{"session_height", sessionHeight},
			}},
			{"pipeline", bson.A{
				bson.D{{"$match", bson.D{
					{"$expr", bson.D{
						{"$and", bson.A{
							bson.D{{"$eq", bson.A{"$servicer", "$$servicer"}}},
							bson.D{{"$eq", bson.A{"$application", "$$application"}}},
							bson.D{{"$eq", bson.A{"$service", "$$service"}}},
							bson.D{{"$eq", bson.A{"$session_height", "$$session_height"}}},
						}},
					}},
				}}},
				bson.D{{"$project", bson.D{
					{"relays", 1},
				}}},
			}},
			{"as", "relays_by_session"},
		}}},
		bson.D{{"$unwind", bson.D{
			{"path", "$relays_by_session"},
			{"preserveNullAndEmptyArrays", true},
		}}},
		bson.D{{"$project", bson.D{
			{"_id", 0},
			{"prompt_id", 1},
			{"node", 1},
			{"relays", bson.D{{"$ifNull", bson.A{
				"$relays_by_session.relays",
				0,
			}}}},
		}}},
		bson.D{{"$match", bson.D{{"relays", bson.D{{"$lt", maxRelaysPerSession}}}}}},
		bson.D{{"$group", bson.D{
			{"_id", "$node"},
			{"consumed_relays", bson.D{{"$first", "$relays"}}},
			{"prompts", bson.D{{"$addToSet", bson.D{
				{"prompt_id", "$prompt_id"},
				{"relay_timeout", "$relay_timeout"},
			}}}},
		}}},
		bson.D{{"$set", bson.D{
			{"remaining_relays", bson.D{
				{"$subtract", bson.A{maxRelaysPerSession, "$consumed_relays"}},
			}},
			{"prompts", bson.D{{"$sortArray", bson.D{
				{"input", "$prompts"},
				{"sortBy", bson.D{{"prompt_id", 1}}},
			}}}},
		}}},
		bson.D{{"$project", bson.D{
			{"_id", 0},
			{"node", "$_id"},
			{"prompts", bson.D{
				{"$slice", bson.A{
					"$prompts",
					"$remaining_relays",
				}},
			}},
			{"remaining_relays", 1},
		}}},
		bson.D{{"$unwind", bson.D{
			{"path", "$prompts"},
			{"preserveNullAndEmptyArrays", false},
		}}},
		bson.D{{"$project", bson.D{
			{"node", 1},
			{"prompt_id", "$prompts.prompt_id"},
			{"relay_timeout", "$prompts.relay_timeout"},
			{"remaining_relays", 1},
		}}},
		// 15k of pending task request is a crazy amount, larger than this will throw an error on Temporal due to the
		// size of the JSON payload.
		bson.D{{"$limit", 15000}},
	}
}

func PrintPipeline(queryName string, pipeline mongo.Pipeline, logger *zerolog.Logger) error {
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

	logger.Info().
		Str("QueryName", queryName).
		Str("Pipeline", string(prettyJSON)).
		Msg("MongoDB Pipeline")

	return nil
}

func (aCtx *Ctx) GetTasks(ctx context.Context, params GetTasksParams) (result *GetTaskRequestResults, e error) {
	result = &GetTaskRequestResults{TaskRequests: make([]TaskRequest, 0)}
	tasksCtx, taskCancelFn := context.WithTimeout(ctx, 5*time.Minute)
	defer taskCancelFn()
	// get tasks for the retrieved node and service that are not done yet
	taskCollection := aCtx.App.Mongodb.GetCollection(types.TaskCollection)
	pipeline := getTaskRequestPipeline(
		params.Nodes, params.Application,
		params.Service, params.SessionHeight,
		aCtx.App.Config.Rpc.RelayPerSession,
	)
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
