package activities

import (
	"context"
	"errors"
	"manager/types"
	"packages/mongodb"
	"sort"
	"time"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gonum.org/v1/gonum/stat"
)

// The maximum age of a sample living in a buffer.
const sampleTTLDays uint32 = 5

// The maximum age of a task entry.
const taskTTLDays uint32 = 32

// Minimum number of samples to have in a task to consider that it does not require more samples
// According to "tinyBenchmarks: evaluating LLMs with fewer examples" 100 is enough, but also 50 seems adequate.
const MinSamplesPerTask uint32 = 50

// Maximum size of result buffer and also maximum number of samples to ask per task
const MaxConcurrentSamplesPerTask uint32 = 10

// This is the length of the buffer and will set the maximum accuracy of the metric.
const circularBufferLength uint32 = MinSamplesPerTask

// Keep track of circular buffer start and end indexes
type CircularIndexes struct {
	Start uint32 `bson:"cir_start"`
	End   uint32 `bson:"cir_end"`
}

func bufferLimitCheck(nextVal int, limitValue uint32) uint32 {
	// Check for overflow
	if nextVal >= int(circularBufferLength) {
		nextVal = 0
	} else if nextVal <= 0 {
		// Check for underflow
		nextVal = int(circularBufferLength - 1)
	}

	// Check for limit
	if nextVal >= int(limitValue) {
		nextVal = int(limitValue)
	}

	return uint32(nextVal)
}

// ------------------------------------------------------------------------------
// TaskRecord
// ------------------------------------------------------------------------------

// Create base task, rename this one to NumericalTaskRecord and create also one that works with signatures and tokenizers

// All information for a given task
// Each task will have its own buffer and will be updated independently from others
type TaskRecord struct {
	Framework     string                          `bson:"framework"`
	Task          string                          `bson:"task"`
	MeanScore     float32                         `bson:"mean_scores"`
	MedianScore   float32                         `bson:"median_scores"`
	StdScore      float32                         `bson:"std_scores"`
	NumSamples    uint32                          `bson:"num_samples"`
	ScoresSamples [circularBufferLength]float32   `bson:"scores"`
	Times         [circularBufferLength]time.Time `bson:"times"`
	Ids           [circularBufferLength]int       `bson:"ids"`
	Indexes       CircularIndexes                 `bson:"indexes"`
}

// Calculate task statistics
func (record *TaskRecord) CalculateStats(l *zerolog.Logger) (err error) {

	// Slice the buffer and cast
	var auxData []float64
	idxNow := record.Indexes.Start
	for true {
		// run until we complete the circular buffer
		if idxNow == record.Indexes.End {
			break
		}
		// Add sample to data array
		auxData = append(auxData, float64(record.ScoresSamples[idxNow]))
		// perform the step
		nextVal := int(idxNow) + 1
		// Check limits and assign value
		idxNow = bufferLimitCheck(nextVal, record.Indexes.End)
	}
	length := len(auxData)
	if length == 0 {
		record.MeanScore = 0
		record.StdScore = 0
		record.MedianScore = 0
	} else if length == 1 {
		record.MeanScore = record.ScoresSamples[record.Indexes.Start]
		record.StdScore = 0
		record.MedianScore = record.ScoresSamples[record.Indexes.Start]
	} else {
		// Calculate the mean
		record.MeanScore = float32(stat.Mean(auxData, nil))
		// Calculate the standard deviation
		record.StdScore = float32(stat.StdDev(auxData, nil))
		// Calculate the median
		sort.Float64s(auxData)
		if length%2 == 0 {
			record.MedianScore = float32((auxData[length/2-1] + auxData[length/2]) / 2)
		} else {
			record.MedianScore = float32(auxData[length/2])
		}
	}
	return err
}

// Gets the sample index given a step direction (positive: 1 or negative: -1) and for a given marker (start or end of buffer)
func (record *TaskRecord) stepIndex(step int, marker string) error {
	// Get values
	var currValue uint32
	var limitValue uint32
	if marker == "start" {
		currValue = record.Indexes.Start
		limitValue = record.Indexes.End
	} else if marker == "end" {
		currValue = record.Indexes.End
		limitValue = record.Indexes.Start
	} else {
		return errors.New("buffer: invalid marker designation")
	}

	// perform the step
	nextVal := int(currValue) + step

	// Check limits and assign value
	currValue = bufferLimitCheck(nextVal, limitValue)

	// Update values
	if marker == "start" {
		record.Indexes.Start = currValue
	} else {
		record.Indexes.End = currValue
	}
	record.NumSamples = uint32(int(record.NumSamples) + step)

	return nil
}

// Updates the indexes making them point to the initial and final samples in a given time window.
func (record *TaskRecord) CycleIndexes(l *zerolog.Logger) error {

	// Maximum age of a sample
	maxAge := time.Duration(sampleTTLDays) * 24 * time.Hour
	// Check the date of the index start
	oldestAge := time.Since(record.Times[record.Indexes.Start])

	for oldestAge >= maxAge {
		// Increment the start
		err := record.stepIndex(1, "start")
		if err != nil {
			return err
		}
		// Update the date
		oldestAge = time.Since(record.Times[record.Indexes.Start])
		// Break if met the limit
		if record.Indexes.Start == record.Indexes.End {
			l.Info().Str("framework", record.Framework).Str("task", record.Task).Msg("Circular buffer collapsed.")
			break
		}
	}

	return nil
}

