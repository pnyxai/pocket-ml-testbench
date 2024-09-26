package records

import (
	"context"
	"fmt"
	"manager/types"
	"packages/mongodb"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gonum.org/v1/gonum/stat"
)

// ------------------------------------------------------------------------------
// BaseTaskRecord
// ------------------------------------------------------------------------------

// This is the basic information that all tasks should have
type BaseTaskRecord struct {
	NodeID    primitive.ObjectID `bson:"node_id"`
	Framework string             `bson:"framework"`
	Task      string             `bson:"task"`

	LastSeen   time.Time `bson:"last_seen"`
	LastHeight int64     `bson:"last_height"`
}

func (record *BaseTaskRecord) GetNodeID() primitive.ObjectID {
	return record.NodeID
}

func (record *BaseTaskRecord) GetTask() string {
	return record.Task
}

func (record *BaseTaskRecord) GetFramework() string {
	return record.Framework
}

func (record *BaseTaskRecord) GetLastSeen() time.Time {
	return record.LastSeen
}

func (record *BaseTaskRecord) GetLastHeight() int64 {
	return record.LastHeight
}

func (record *BaseTaskRecord) UpdateLastSeen(timeSample time.Time) (err error) {
	record.LastSeen = timeSample
	return nil
}

func (record *BaseTaskRecord) UpdateLastHeight(height int64) (err error) {
	record.LastHeight = height
	return nil
}

// The maximum age of a task entry.
const TaskTTLDays uint32 = 32

// ------------------------------------------------------------------------------
// TaskInterface all task structs will respond to this, for ease of processing
// ------------------------------------------------------------------------------

type TaskInterface interface {
	ProcessData(l *zerolog.Logger) error
	StepIndex(step uint32, marker string, positive_step bool, l *zerolog.Logger) error
	CycleIndexes(l *zerolog.Logger) (bool, error)
	InsertSample(timeSample time.Time, data interface{}, l *zerolog.Logger) (err error)
	GetNumSamples() uint32
	GetFramework() string
	GetTask() string
	GetMinSamplesPerTask() uint32
	GetMaxConcurrentSamplesPerTask() uint32
	GetCircularBufferLength() uint32
	GetSampleTTLDays() uint32
	GetResultStruct() ResultInterface
	GetLastSeen() time.Time
	GetLastHeight() int64
	UpdateLastSeen(timeSample time.Time) (err error)
	UpdateLastHeight(height int64) (err error)
	IsOK() bool
	NewTask(nodeID primitive.ObjectID, framework string, task string, date time.Time, l *zerolog.Logger)
	LoadTask(nodeID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error)
	UpdateTask(nodeID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error)
}

