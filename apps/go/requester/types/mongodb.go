package types

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	TaskCollection     = "tasks"
	InstanceCollection = "instances"
	PromptsCollection  = "prompts"
	ResponseCollection = "responses"
)

type RequesterArgs struct {
	Address string `json:"address"`
	Service string `json:"service"`
	Method  string `json:"method"`
	Path    string `json:"path"`
}

type Task struct {
	Id            primitive.ObjectID `bson:"_id"`
	RequesterArgs `bson:"requester_args"`
	Done          bool `bson:"done"`
}

type Instance struct {
	Id     primitive.ObjectID `bson:"_id"`
	TaskId primitive.ObjectID `bson:"task_id"`
	Done   bool               `bson:"done"`
}

type Prompt struct {
	Id         primitive.ObjectID `bson:"_id"`
	TaskId     primitive.ObjectID `bson:"task_id"`
	InstanceId primitive.ObjectID `bson:"instance_id"`
	Done       bool               `bson:"done"`
}

type Response struct {
	Id primitive.ObjectID `bson:"_id"`
	// cross references
	TaskId     primitive.ObjectID `bson:"task_id"`
	InstanceId primitive.ObjectID `bson:"instance_id"`
	PromptId   primitive.ObjectID `bson:"prompt_id"`
}