func (record *TaskRecord) IsertSample(sample float32, timeSample time.Time, id int) (err error) {
	// Increment the end
	err = record.stepIndex(1, "end")
	// Save sample
	record.ScoresSamples[record.Indexes.End] = sample
	record.Times[record.Indexes.End] = timeSample
	record.Ids[record.Indexes.End] = id

	return nil
}

//------------------------------------------------------------------------------
// NodeRecord
//------------------------------------------------------------------------------

// DB entry of a given node-service pair
// The "Tasks" array will hold as many entries as tasks being tested
type NodeRecord struct {
	Address        string       `bson:"address"`
	Service        string       `bson:"service"`
	LastSeenHeight uint32       `bson:"last_seen_height"`
	LastSeenTime   time.Time    `bson:"last_seen_time"`
	Tasks          []TaskRecord `bson:"tasks"`
	Tokenizer      string       `bson:"tokenizer"` // TODO: Remove this in the future, in favor of a signature task
}

// Go through all task and remove the ones that have no new samples since the limit
func (record *NodeRecord) PruneTasks(l *zerolog.Logger) error {

	// Maximum age of a task
	maxAge := time.Duration(taskTTLDays) * 24 * time.Hour
	// Indices to remove
	var indicesToRemove []int
	// For each task
	for i, task := range record.Tasks {
		// Check the date of the index end
		oldestAge := time.Since(task.Times[task.Indexes.End])
		// Check
		if oldestAge >= maxAge {
			// Add to remove list
			indicesToRemove = append(indicesToRemove, i)

			l.Info().Str("address", record.Address).Str("service", record.Service).Str("framework", task.Framework).Str("task", task.Task).Msg("Removing task due to old age.")
		}
	}
	// Remove elements from the original slice based on indicesToRemove
	for i := len(indicesToRemove) - 1; i >= 0; i-- {
		index := indicesToRemove[i]
		record.Tasks = append(record.Tasks[:index], record.Tasks[index+1:]...)
	}

	return nil
}

func (record *NodeRecord) FindAndLoadNode(node types.NodeData, collection mongodb.CollectionAPI, l *zerolog.Logger) (bool, error) {

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

func (record *NodeRecord) AppendTask(framework string, task string, date time.Time) *TaskRecord {
	var timeArray [circularBufferLength]time.Time
	for i := range timeArray {
		timeArray[i] = date
	}

	newTask := TaskRecord{
		Framework:     framework,
		Task:          task,
		MeanScore:     0.0,
		StdScore:      0.0,
		NumSamples:    0,
		ScoresSamples: [circularBufferLength]float32{},
		Times:         timeArray,
		Indexes: CircularIndexes{
			Start: 0,
			End:   0,
		},
	}

	record.Tasks = append(record.Tasks, newTask)

	return &newTask
}

func (record *NodeRecord) Init(params types.AnalyzeNodeParams, l *zerolog.Logger) error {
	// Initialize empty record

	// TODO: Remove this placeholder
	record.Tokenizer = "83332a7f32e4188bb276a18ff78620acfd3c6edbd68002b746bda990ed30d56c"

	// Set node data
	record.Address = params.Node.Address
	record.Service = params.Node.Service

	// Never tested
	record.LastSeenHeight = 0
	defaultDate := time.Date(2018, 1, 1, 00, 00, 00, 100, time.Local)
	record.LastSeenTime = defaultDate

	// Create all tests
	if len(params.Tests) == 0 {
		return errors.New(`tests array cannot be empty`)
	}
	for _, test := range params.Tests {

		for _, task := range test.Tasks {
			// Add all tasks with the current date as maker for creation
			_ = record.AppendTask(test.Framework, task, time.Now())
		}
	}

	return nil

}

func (record *NodeRecord) UpdateNode(collection mongodb.CollectionAPI, l *zerolog.Logger) (bool, error) {

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

//------------------------------------------------------------------------------
// ResultRecord
//------------------------------------------------------------------------------

// Record written by the evaluator.
// The NumSamples field indicates how many samples were actually calculated
type ResultRecord struct {
	Address    string                               `bson:"address"`
	Service    string                               `bson:"service"`
	Height     uint32                               `bson:"height"`
	Framework  string                               `bson:"framework"`
	Task       string                               `bson:"task"`
	Status     uint32                               `bson:"status"`
	NumSamples uint32                               `bson:"num_samples"`
	Scores     [MaxConcurrentSamplesPerTask]float32 `bson:"scores"`
	SampleIds  [MaxConcurrentSamplesPerTask]int     `bson:"sample_ids"`
}

func (record *ResultRecord) FindAndLoadResults(address string,
	service string,
	framework string,
	task string,
	collection mongodb.CollectionAPI,
	l *zerolog.Logger) (bool, error) {

	// Set filtering for this result
	result_filter := bson.D{{Key: "address", Value: address},
		{Key: "service", Value: service},
		{Key: "framework", Value: framework},
		{Key: "task", Value: task},
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
			l.Warn().Str("address", address).Str("service", service).Msg("Node results entry not found.")
			found = false
		} else {
			l.Error().Msg("Could not retrieve results data from MongoDB.")
			return false, err
		}
	}

	return found, nil
}