// Get specific task data from a node record
func GetTaskData(nodeID primitive.ObjectID, taskType string, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (TaskInterface, bool) {

	// Look for entry
	if taskType == NumericalTaskTypeName {
		// get task record
		var record NumericalTaskRecord
		found, err := record.LoadTask(nodeID, framework, task, mongoDB, l)
		if err != nil {
			l.Error().Str("nodeID", nodeID.String()).Str("framework", framework).Str("task", task).Msg("cannot find default task buffer")
			return nil, false
		}
		if !found {
			// Initialize and save
			record.NewTask(nodeID, framework, task, types.EpochStart.UTC(), l)
			record.UpdateTask(nodeID, framework, task, mongoDB, l)
		}
		return &record, true
	} else if taskType == SignatureTaskTypeName {
		// set task record
		var record SignatureTaskRecord
		found, err := record.LoadTask(nodeID, framework, task, mongoDB, l)
		if err != nil {
			l.Error().Str("nodeID", nodeID.String()).Str("framework", framework).Str("task", task).Msg("cannot find default task buffer")
			return nil, false
		}
		if !found {
			// Initialize and save
			record.NewTask(nodeID, framework, task, types.EpochStart.UTC(), l)
			record.UpdateTask(nodeID, framework, task, mongoDB, l)
		}
		return &record, true
	}

	return nil, false
}

// Depending on the framework-task pair, the type of data that is saved will vary.
// This functions queries the config to return the actual type of task data to use.
func GetTaskType(framework string, task string, configMap map[string]types.FrameworkConfig, l *zerolog.Logger) (taskType string, err error) {

	// Get Framework config
	frameworkCfg, ok := configMap[framework]
	if !ok {
		l.Error().Str("framework", framework).Msg("framework config not found")
		err = fmt.Errorf("framework config not found")
		return "", err
	}

	// Get task type
	taskType, ok = frameworkCfg.TasksTypes[task]
	if !ok {
		// Search for the "any" field
		taskType, ok = frameworkCfg.TasksTypes["any"]
		if !ok {
			l.Error().Str("framework", framework).Str("task", task).Msg("cannot find default (or specific) value for task type")
			err = fmt.Errorf("cannot find default (or specific) value for task type")
			return "", err
		}
	}

	return taskType, nil
}

// Analyzes the configuration and returns if it is possible to proceed with this task triggering/analysis
// A task can depend on others (such as having a tokenizer signature), here we check for that
func CheckTaskDependency(nodeData *NodeRecord, framework string, task string, configMap map[string]types.FrameworkConfig, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	// Get Framework config
	frameworkCfg, ok := configMap[framework]
	if !ok {
		l.Error().Str("framework", framework).Msg("framework config not found")
		err := fmt.Errorf("framework config not found")
		return false, err
	}

	// Get task dependency
	taskDep, ok := frameworkCfg.TasksDependency[task]
	if !ok {
		// Search for the "any" field
		taskDep, ok = frameworkCfg.TasksDependency["any"]
		if !ok {
			l.Error().Str("framework", framework).Str("task", task).Msg("cannot find default (or specific) value for task type")
			err := fmt.Errorf("cannot find default (or specific) value for task type")
			return false, err
		}
		if len(taskDep) == 0 {
			l.Error().Str("framework", framework).Str("task", task).Msg("malformed dependency array for task type")
			err := fmt.Errorf("malformed dependency array for task type")
			return false, err
		}
	}

	// Check dependency
	depOK := true
	for idxDep := 0; idxDep < len(taskDep); idxDep++ {
		// get data from entry
		frameworkTaskandStatus := strings.Split(taskDep[idxDep], ":")
		if len(frameworkTaskandStatus) != 3 {
			l.Error().Str("framework", framework).Str("task", task).Msg("malformed dependency configuration, expected three elements separated by \":\" ")
			depOK = false
			break
		}
		if frameworkTaskandStatus[0] == "none" {
			// No dependencies
			l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", framework).Str("task", task).Msg("No dependency: Dependecy OK")
			continue
		}
		taskType, err := GetTaskType(frameworkTaskandStatus[0], frameworkTaskandStatus[1], configMap, l)
		if err != nil {
			l.Error().Str("framework", framework).Str("task", task).Str("task type", taskType).Msg("Error getting task type")
			return false, err
		}
		thisTaskRecord, found := GetTaskData(nodeData.ID, taskType, frameworkTaskandStatus[0], frameworkTaskandStatus[1], mongoDB, l)
		if !found {
			// The task is not even created, we must fail
			depOK = false
			break
		} else {
			// Check the condition
			if frameworkTaskandStatus[2] == "present" {
				// Task is present, so OK
				l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", framework).Str("task", task).Msg("Present: Dependecy OK")
				continue
			} else if frameworkTaskandStatus[2] == "ok" {
				// Check for it having a correct value
				if thisTaskRecord.IsOK() {
					l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", framework).Str("task", task).Msg("OK: Dependecy OK")
					continue
				} else {
					l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", framework).Str("task", task).Msg("OK: Dependecy NOT OK")
					depOK = false
					break
				}
			} else {
				l.Error().Str("framework", framework).Str("task", task).Msg("dependency configuration cannot be processed (status type unknown)")
				depOK = false
				break
			}
		}
	}

	return depOK, nil
}

// Analyzes the configuration and checks whether the triggering the task will
// break the schedule limits or not (i.e. trigger twice in the same session)
func CheckTaskSchedule(taskData TaskInterface, block types.BlockData, configMap map[string]types.FrameworkConfig, l *zerolog.Logger) (bool, error) {

	framework := taskData.GetFramework()
	task := taskData.GetTask()

	// Get Framework config
	frameworkCfg, ok := configMap[framework]
	if !ok {
		l.Error().Str("framework", framework).Msg("framework config not found")
		err := fmt.Errorf("framework config not found")
		return false, err
	}

	// Get task schedule
	taskSchedule, ok := frameworkCfg.ScheduleLimits[task]
	if !ok {
		// Search for the "any" field
		taskSchedule, ok = frameworkCfg.ScheduleLimits["any"]
		if !ok {
			l.Error().Str("framework", framework).Str("task", task).Msg("cannot find default (or specific) value for task schedule")
			err := fmt.Errorf("cannot find default (or specific) value for task schedule")
			return false, err
		}
	}

	// Check schedule
	frameworkTaskandSchedule := strings.Split(taskSchedule, ":")
	if len(frameworkTaskandSchedule) != 2 {
		l.Error().Str("framework", framework).Str("task", task).Msg("malformed dependency configuration, expected two elements separated by \":\" ")
		return false, nil
	}
	if frameworkTaskandSchedule[0] == "none" {
		// No dependencies
		l.Debug().Str("framework", framework).Str("task", task).Msg("No schedule: Dchedule OK")
		return true, nil
	}
	value, err := strconv.ParseInt(frameworkTaskandSchedule[0], 10, 32)
	if err != nil {
		l.Error().Str("framework", framework).Str("task", task).Msg("malformed dependency configuration, first element must be an integer number")
		return false, nil
	}

	if frameworkTaskandSchedule[1] == "session" {
		// Check if session is within minimum schedule
		lastHeight := taskData.GetLastHeight()
		if (block.Height - lastHeight) >= (value * block.BlocksPerSession) {
			return true, nil
		}

	} else if frameworkTaskandSchedule[1] == "block" {
		// Check if amount of blocks have passed
		lastHeight := taskData.GetLastHeight()
		if (block.Height - lastHeight) >= value {
			return true, nil
		}

	} else {
		l.Error().Str("framework", framework).Str("task", task).Str("second_element", frameworkTaskandSchedule[1]).Msg("schedule configuration cannot be processed (second element type unknown)")
		return false, nil
	}

	return true, nil
}

// Analyzes the configuration and checks wheter the task should be triggered
// despite having its buffers filled and up to date. This is useful for tasks
// that require scheduled updates, like signatures (i.e getting tokenizers every session)
func CheckTaskTriggerMin(taskData TaskInterface, block types.BlockData, configMap map[string]types.FrameworkConfig, l *zerolog.Logger) (uint32, error) {

	framework := taskData.GetFramework()
	task := taskData.GetTask()

	// Get Framework config
	frameworkCfg, ok := configMap[framework]
	if !ok {
		l.Error().Str("framework", framework).Msg("framework config not found")
		err := fmt.Errorf("framework config not found")
		return 0, err
	}

	// Get task schedule
	taskTriggerMin, ok := frameworkCfg.TriggerMinimum[task]
	if !ok {
		// Search for the "any" field
		taskTriggerMin, ok = frameworkCfg.TriggerMinimum["any"]
		if !ok {
			l.Error().Str("framework", framework).Str("task", task).Msg("cannot find default (or specific) value for task trigger minimum")
			err := fmt.Errorf("cannot find default (or specific) value for task trigger minimum")
			return 0, err
		}
	}

	// Check trigger minimum
	value, err := strconv.ParseInt(taskTriggerMin, 10, 32)
	if err != nil {
		l.Error().Str("framework", framework).Str("task", task).Msg("malformed trigger minimum configuration, the entry must be a positive integer number")
		return 0, nil
	}

	return uint32(value), nil
}

// ------------------------------------------------------------------------------
// NumericalTaskRecord
// ------------------------------------------------------------------------------

const NumericalTaskTypeName string = "numerical"

// The maximum age of a sample living in a buffer.
const NumericalSampleTTLDays uint32 = 5

// Minimum number of samples to have in a task to consider that it does not require more samples
// According to "tinyBenchmarks: evaluating LLMs with fewer examples" 100 is enough, but also 50 seems adequate.
const NumericalMinSamplesPerTask uint32 = 50

// Maximum size of result buffer and also maximum number of samples to ask per task
const NumericalMaxConcurrentSamplesPerTask uint32 = 10

// This is the length of the buffer and will set the maximum accuracy of the metric.
const NumericalCircularBufferLength uint32 = NumericalMinSamplesPerTask

// All information for a given task
// Each task will have its own data, depending on what it is
type NumericalTaskRecord struct {
	TaskData BaseTaskRecord `bson:"task_data"`
	// metrics
	MeanScore   float32 `bson:"mean_scores"`
	MedianScore float32 `bson:"median_scores"`
	StdScore    float32 `bson:"std_scores"`
	// Times
	MeanProcessTime   float32 `bson:"mean_times"`
	MedianProcessTime float32 `bson:"median_times"`
	StdProcessTime    float32 `bson:"std_times"`
	// Errors
	ErrorRate  float32     `bson:"error_rate"`
	ErrorCodes map[int]int `bson:"error_codes"`
	// buffer
	ScoresSamples []ScoresSample `bson:"scores"`
	// circular buffer control
	CircBuffer types.CircularBuffer `bson:"circ_buffer_control"`
}

type ScoresSample struct {
	Score      float64 `bson:"score"`
	ID         int     `bson:"id"`
	RunTime    float32 `bson:"run_time"`
	StatusCode int     `bson:"status_code"`
}

func (record *NumericalTaskRecord) NewTask(nodeID primitive.ObjectID, framework string, task string, date time.Time, l *zerolog.Logger) {
	// TODO: Get default values from framework-task
	bufferLen := NumericalCircularBufferLength
	timeArray := make([]time.Time, bufferLen)
	for i := range timeArray {
		timeArray[i] = date
	}

	record.TaskData.NodeID = nodeID
	record.TaskData.Framework = framework
	record.TaskData.Task = task
	record.TaskData.LastSeen = date

	record.MeanScore = 0.0
	record.MedianScore = 0.0
	record.StdScore = 0.0
	record.MeanProcessTime = 0.0
	record.MedianProcessTime = 0.0
	record.StdProcessTime = 0.0
	record.ErrorRate = 0.0
	record.ErrorCodes = make(map[int]int, 0)
	record.ScoresSamples = make([]ScoresSample, bufferLen)

	record.CircBuffer = types.CircularBuffer{
		CircBufferLen: bufferLen,
		NumSamples:    0,
		Times:         timeArray,
		Indexes: types.CircularIndexes{
			Start: 0,
			End:   0,
		},
	}

}

func (record *NumericalTaskRecord) LoadTask(nodeID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	task_filter := bson.D{{Key: "task_data.node_id", Value: nodeID}, {Key: "task_data.framework", Value: framework}, {Key: "task_data.task", Value: task}}
	tasksCollection := mongoDB.GetCollection(types.NumericalTaskCollection)
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Retrieve this node entry
	var found bool = true
	cursor := tasksCollection.FindOne(ctxM, task_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("node_id", nodeID.String()).Str("framework", framework).Str("task", task).Msg("Numerical Task not found")
			found = false
		} else {
			l.Error().Msg("Could not retrieve task data from MongoDB.")
			fmt.Print(err)
			return false, err
		}
	}

	return found, nil
}

