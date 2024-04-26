package activities

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.temporal.io/sdk/temporal"
	"packages/logger"
	"packages/mongodb"
	"requester/types"
	"time"
)

type LookupTaskRequestParams struct {
	// Pass a 0 to get the latest
	Node string `json:"node"`
	// chain (morse) service (shannon)
	Service string `json:"service"`
}

type CompactTaskRequest struct {
	TaskId     string `json:"task_id"`
	InstanceId string `json:"instance_id"`
	PromptId   string `json:"prompt_id"`
}

type LookupTaskRequestResults struct {
	TaskRequests []CompactTaskRequest `json:"task_requests"`
}

type promptFilter struct {
	TaskId     primitive.ObjectID
	InstanceId primitive.ObjectID
}

var LookupTaskRequestName = "lookup_task_request"

func GetRecords[T interface{}](ctx context.Context, collection mongodb.CollectionAPI, filter interface{}, opts ...*options.FindOptions) (docs []*T, e error) {
	cursor, err := collection.Find(ctx, filter, opts...)
	if err != nil {
		e = err
		return
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		er := cursor.Close(ctx)
		if er != nil {
		}
	}(cursor, ctx)
	if e = cursor.All(context.Background(), &docs); e != nil {
		return nil, e
	}
	return
}

func (aCtx *Ctx) LookupTaskRequest(ctx context.Context, params LookupTaskRequestParams) (result *LookupTaskRequestResults, e error) {
	result = &LookupTaskRequestResults{
		TaskRequests: make([]CompactTaskRequest, 0),
	}
	l := logger.GetActivityLogger(LookupTaskRequestName, ctx, params)
	// get the CollectionApi
	taskCollection := aCtx.App.Mongodb.GetCollection(types.TaskCollection)
	instanceCollection := aCtx.App.Mongodb.GetCollection(types.InstanceCollection)
	promptsCollection := aCtx.App.Mongodb.GetCollection(types.PromptsCollection)

	tasksCtx, cancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFn()
	// get tasks for the retrieved node and service that are not done yet
	tasks, taskErr := GetRecords[types.Task](tasksCtx, taskCollection, bson.M{"node": params.Node, "service": params.Service, "done": false}, nil)
	if taskErr != nil {
		l.Error("Failed to lookup tasks", "error", taskErr)
		e = temporal.NewApplicationErrorWithCause("unable to find tasks on database", "Database", taskErr)
		return
	}

	if len(tasks) == 0 {
		// nothing to do
		return
	}

	tasksIds := make([]primitive.ObjectID, len(tasks))
	for i, _ := range tasks {
		tasksIds = append(tasksIds, tasks[i].Id)
	}

	instanceCtx, cancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFn()

	// get instances of the read tasks that are not done yet
	instancesFilter := bson.M{"task_id": bson.M{"$in": tasksIds}, "done": false}
	instances, instanceErr := GetRecords[types.Instance](instanceCtx, instanceCollection, instancesFilter, nil)
	if instanceErr != nil {
		l.Error("Failed to lookup instances", "error", instanceErr, "filter", instancesFilter)
		e = temporal.NewApplicationErrorWithCause("unable to find instances on database", "Database", instanceErr)
		return
	}

	if len(instances) == 0 {
		l.Error("0 documents read from instances when looking for existent Task Ids", "filter", instancesFilter)
		e = temporal.NewApplicationError("0 documents read from instances when looking for existent Task Ids", "Database", instancesFilter)
		return
	}

	tasksInstances := make([]promptFilter, len(instances))
	for i, _ := range instances {
		tasksInstances = append(tasksInstances, promptFilter{
			TaskId:     instances[i].TaskId,
			InstanceId: instances[i].TaskId,
		})
	}

	promptsCtx, cancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFn()

	promptsOrFilter := make(bson.A, 0)
	promptsFilter := bson.M{"$or": promptsOrFilter}
	for i, _ := range tasksInstances {
		promptsOrFilter = append(promptsOrFilter, bson.M{
			"task_id":     tasksInstances[i].TaskId,
			"instance_id": tasksInstances[i].TaskId,
			"done":        true,
		})
	}
	prompts, promptsErr := GetRecords[types.Prompt](promptsCtx, promptsCollection, promptsFilter, nil)
	if promptsErr != nil {
		l.Error("Failed to lookup prompts", "error", promptsErr, "filter", promptsFilter)
		e = temporal.NewApplicationErrorWithCause("unable to find prompts on database", "Database", promptsErr)
		return
	}

	if len(prompts) == 0 {
		l.Error("0 documents read from prompts when looking for existent Task Ids and Instance Id", "filter", promptsFilter)
		e = temporal.NewApplicationError("0 documents read from prompts when looking for existent Task Ids and Instance Id", "Database", promptsFilter)
		return
	}

	for i, _ := range prompts {
		prompt := prompts[i]
		result.TaskRequests = append(result.TaskRequests, CompactTaskRequest{
			TaskId:     prompt.TaskId.String(),
			InstanceId: prompt.InstanceId.String(),
			PromptId:   prompt.Id.String(),
		})
	}

	return
}
