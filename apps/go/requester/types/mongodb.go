package types

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
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
	Id   primitive.ObjectID `bson:"_id"`
	Done bool               `bson:"done"`
	// -- Relations Below --
	// id and/or entity if load with a $lookup
	TaskId primitive.ObjectID `bson:"task_id"`
	Task   *Task              `bson:"task"`
}

type Prompt struct {
	Id      primitive.ObjectID `bson:"_id"`
	Data    string             `bson:"data"`
	Timeout int64              `bson:"timeout"`
	Done    bool               `bson:"done"`
	// -- Relations Below --
	// id and/or entity if load with a $lookup
	TaskId primitive.ObjectID `bson:"task_id"`
	Task   *Task              `bson:"task"`
	// id and/or entity if load with a $lookup
	InstanceId primitive.ObjectID `bson:"instance_id"`
	Instance   *Instance          `bson:"instance"`
}

func (p *Prompt) GetTimeoutDuration() time.Duration {
	if p.Timeout == 0 {
		return time.Duration(120000) * time.Millisecond
	}
	return time.Duration(p.Timeout) * time.Millisecond
}

type Response struct {
	Id       primitive.ObjectID `bson:"_id"`
	Response string             `bson:"response"`
	Ok       bool               `bson:"ok"`
	Code     int                `bson:"error_code"`
	Ms       int64              `bson:"ms"`
	Error    string             `bson:"error"`
	// cross references
	TaskId     primitive.ObjectID `bson:"task_id"`
	InstanceId primitive.ObjectID `bson:"instance_id"`
	PromptId   primitive.ObjectID `bson:"prompt_id"`
}

func (r *Response) SetError(code int, e error) {
	r.Ok = false
	r.Code = code
	r.Error = e.Error()
}