func (record *NumericalTaskRecord) UpdateTask(nodeID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	tasksCollection := mongoDB.GetCollection(types.NumericalTaskCollection)

	opts := options.FindOneAndUpdate().SetUpsert(true)
	task_filter := bson.D{{Key: "task_data.node_id", Value: nodeID}, {Key: "task_data.framework", Value: framework}, {Key: "task_data.task", Value: task}}
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update given struct
	update := bson.D{{Key: "$set", Value: record}}
	// Get collection and update
	var found bool = true
	err := tasksCollection.FindOneAndUpdate(ctxM, task_filter, update, opts).Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("node_id", nodeID.String()).Str("framework", framework).Str("task", task).Msg("Numerical Task not found, creating one.")
			found = false
		} else {
			l.Error().Msg("Could not retrieve numerical task data from MongoDB.")
			return false, err
		}
	}

	return found, nil
}

func (record *NumericalTaskRecord) GetMinSamplesPerTask() uint32 {
	return NumericalMinSamplesPerTask
}

func (record *NumericalTaskRecord) GetMaxConcurrentSamplesPerTask() uint32 {
	return NumericalMaxConcurrentSamplesPerTask
}

func (record *NumericalTaskRecord) GetSampleTTLDays() uint32 {
	return NumericalSampleTTLDays
}

