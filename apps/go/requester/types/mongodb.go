package types

import (
	"context"
	"packages/mongodb"
	"time"

	"github.com/pokt-foundation/pocket-go/provider"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	TaskCollection            = "tasks"
	InstanceCollection        = "instances"
	PromptsCollection         = "prompts"
	ResponseCollection        = "responses"
	RelaysBySessionCollection = "relays_by_session"
)

type RequesterArgs struct {
	Address string                `json:"address"`
	Service string                `json:"service"`
	Method  string                `json:"method"`
	Path    string                `json:"path"`
	Headers provider.RelayHeaders `json:"headers"`
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
	Task   *Task              `bson:"task,omitempty"`
}

type Prompt struct {
	Id      primitive.ObjectID `bson:"_id"`
	Data    string             `bson:"data"`
	Timeout int64              `bson:"timeout"`
	Done    bool               `bson:"done"`
	// -- Relations Below --
	// id and/or entity if load with a $lookup
	TaskId primitive.ObjectID `bson:"task_id"`
	Task   *Task              `bson:"task,omitempty"`
	// id and/or entity if load with a $lookup
	InstanceId primitive.ObjectID `bson:"instance_id"`
	Instance   *Instance          `bson:"instance,omitempty"`
}

func (p *Prompt) GetTimeoutDuration() time.Duration {
	if p.Timeout == 0 {
		return time.Duration(120) * time.Second
	}
	return time.Duration(p.Timeout) * time.Second
}

type RelayResponse struct {
	Id            primitive.ObjectID `bson:"_id"`
	Ok            bool               `bson:"ok"`
	Code          int                `bson:"error_code"`
	Ms            int64              `bson:"ms"`
	Response      string             `bson:"response"`
	Error         string             `bson:"error"`
	Height        int64              `bson:"height"`
	SessionHeight int64              `bson:"session_height"`
	// cross references
	TaskId     primitive.ObjectID `bson:"task_id"`
	InstanceId primitive.ObjectID `bson:"instance_id"`
	PromptId   primitive.ObjectID `bson:"prompt_id"`
}

func (r *RelayResponse) SetError(code int, e error) {
	r.Ok = false
	r.Code = code
	r.Error = e.Error()
}

func (r *RelayResponse) Save(ctx context.Context, collection mongodb.CollectionAPI) (err error) {
	opts := options.Update().SetUpsert(true)
	filter := bson.M{"_id": r.Id}
	update := bson.M{"$set": r}
	_, err = collection.UpdateOne(ctx, filter, update, opts)
	return
}

type RelaysBySession struct {
	Servicer      string `bson:"servicer"`
	Application   string `bson:"application"`
	Service       string `bson:"service"`
	SessionHeight int64  `bson:"session_height"`
	IsError       bool   `bson:"-"`
	Ms            int64  `bson:"-"`
}

func (r *RelaysBySession) IncreaseRelay(ctx context.Context, collection mongodb.CollectionAPI) (err error) {
	opts := options.Update().SetUpsert(true)
	filter := bson.M{
		"servicer":       r.Servicer,
		"application":    r.Application,
		"service":        r.Service,
		"session_height": r.SessionHeight,
	}
	errorIncrement := 0
	if r.IsError {
		errorIncrement++
	}
	update := bson.M{"$set": r, "$inc": bson.M{"relays": 1, "errors": errorIncrement, "time": r.Ms}}
	_, err = collection.UpdateOne(ctx, filter, update, opts)
	return
}
