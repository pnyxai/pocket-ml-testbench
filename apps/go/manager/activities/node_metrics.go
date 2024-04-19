package activities

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// This is the length of the buffer and will set the maximum accuracy of the metric.
// According to "tinyBenchmarks: evaluating LLMs with fewer examples" 100 is enough, but also 50 seems adequate.
const CircularBufferLength uint32 = 50

// Keep track of circular buffer start and end indexes
type CircularIndexes struct {
	Start uint32 `bson:"cir_start"`
	End   uint32 `bson:"cir_end"`
}

// All information for a given task
// Each task will have its own buffer and will be updated independently from others
type TaskRecord struct {
	Task          string                          `bson:"task"`
	MeanScore     float32                         `bson:"mean_scores"`
	StdScore      float32                         `bson:"std_scores"`
	NumSamples    uint32                          `bson:"num_samples"`
	ScoresSamples [CircularBufferLength]float32   `bson:"scores"`
	Times         [CircularBufferLength]time.Time `bson:"times"`
	Indexes       CircularIndexes                 `bson:"indexes"`
}

// DB entry of a given node-service pair
// The "Tasks" array will hold as many entries as tasks being tested
type NodeRecord struct {
	Address        string       `bson:"address"`
	Service        string       `bson:"service"`
	LastSeenHeight uint32       `bson:"last_seen_height"`
	LastSeenTime   time.Time    `bson:"last_seen_time"`
	Tasks          []TaskRecord `bson:"tasks"`
}

func (record *NodeRecord) FindAndLoadNode(node NodeData, collection *mongo.Collection, l *zerolog.Logger) (bool, error) {

	// Set filtering for this node-service pair data
	node_filter := bson.D{{Key: "address", Value: node.Address}, {Key: "service", Value: node.Service}}
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Retrieve this node entry
	var found bool = true
	cursor := collection.FindOne(ctxM, node_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("address", node.Address).Str("service", node.Service).Msg("Node entry not found.")
			found = false
		} else {
			l.Error().Msg("Could not retrieve node data from MongoDB.")
			return false, err
		}
	}

	return found, nil
}

func (record *NodeRecord) Init(params AnalyzeNodeParams, l *zerolog.Logger) error {
	// Initialize empty record

	// Set node data
	record.Address = params.Node.Address
	record.Service = params.Node.Service

	// Never tested
	record.LastSeenHeight = 0
	defaultDate := time.Date(2018, 1, 1, 00, 00, 00, 100, time.Local)
	record.LastSeenTime = defaultDate

	// Create all tasks
	if len(params.Tasks) == 0 {
		return errors.New(`task array cannot be empty`)
	}
	for _, task := range params.Tasks {
		var timeArray [CircularBufferLength]time.Time
		for i := range timeArray {
			timeArray[i] = defaultDate
		}

		newTask := TaskRecord{
			Task:          task,
			MeanScore:     0.0,
			StdScore:      0.0,
			NumSamples:    0,
			ScoresSamples: [CircularBufferLength]float32{},
			Times:         timeArray,
			Indexes: CircularIndexes{
				Start: 0,
				End:   0,
			},
		}
		record.Tasks = append(record.Tasks, newTask)
	}

	return nil

}

func (record *NodeRecord) UpdateNode(collection *mongo.Collection, l *zerolog.Logger) (bool, error) {

	opts := options.FindOneAndUpdate().SetUpsert(true)
	node_filter := bson.D{{Key: "address", Value: record.Address}, {Key: "service", Value: record.Service}}
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update given struct
	update := bson.D{{Key: "$set", Value: record}}
	// Get collection and update
	var found bool = true
	err := collection.FindOneAndUpdate(ctxM, node_filter, update, opts).Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("address", record.Address).Str("service", record.Service).Msg("Node entry not found, a new one was created.")
			found = false
		} else {
			l.Error().Msg("Could not retrieve node data from MongoDB.")
			return false, err
		}
	}

	return found, nil
}