func (record *NumericalTaskRecord) GetCircularBufferLength() uint32 {
	return NumericalCircularBufferLength
}

func (record *NumericalTaskRecord) GetFramework() string {
	return record.TaskData.GetFramework()
}

func (record *NumericalTaskRecord) GetTask() string {
	return record.TaskData.GetTask()
}

func (record *NumericalTaskRecord) GetLastSeen() time.Time {
	return record.TaskData.GetLastSeen()
}

func (record *NumericalTaskRecord) GetLastHeight() int64 {
	return record.TaskData.GetLastHeight()
}

func (record *NumericalTaskRecord) UpdateLastSeen(timeSample time.Time) (err error) {
	record.TaskData.UpdateLastSeen(timeSample)
	return nil
}

func (record *NumericalTaskRecord) UpdateLastHeight(height int64) (err error) {
	record.TaskData.UpdateLastHeight(height)
	return nil
}

// Returns the number of valid samples in the circular buffer
func (record *NumericalTaskRecord) GetNumSamples() uint32 {
	return record.CircBuffer.NumSamples
}

// Returns True if the task is ok, meaning that their values are updated and correct
func (record *NumericalTaskRecord) IsOK() bool {
	if record.MeanScore+record.MedianScore+record.StdScore != 0.0 {
		// we have some values, so this task is ok
		return true
	} else {
		return false
	}
}

