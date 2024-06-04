package records

import (
	"context"
	"packages/mongodb"
	"time"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ------------------------------------------------------------------------------
// BaseResultRecord
// ------------------------------------------------------------------------------

// This is the basic information that all results should have
type BaseResultRecord struct {
	Address   string `bson:"address"`
	Service   string `bson:"service"`
	Height    uint32 `bson:"height"`
	Framework string `bson:"framework"`
	Task      string `bson:"task"`
	Status    uint32 `bson:"status"`
}

// ------------------------------------------------------------------------------
// ResultInterface all results structs will respond to this, for ease of processing
// ------------------------------------------------------------------------------

type ResultInterface interface {
	GetNumSamples() uint32
	GetStatus() uint32
	GetSample(int) interface{}
	FindAndLoadResults(address string,
		service string,
		framework string,
		task string,
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
	NumSamples    uint32           `bson:"num_samples"`
	ScoresSamples []ScoresSample   `bson:"scores"`
}

func (record *NumericalResultRecord) GetStatus() uint32 {
	return record.ResultData.Status
}

func (record *NumericalResultRecord) GetNumSamples() uint32 {
	return record.NumSamples
}
func (record *NumericalResultRecord) GetSample(index int) interface{} {
	return record.ScoresSamples[index]
}

func (record *NumericalResultRecord) FindAndLoadResults(address string,
	service string,
	framework string,
	task string,
	collection mongodb.CollectionAPI,
	l *zerolog.Logger) (bool, error) {

	// Set filtering for this result
	result_filter := bson.D{{Key: "result_data.address", Value: address},
		{Key: "result_data.service", Value: service},
		{Key: "result_data.framework", Value: framework},
		{Key: "result_data.task", Value: task},
	}
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Retrieve this node entry
	var found bool = true
	cursor := collection.FindOne(ctxM, result_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Debug().Str("address", address).Str("service", service).Msg("Node results entry not found.")
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
// The NumericalResultRecord field indicates how many samples were actually calculated
type SignatureResultRecord struct {
	ResultData    BaseResultRecord  `bson:"result_data"`
	NumSamples    uint32            `bson:"num_samples"`
	ScoresSamples []SignatureSample `bson:"signatures"`
}

func (record *SignatureResultRecord) GetStatus() uint32 {
	return record.ResultData.Status
}

func (record *SignatureResultRecord) GetNumSamples() uint32 {
	return record.NumSamples
}
func (record *SignatureResultRecord) GetSample(index int) interface{} {
	return record.ScoresSamples[index]
}

func (record *SignatureResultRecord) FindAndLoadResults(address string,
	service string,
	framework string,
	task string,
	collection mongodb.CollectionAPI,
	l *zerolog.Logger) (bool, error) {

	// Set filtering for this result
	result_filter := bson.D{{Key: "result_data.address", Value: address},
		{Key: "result_data.service", Value: service},
		{Key: "result_data.framework", Value: framework},
		{Key: "result_data.task", Value: task},
	}
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Retrieve this node entry
	var found bool = true
	cursor := collection.FindOne(ctxM, result_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Debug().Str("address", address).Str("service", service).Msg("Node results entry not found.")
			found = false
		} else {
			l.Error().Msg("Could not retrieve results data from MongoDB.")
			return false, err
		}
	}

	return found, nil
}
