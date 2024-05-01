package activities

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.temporal.io/sdk/temporal"
	"packages/logger"
	"requester/common"
	"requester/types"
	"time"
)

type GetTasksParams struct {
	// Pass a 0 to get the latest
	Node string `json:"node"`
	// chain (morse) service (shannon)
	Service string `json:"service"`
}

type TaskRequest struct {
	TaskId     string `json:"task_id"`
	InstanceId string `json:"instance_id"`
	PromptId   string `json:"prompt_id"`
}

type GetTaskRequestResults struct {
	TaskRequests []TaskRequest `json:"task_requests"`
}

type promptFilter struct {
	TaskId     primitive.ObjectID
	InstanceId primitive.ObjectID
}

var GetTasksName = "get_tasks"

func (aCtx *Ctx) GetTasks(ctx context.Context, params GetTasksParams) (result *GetTaskRequestResults, e error) {
	result = &GetTaskRequestResults{
		TaskRequests: make([]TaskRequest, 0),
	}
	l := logger.GetActivityLogger(GetTasksName, ctx, params)

	tasksCtx, taskCancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer taskCancelFn()
	// get tasks for the retrieved node and service that are not done yet
	taskCollection := aCtx.App.Mongodb.GetCollection(types.TaskCollection)
	taskFilter := bson.M{"node": params.Node, "service": params.Service, "done": false}
	tasks, taskErr := common.GetRecords[types.Task](tasksCtx, taskCollection, taskFilter, nil)
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

	instanceCtx, instanceCancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer instanceCancelFn()

	// get instances of the read tasks that are not done yet
	instanceCollection := aCtx.App.Mongodb.GetCollection(types.InstanceCollection)
	instancesFilter := bson.M{"task_id": bson.M{"$in": tasksIds}, "done": false}
	instances, instanceErr := common.GetRecords[types.Instance](instanceCtx, instanceCollection, instancesFilter, nil)
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

	promptsCtx, promptCancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer promptCancelFn()

	promptsCollection := aCtx.App.Mongodb.GetCollection(types.PromptsCollection)
	promptsOrFilter := make(bson.A, 0)
	promptsFilter := bson.M{"$or": promptsOrFilter}
	for i, _ := range tasksInstances {
		promptsOrFilter = append(promptsOrFilter, bson.M{
			"task_id":     tasksInstances[i].TaskId,
			"instance_id": tasksInstances[i].TaskId,
			"done":        true,
		})
	}
	prompts, promptsErr := common.GetRecords[types.Prompt](promptsCtx, promptsCollection, promptsFilter, nil)
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
		result.TaskRequests = append(result.TaskRequests, TaskRequest{
			TaskId:     prompt.TaskId.Hex(),
			InstanceId: prompt.InstanceId.Hex(),
			PromptId:   prompt.Id.Hex(),
		})
	}

	return
}