// Calculate task statistics
func (record *NumericalTaskRecord) ProcessData(l *zerolog.Logger) (err error) {

	// Get valid samples
	validIdx, err := record.CircBuffer.GetBufferValidIndexes(l)
	if err != nil {
		return err
	}

	// Slice the buffer and cast
	var auxDataScores []float64
	var auxDataTimes []float64
	totalPunibleErrors := 0
	punibleErrorsCodes := make(map[int]int)
	for _, sampleId := range validIdx {
		sampleStatus := record.ScoresSamples[sampleId].StatusCode
		if sampleStatus == 0 {
			// Add sample to data array
			auxDataScores = append(auxDataScores, float64(record.ScoresSamples[sampleId].Score))
			auxDataTimes = append(auxDataTimes, float64(record.ScoresSamples[sampleId].RunTime))
		} else if sampleStatus == RelayResponseCodes.Node || sampleStatus == RelayResponseCodes.Evaluation {
			// This is a Node or Evaluation (response) error, we should punish the node
			totalPunibleErrors += 1
			punibleErrorsCodes[sampleStatus] += 1
		}
	}

	// Total valid samples
	length := len(auxDataScores)

	// Set errors
	record.ErrorCodes = punibleErrorsCodes
	record.ErrorRate = float32(totalPunibleErrors) / float32(length+totalPunibleErrors)

	// Calculate the scores and times
	if length == 0 {
		record.MeanScore = 0
		record.StdScore = 0
		record.MedianScore = 0
		record.MeanProcessTime = 0
		record.StdProcessTime = 0
		record.MedianProcessTime = 0

	} else if length == 1 {
		record.MeanScore = float32(record.ScoresSamples[record.CircBuffer.Indexes.Start].Score)
		record.StdScore = 0
		record.MedianScore = float32(record.ScoresSamples[record.CircBuffer.Indexes.Start].Score)
		record.MeanProcessTime = float32(record.ScoresSamples[record.CircBuffer.Indexes.Start].RunTime)
		record.StdProcessTime = 0
		record.MedianProcessTime = float32(record.ScoresSamples[record.CircBuffer.Indexes.Start].RunTime)
	} else {
		// Calculate the mean
		record.MeanScore = float32(stat.Mean(auxDataScores, nil))
		// Calculate the standard deviation
		record.StdScore = float32(stat.StdDev(auxDataScores, nil))
		// Calculate the median
		sort.Float64s(auxDataScores)
		if length%2 == 0 {
			record.MedianScore = float32((auxDataScores[length/2-1] + auxDataScores[length/2]) / 2)
		} else {
			record.MedianScore = float32(auxDataScores[length/2])
		}

		// Same for times
		record.MeanProcessTime = float32(stat.Mean(auxDataTimes, nil))
		record.StdProcessTime = float32(stat.StdDev(auxDataTimes, nil))
		sort.Float64s(auxDataTimes)
		if length%2 == 0 {
			record.MedianProcessTime = float32((auxDataTimes[length/2-1] + auxDataTimes[length/2]) / 2)
		} else {
			record.MedianProcessTime = float32(auxDataTimes[length/2])
		}
	}
	return err
}

