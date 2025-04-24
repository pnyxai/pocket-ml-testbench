package records

import (
	"context"
	"packages/mongodb"
	"time"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ------------------------------------------------------------------------------
// BaseResultRecord
// ------------------------------------------------------------------------------

// This is the basic information that all results should have
type BaseResultRecord struct {
	TaskID       string    `bson:"task_id"`
	NumSamples   uint32    `bson:"num_samples"`
	Status       uint32    `bson:"status"`
	ResultHeight int64     `bson:"result_height"`
	ResultTime   time.Time `bson:"result_time"`
}

func (record *BaseResultRecord) GetResultTime() time.Time {
	return record.ResultTime
}

func (record *BaseResultRecord) GetResultHeight() int64 {
	return record.ResultHeight
}

type RelayResponseCodesEnum struct {
	Ok             int
	Relay          int
	Supplier       int
	OutOfSession   int
	BadParams      int
	PromptNotFound int
	DatabaseRead   int
	PocketRpc      int
	SignerNotFound int
	SignerError    int
	AATSignature   int
	Evaluation     int
}

var RelayResponseCodes = RelayResponseCodesEnum{
	Ok:             0,
	Relay:          1,
	Supplier:       2,
	OutOfSession:   3,
	BadParams:      4,
	PromptNotFound: 5,
	DatabaseRead:   6,
	PocketRpc:      7,
	SignerNotFound: 8,
	SignerError:    9,
	AATSignature:   10,
	Evaluation:     11,
}

// ------------------------------------------------------------------------------
// ResultInterface all results structs will respond to this, for ease of processing
// ------------------------------------------------------------------------------

type ResultInterface interface {
	GetResultHeight() int64
	GetResultTime() time.Time
	GetNumSamples() uint32
	GetStatus() uint32
	GetSample(int) interface{}
	FindAndLoadResults(taskID primitive.ObjectID,
		collection mongodb.CollectionAPI,
		l *zerolog.Logger) (bool, error)
}

//------------------------------------------------------------------------------
// NumericalResultRecord
//------------------------------------------------------------------------------

// Record written by the evaluator.
// The NumericalResultRecord field indicates how many samples were actually calculated
type NumericalResultRecord struct {
	ResultData    BaseResultRecord `bson:"result_data"`
	ScoresSamples []ScoresSample   `bson:"scores"`
}

func (record *NumericalResultRecord) GetResultTime() time.Time {
	return record.ResultData.GetResultTime()
}

func (record *NumericalResultRecord) GetResultHeight() int64 {
	return record.ResultData.GetResultHeight()
}

func (record *NumericalResultRecord) GetNumSamples() uint32 {
	return record.ResultData.NumSamples
}

func (record *NumericalResultRecord) GetStatus() uint32 {
	return record.ResultData.Status
}

func (record *NumericalResultRecord) GetSample(index int) interface{} {
	return record.ScoresSamples[index]
}

func (record *NumericalResultRecord) FindAndLoadResults(taskID primitive.ObjectID,
	collection mongodb.CollectionAPI,
	l *zerolog.Logger) (bool, error) {

	// Set filtering for this result
	result_filter := bson.D{{Key: "result_data.task_id", Value: taskID}}
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Retrieve this supplier entry
	var found bool = true
	cursor := collection.FindOne(ctxM, result_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Debug().Str("task_id", taskID.String()).Msg("Results entry not found for given TaskID.")
			found = false
		} else {
			l.Error().Msg("Could not retrieve results data from MongoDB.")
			return false, err
		}
	}

	return found, nil
}

//------------------------------------------------------------------------------
// NumericalResultRecord
//------------------------------------------------------------------------------

// Record written by the evaluator.
// The SignatureResultRecord field indicates how many samples were actually calculated
type SignatureResultRecord struct {
	ResultData    BaseResultRecord  `bson:"result_data"`
	ScoresSamples []SignatureSample `bson:"signatures"`
}

func (record *SignatureResultRecord) GetResultTime() time.Time {
	return record.ResultData.GetResultTime()
}

func (record *SignatureResultRecord) GetResultHeight() int64 {
	return record.ResultData.GetResultHeight()
}

func (record *SignatureResultRecord) GetNumSamples() uint32 {
	return record.ResultData.NumSamples
}

func (record *SignatureResultRecord) GetStatus() uint32 {
	return record.ResultData.Status
}

func (record *SignatureResultRecord) GetSample(index int) interface{} {
	return record.ScoresSamples[index]
}

func (record *SignatureResultRecord) FindAndLoadResults(taskID primitive.ObjectID,
	collection mongodb.CollectionAPI,
	l *zerolog.Logger) (bool, error) {

	// Set filtering for this result
	result_filter := bson.D{{Key: "result_data.task_id", Value: taskID}}

	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Retrieve this supplier entry
	var found bool = true
	cursor := collection.FindOne(ctxM, result_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Debug().Str("task_id", taskID.String()).Msg("Results entry not found for given TaskID.")
			found = false
		} else {
			l.Error().Msg("Could not retrieve results data from MongoDB.")
			return false, err
		}
	}
	if found {
		l.Debug().Str("task_id", taskID.String()).Msg("Results retrieved")
	}

	return found, nil
}
