package types

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var (
	TaskCollection              = "tasks"
	InstanceCollection          = "instances"
	PromptsCollection           = "prompts"
	ResponsesCollection         = "responses"
	SuppliersCollection         = "suppliers"
	ResultsCollection           = "results"
	NumericalTaskCollection     = "buffers_numerical"
	SignaturesTaskCollection    = "buffers_signatures"
	TaxonomySummariesCollection = "taxonomy_summaries"
	TackedTaskSamplesCollection = "tracked_task_samples"
)

type RelayResponse struct {
	Id       primitive.ObjectID `bson:"_id"`
	Ok       bool               `bson:"ok"`
	Code     int                `bson:"error_code"`
	Ms       int64              `bson:"ms"`
	Response string             `bson:"response"`
	Error    string             `bson:"error"`
	// cross references
	TaskId     primitive.ObjectID `bson:"task_id"`
	InstanceId primitive.ObjectID `bson:"instance_id"`
	PromptId   primitive.ObjectID `bson:"prompt_id"`
}

type Instance struct {
	Id       primitive.ObjectID `bson:"_id"`
	TaskName string             `bson:"task_name"`
	DocID    int64              `bson:"doc_id"`
	Done     bool               `bson:"done"`
	// -- Relations Below --
	// id and/or entity if load with a $lookup
	TaskId primitive.ObjectID `bson:"task_id"`
}

type Prompt struct {
	Id   primitive.ObjectID `bson:"_id"`
	Data string             `bson:"data"`
	// -- Relations Below --
	TaskId     primitive.ObjectID `bson:"task_id"`
	InstanceId primitive.ObjectID `bson:"instance_id"`
}

type TrackedTaskSample struct {
	SupplierAddress string    `bson:"supplier_address"`
	DocID           int64     `bson:"doc_id"`
	TaskName        string    `bson:"task_name"`
	Prompt          string    `bson:"prompt"`
	Response        string    `bson:"response"`
	ResponseMs      int64     `bson:"response_ms"`
	Score           float64   `bson:"score"`
	SampleDate      time.Time `bson:"sample_date"`
}