// Gets the sample index given a step direction (positive: 1 or negative: -1) and for a given marker (start or end of buffer)
func (record *NumericalTaskRecord) StepIndex(step uint32, marker string, positive_step bool, l *zerolog.Logger) error {
	return record.CircBuffer.StepIndex(step, marker, positive_step, l)
}

// Updates the indexes making them point to the initial and final samples in a given time window.
func (record *NumericalTaskRecord) CycleIndexes(l *zerolog.Logger) (bool, error) {
	return record.CircBuffer.CycleIndexes(NumericalSampleTTLDays, l)
}
func (record *NumericalTaskRecord) InsertSample(timeSample time.Time, data interface{}, l *zerolog.Logger) (err error) {
	// Assert data type
	dataOk, ok := data.(ScoresSample)
	if !ok {
		return fmt.Errorf("invalid sample data type")
	}

	// Increment the end
	err = record.StepIndex(1, "end", true, l)
	// Save sample if it is OK or it is an error imputable to the node
	// the rest are ignored on purpose to avoid polluting the buffer with information
	// that is not important to the servicer node. To debug other errors, check the logs...
	if dataOk.StatusCode == RelayResponseCodes.Ok ||
		dataOk.StatusCode == RelayResponseCodes.Node ||
		dataOk.StatusCode == RelayResponseCodes.Evaluation {
		record.ScoresSamples[record.CircBuffer.Indexes.End].Score = dataOk.Score
		record.ScoresSamples[record.CircBuffer.Indexes.End].ID = dataOk.ID
		record.ScoresSamples[record.CircBuffer.Indexes.End].RunTime = dataOk.RunTime
		record.ScoresSamples[record.CircBuffer.Indexes.End].StatusCode = dataOk.StatusCode
		record.CircBuffer.Times[record.CircBuffer.Indexes.End] = timeSample
	}

	return nil
}

func (record *NumericalTaskRecord) GetResultStruct() ResultInterface {
	var thisTaskResults NumericalResultRecord
	return &thisTaskResults
}

// ------------------------------------------------------------------------------
// SignatureTaskRecord
// ------------------------------------------------------------------------------

const SignatureTaskTypeName string = "signature"

