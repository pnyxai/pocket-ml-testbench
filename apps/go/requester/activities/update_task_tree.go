package activities

import (
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.temporal.io/sdk/temporal"
	"requester/common"
	"requester/types"
)

type UpdateTaskTreeRequest struct {
	PromptId string `json:"prompt_id"`
}

type UpdateTaskTreeResponse struct {
	TaskId string `json:"task_id"`
	IsDone bool   `json:"is_done"`
}

var UpdateTaskTreeName = "update_task_tree"

func UpdateTaskTreeSessionWrapper(aCtx *Ctx, params *UpdateTaskTreeRequest) func(ctx mongo.SessionContext) (interface{}, error) {
	return func(ctx mongo.SessionContext) (response interface{}, err error) {
		result := UpdateTaskTreeResponse{}
		response = &result
		// lookup for the prompt
		promptCollection := aCtx.App.Mongodb.GetCollection(types.PromptsCollection)
		promptIdFilter := bson.M{"_id": params.PromptId}
		prompt, promptErr := common.GetRecord[types.Prompt](ctx, promptCollection, promptIdFilter)
		if promptErr != nil {
			err = temporal.NewApplicationErrorWithCause("error finding prompt", "DatabaseFindOneError", promptErr)
			return
		}
		// assign task id
		result.TaskId = prompt.TaskId.Hex()
		// update prompt as done
		prompt.Done = true
		_, promptUpdateErr := promptCollection.UpdateOne(ctx, promptIdFilter, bson.M{"$set": prompt})
		if promptUpdateErr != nil {
			err = temporal.NewApplicationErrorWithCause("error updating prompt", "DatabaseUpdateError", promptUpdateErr)
			return
		}

		// lookup for all the prompts of the instance
		promptsFilter := bson.M{"instance_id": prompt.InstanceId}
		prompts, promptsErr := common.GetRecords[types.Prompt](ctx, promptCollection, promptsFilter)
		if promptsErr != nil {
			err = temporal.NewApplicationErrorWithCause("error finding prompts by task id", "DatabaseFindError", promptsErr, prompt.TaskId.Hex())
			return
		}

		// check if all are done or not
		allPromptsOfInstanceAreDone := true
		for i := range prompts {
			if !prompts[i].Done {
				allPromptsOfInstanceAreDone = false
				break
			}
		}

		// A: if at least one prompt is not mark yet, return false, nil
		if !allPromptsOfInstanceAreDone {
			result.IsDone = allPromptsOfInstanceAreDone
			return
		}

		// B: if all the prompts are done, mark the instance as done and move to evaluate all instances of the task
		instanceCollection := aCtx.App.Mongodb.GetCollection(types.InstanceCollection)

		// lookup the instance of this prompt
		instanceFilter := bson.M{"_id": prompt.InstanceId}
		instance, instanceErr := common.GetRecord[types.Instance](ctx, instanceCollection, instanceFilter)
		if instanceErr != nil {
			err = temporal.NewApplicationErrorWithCause("error finding instance", "DatabaseFindOneError", instanceErr)
			return
		}
		// update instance as done
		instance.Done = true
		_, instanceUpdateErr := instanceCollection.UpdateOne(ctx, instanceFilter, bson.M{"$set": instance})
		if instanceUpdateErr != nil {
			err = temporal.NewApplicationErrorWithCause("error updating instance", "DatabaseUpdateError", instanceUpdateErr)
			return
		}

		// lookup for all the instance of the task of this prompt
		instancesFilter := bson.M{"task_id": prompt.TaskId}
		instances, instancesErr := common.GetRecords[types.Instance](ctx, instanceCollection, instancesFilter)
		if instancesErr != nil {
			err = temporal.NewApplicationErrorWithCause("error finding instances by task id", "DatabaseFindError", instancesErr, prompt.TaskId.Hex())
			return
		}

		// check if all the instances are done
		allInstancesOfTaskAreDone := true
		for i := range instances {
			if !instances[i].Done {
				allInstancesOfTaskAreDone = false
				break
			}
		}

		// A: if at least one instance is not mark yet, return false, nil
		if !allInstancesOfTaskAreDone {
			result.IsDone = allInstancesOfTaskAreDone
			return
		}

		// B: if all the instances are done, lookup for the task and mark as done
		taskCollection := aCtx.App.Mongodb.GetCollection(types.TaskCollection)
		taskIdFilter := bson.M{"_id": prompt.TaskId}
		task, taskErr := common.GetRecord[types.Task](ctx, taskCollection, taskIdFilter)
		if taskErr != nil {
			err = temporal.NewApplicationErrorWithCause("error finding task", "DatabaseFindOneError", taskErr)
			return
		}

		task.Done = true
		_, taskUpdateErr := taskCollection.UpdateOne(ctx, taskIdFilter, bson.M{"$set": task})
		if taskUpdateErr != nil {
			err = temporal.NewApplicationErrorWithCause("error updating task", "DatabaseUpdateError", taskUpdateErr)
			return
		}

		result.IsDone = task.Done
		return
	}
}

func (aCtx *Ctx) UpdateTaskTree(ctx context.Context, params UpdateTaskTreeRequest) (response *UpdateTaskTreeResponse, err error) {
	response = &UpdateTaskTreeResponse{}
	session, sessionErr := aCtx.App.Mongodb.StartSession()

	if sessionErr != nil {
		err = temporal.NewApplicationErrorWithCause("error starting a database session", "DatabaseSessionError", sessionErr)
		return
	}

	result, transactionErr := session.WithTransaction(ctx, UpdateTaskTreeSessionWrapper(aCtx, &params))

	if transactionErr != nil {
		err = temporal.NewApplicationErrorWithCause("error creating database transaction context", "DatabaseSessionTransactionError", transactionErr)
		return
	}

	if result == nil {
		err = temporal.NewApplicationError("transaction does not return error and neither a result", "DatabaseSessionTransactionError")
		return
	}

	if v, ok := result.(*UpdateTaskTreeResponse); !ok {
		err = temporal.NewApplicationError("transaction does not return a boolean type as expected", "DatabaseSessionTransactionError")
		return
	} else {
		response = v
	}

	return
}
