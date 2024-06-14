package records

import (
	"context"
	"errors"
	"fmt"
	"manager/types"
	"packages/mongodb"
	"time"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//------------------------------------------------------------------------------
// NodeRecord
//------------------------------------------------------------------------------

// DB entry of a given node-service pair
// The "Tasks" array will hold as many entries as tasks being tested
type NodeRecord struct {
	Address        string                `bson:"address"`
	Service        string                `bson:"service"`
	LastSeenHeight uint32                `bson:"last_seen_height"`
	LastSeenTime   time.Time             `bson:"last_seen_time"`
	NumericalTasks []NumericalTaskRecord `bson:"numerical_tasks"`
	SignatureTasks []SignatureTaskRecord `bson:"signature_tasks"`
}

// Creates and array of interfaces that contains all tasks
func (record *NodeRecord) CombineTasks() []TaskInterface {
	combinedTasks := make([]TaskInterface, 0, len(record.NumericalTasks)+len(record.SignatureTasks))

	for _, element := range record.NumericalTasks {
		combinedTasks = append(combinedTasks, &element)
	}

	for _, element := range record.SignatureTasks {
		combinedTasks = append(combinedTasks, &element)
	}

	return combinedTasks
}

func (record *NodeRecord) GetPrunedTasks(taskArray []TaskInterface, maxAge time.Duration, l *zerolog.Logger) []TaskInterface {
	// Indices to remove
	var indicesToRemove []int
	// For each task
	for i, task := range taskArray {
		// Check the date of the index end
		oldestAge := time.Since(task.GetLastSeen())
		// Check
		if oldestAge >= maxAge {
			// Add to remove list
			indicesToRemove = append(indicesToRemove, i)

			l.Info().Str("address", record.Address).Str("service", record.Service).Str("framework", task.GetFramework()).Str("task", task.GetTask()).Msg("Removing task due to old age.")
		}
	}

	// Remove Tasks
	for i := len(indicesToRemove) - 1; i >= 0; i-- {
		index := indicesToRemove[i]
		taskArray = append(taskArray[:index], taskArray[index+1:]...)
	}

	return taskArray
}

// Go through all task and remove the ones that have no new samples since the limit
func (record *NodeRecord) PruneTasks(l *zerolog.Logger) error {

	// Maximum age of a task
	maxAge := time.Duration(TaskTTLDays) * 24 * time.Hour

	// Remove Numerical Tasks
	var numTaskInterfaces []TaskInterface
	for _, task := range record.NumericalTasks {
		numTaskInterfaces = append(numTaskInterfaces, &task)
	}
	tasksPrunned := record.GetPrunedTasks(numTaskInterfaces, maxAge, l)
	for i, task := range tasksPrunned {
		record.NumericalTasks[i] = *(task.(*NumericalTaskRecord))
	}

	// Remove Signature Tasks
	var signTaskInterfaces []TaskInterface
	for _, task := range record.SignatureTasks {
		signTaskInterfaces = append(signTaskInterfaces, &task)
	}
	tasksPrunned = record.GetPrunedTasks(signTaskInterfaces, maxAge, l)
	for i, task := range tasksPrunned {
		record.SignatureTasks[i] = *(task.(*SignatureTaskRecord))
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
			// }else if err == mongo. {
		} else {
			l.Error().Msg("Could not retrieve node data from MongoDB.")
			fmt.Print(err)
			return false, err
		}
	}

	return found, nil
}

func (record *NodeRecord) AppendTask(framework string, task string, date time.Time, frameworkConfigMap map[string]types.FrameworkConfig, l *zerolog.Logger) TaskInterface {

	taskType, err := GetTaskType(framework, task, frameworkConfigMap, l)
	if err != nil {
		return nil
	}

	// Fill base task data
	baseTaskData := BaseTaskRecord{
		Framework: framework,
		Task:      task,
		LastSeen:  date,
	}

	if taskType == NumericalTaskTypeName {
		// TODO: Get default values from framework-task
		bufferLen := NumericalCircularBufferLength
		timeArray := make([]time.Time, bufferLen)
		for i := range timeArray {
			timeArray[i] = date
		}

		newTask := NumericalTaskRecord{
			TaskData:      baseTaskData,
			MeanScore:     0.0,
			StdScore:      0.0,
			ScoresSamples: make([]ScoresSample, bufferLen),
			CircBuffer: types.CircularBuffer{
				CircBufferLen: bufferLen,
				NumSamples:    0,
				Times:         timeArray,
				Indexes: types.CircularIndexes{
					Start: 0,
					End:   0,
				},
			},
		}

		record.NumericalTasks = append(record.NumericalTasks, newTask)

		return &newTask

	} else if taskType == SignatureTaskTypeName {

		// TODO: Get default values from framework-task
		bufferLen := SignatureCircularBufferLength
		timeArray := make([]time.Time, bufferLen)
		for i := range timeArray {
			timeArray[i] = date
		}

		newTask := SignatureTaskRecord{
			TaskData:      baseTaskData,
			LastSignature: "",
			Signatures:    make([]SignatureSample, bufferLen),
			CircBuffer: types.CircularBuffer{
				CircBufferLen: bufferLen,
				NumSamples:    0,
				Times:         timeArray,
				Indexes: types.CircularIndexes{
					Start: 0,
					End:   0,
				},
			},
		}

		record.SignatureTasks = append(record.SignatureTasks, newTask)

		return &newTask
	}

	return nil

}

func (record *NodeRecord) Init(params types.AnalyzeNodeParams, frameworkConfigMap map[string]types.FrameworkConfig, l *zerolog.Logger) error {
	// Initialize empty record

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
			_ = record.AppendTask(test.Framework, task, time.Now(), frameworkConfigMap, l)
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