// The maximum age of a sample living in a buffer.
const SignatureSampleTTLDays uint32 = 5

// Minimum number of samples to have in a task to consider that it does not require more samples
const SignatureMinSamplesPerTask uint32 = 5

// Maximum size of result buffer and also maximum number of samples to ask per task
const SignatureMaxConcurrentSamplesPerTask uint32 = 1

// This is the length of the buffer and will set the maximum accuracy of the metric.
const SignatureCircularBufferLength uint32 = SignatureMinSamplesPerTask

// Signatures task data
type SignatureTaskRecord struct {
	TaskData BaseTaskRecord `bson:"task_data"`
	// Specific fields
	LastSignature string `bson:"last_signature"`
	// Errors
	ErrorCode int `bson:"error_code"`
	// buffers
	Signatures []SignatureSample `bson:"signatures"`
	// circular buffer control
	CircBuffer types.CircularBuffer `bson:"circ_buffer_control"`
}

type SignatureSample struct {
	Signature  string `bson:"signature"`
	ID         int    `bson:"id"`
	StatusCode int    `bson:"status_code"`
}

func (record *SignatureTaskRecord) NewTask(nodeID primitive.ObjectID, framework string, task string, date time.Time, l *zerolog.Logger) {
	// TODO: Get default values from framework-task
	bufferLen := SignatureCircularBufferLength
	timeArray := make([]time.Time, bufferLen)
	for i := range timeArray {
		timeArray[i] = date
	}

	record.TaskData.NodeID = nodeID
	record.TaskData.Framework = framework
	record.TaskData.Task = task
	record.TaskData.LastSeen = date

	record.LastSignature = ""
	record.ErrorCode = 0
	record.Signatures = make([]SignatureSample, bufferLen)
	record.CircBuffer = types.CircularBuffer{
		CircBufferLen: bufferLen,
		NumSamples:    0,
		Times:         timeArray,
		Indexes: types.CircularIndexes{
			Start: 0,
			End:   0,
		},
	}
}

func (record *SignatureTaskRecord) LoadTask(nodeID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	task_filter := bson.D{{Key: "task_data.node_id", Value: nodeID}, {Key: "task_data.framework", Value: framework}, {Key: "task_data.task", Value: task}}
	tasksCollection := mongoDB.GetCollection(types.SignaturesTaskCollection)
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Retrieve this node entry
	var found bool = true
	cursor := tasksCollection.FindOne(ctxM, task_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("node_id", nodeID.String()).Str("framework", framework).Str("task", task).Msg("Signature Task not found")
			found = false
		} else {
			l.Error().Msg("Could not retrieve task data from MongoDB.")
			fmt.Print(err)
			return false, err
		}
	}

	return found, nil
}

func (record *SignatureTaskRecord) UpdateTask(nodeID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	tasksCollection := mongoDB.GetCollection(types.SignaturesTaskCollection)

	opts := options.FindOneAndUpdate().SetUpsert(true)
	task_filter := bson.D{{Key: "task_data.node_id", Value: nodeID}, {Key: "task_data.framework", Value: framework}, {Key: "task_data.task", Value: task}}
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update given struct
	update := bson.D{{Key: "$set", Value: record}}
	// Get collection and update
	var found bool = true
	err := tasksCollection.FindOneAndUpdate(ctxM, task_filter, update, opts).Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("node_id", nodeID.String()).Str("framework", framework).Str("task", task).Msg("Signature Task not found, creating one.")
			found = false
		} else {
			l.Error().Str("node_id", nodeID.String()).Str("framework", framework).Str("task", task).Msg("Could not retrieve signature task data from MongoDB.")
			return false, err
		}
	}

	return found, nil
}

func (record *SignatureTaskRecord) GetMinSamplesPerTask() uint32 {
	return SignatureMinSamplesPerTask
}

func (record *SignatureTaskRecord) GetMaxConcurrentSamplesPerTask() uint32 {
	return SignatureMaxConcurrentSamplesPerTask
}

func (record *SignatureTaskRecord) GetSampleTTLDays() uint32 {
	return SignatureSampleTTLDays
}

func (record *SignatureTaskRecord) GetCircularBufferLength() uint32 {
	return SignatureCircularBufferLength
}

func (record *SignatureTaskRecord) GetFramework() string {
	return record.TaskData.GetFramework()
}

func (record *SignatureTaskRecord) GetTask() string {
	return record.TaskData.GetTask()
}

func (record *SignatureTaskRecord) GetLastSeen() time.Time {
	return record.TaskData.GetLastSeen()
}

func (record *SignatureTaskRecord) GetLastHeight() int64 {
	return record.TaskData.GetLastHeight()
}

func (record *SignatureTaskRecord) UpdateLastSeen(timeSample time.Time) (err error) {
	record.TaskData.UpdateLastSeen(timeSample)
	return nil
}

func (record *SignatureTaskRecord) UpdateLastHeight(height int64) (err error) {
	record.TaskData.UpdateLastHeight(height)
	return nil
}

// Gets the sample index given a step direction (positive: 1 or negative: -1) and for a given marker (start or end of buffer)
func (record *SignatureTaskRecord) StepIndex(step uint32, marker string, positive_step bool, l *zerolog.Logger) error {
	return record.CircBuffer.StepIndex(step, marker, positive_step, l)
}

// Updates the indexes making them point to the initial and final samples in a given time window.
func (record *SignatureTaskRecord) CycleIndexes(l *zerolog.Logger) (bool, error) {
	return record.CircBuffer.CycleIndexes(NumericalSampleTTLDays, l)
}

// Returns the number of valid samples in the circular buffer
func (record *SignatureTaskRecord) GetNumSamples() uint32 {
	return record.CircBuffer.NumSamples
}

// insert a new signature into the circular buffer
func (record *SignatureTaskRecord) InsertSample(timeSample time.Time, data interface{}, l *zerolog.Logger) (err error) {
	// Assert data type
	dataOk, ok := data.(SignatureSample)
	if !ok {
		return fmt.Errorf("invalid sample data type")
	}

	l.Debug().Str("signature", dataOk.Signature).Int("ID", dataOk.ID).Msg("Inserting sample.")

	// Increment the end
	err = record.StepIndex(1, "end", true, l)
	// Save sample if it is OK or it is an error imputable to the node
	if dataOk.StatusCode == RelayResponseCodes.Ok ||
		dataOk.StatusCode == RelayResponseCodes.Node ||
		dataOk.StatusCode == RelayResponseCodes.Evaluation {

		record.Signatures[record.CircBuffer.Indexes.End].Signature = dataOk.Signature
		record.Signatures[record.CircBuffer.Indexes.End].ID = dataOk.ID
		record.Signatures[record.CircBuffer.Indexes.End].StatusCode = dataOk.StatusCode
		record.CircBuffer.Times[record.CircBuffer.Indexes.End] = timeSample

	}

	return nil
}

// Returns True if the task is ok, meaning that their values are updated and correct
func (record *SignatureTaskRecord) IsOK() bool {
	if record.LastSignature != "" && record.ErrorCode == 0 {
		// there is a signature available, so it is OK
		return true
	} else {
		return false
	}
}

// Process the buffer data to produce the signature metrics
func (record *SignatureTaskRecord) ProcessData(l *zerolog.Logger) (err error) {
	// Just update the last signature
	lastSampleStatus := record.Signatures[record.CircBuffer.Indexes.End].StatusCode
	if lastSampleStatus == 0 {
		record.LastSignature = record.Signatures[record.CircBuffer.Indexes.End].Signature
		record.ErrorCode = 0
	} else {
		record.LastSignature = ""
		record.ErrorCode = lastSampleStatus
	}

	return nil
}

func (record *SignatureTaskRecord) GetResultStruct() ResultInterface {
	var thisTaskResults SignatureResultRecord
	return &thisTaskResults
}
