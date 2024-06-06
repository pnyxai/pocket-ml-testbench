package records

import (
	"fmt"
	"manager/types"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"gonum.org/v1/gonum/stat"
)

// ------------------------------------------------------------------------------
// BaseTaskRecord
// ------------------------------------------------------------------------------

// This is the basic information that all tasks should have
type BaseTaskRecord struct {
	Framework string    `bson:"framework"`
	Task      string    `bson:"task"`
	LastSeen  time.Time `bson:"last_seen"`
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

func (record *BaseTaskRecord) UpdateLastSeen(timeSample time.Time) (err error) {
	record.LastSeen = timeSample
	return nil
}

// The maximum age of a task entry.
const TaskTTLDays uint32 = 32

// ------------------------------------------------------------------------------
// TaskInterface all task structs will respond to this, for ease of processing
// ------------------------------------------------------------------------------

type TaskInterface interface {
	ProcessData(l *zerolog.Logger) error
	stepIndex(step int, marker string) error
	CycleIndexes(l *zerolog.Logger) error
	InsertSample(timeSample time.Time, data interface{}) (err error)
	GetNumSamples() uint32
	GetFramework() string
	GetTask() string
	UpdateLastSeen(timeSample time.Time) (err error)
	GetMinSamplesPerTask() uint32
	GetMaxConcurrentSamplesPerTask() uint32
	GetCircularBufferLength() uint32
	GetSampleTTLDays() uint32
	GetResultStruct() ResultInterface
	GetLastSeen() time.Time
	IsOK() bool
}

// Get specific task data from a node record
func GetTaskData(nodeData *NodeRecord, framework string, task string, l *zerolog.Logger) (TaskInterface, bool) {

	// Get all tasks as a single array
	combinedTasks := nodeData.CombineTasks()
	// Look for entry
	for _, taskEntry := range combinedTasks {
		// Check if the Name field matches the search string
		if taskEntry.GetFramework() == framework && taskEntry.GetTask() == task {
			l.Debug().Str("address", nodeData.Address).Str("service", nodeData.Service).Str("framework", framework).Str("task", task).Msg("Found!")
			return taskEntry, true
		}
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
func CheckTaskDependency(nodeData *NodeRecord, framework string, task string, configMap map[string]types.FrameworkConfig, l *zerolog.Logger) (status bool, err error) {
	status = false

	// Get Framework config
	frameworkCfg, ok := configMap[framework]
	if !ok {
		l.Error().Str("framework", framework).Msg("framework config not found")
		err = fmt.Errorf("framework config not found")
		return false, err
	}

	// Get task type
	taskDep, ok := frameworkCfg.TasksDependency[task]
	if !ok {
		// Search for the "any" field
		taskDep, ok = frameworkCfg.TasksDependency["any"]
		if !ok {
			l.Error().Str("framework", framework).Str("task", task).Msg("cannot find default (or specific) value for task type")
			err = fmt.Errorf("cannot find default (or specific) value for task type")
			return false, err
		}
	}

	// Check dependency
	frameworkTaskandStatus := strings.Split(taskDep, ":")
	thisTaskRecord, found := GetTaskData(nodeData, frameworkTaskandStatus[0], frameworkTaskandStatus[1], l)
	if !found {
		// The task is not even created, we must fail
		return false, nil
	} else {
		// Check the condition
		if frameworkTaskandStatus[2] == "present" {
			// Task is present, so OK
			return true, nil
		} else if frameworkTaskandStatus[2] == "ok" {
			// Check for it having a correct value
			if thisTaskRecord.IsOK() {
				return true, nil
			}
		} else {
			l.Error().Str("framework", framework).Str("task", task).Msg("dependency configuration cannot be processed (status type unknown)")
			return false, nil
		}
	}

	return false, nil
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
	// buffer
	ScoresSamples []ScoresSample `bson:"scores"`
	// circular buffer control
	CircBuffer types.CircularBuffer `bson:"circ_buffer_control"`
}

type ScoresSample struct {
	Score float32 `bson:"scores"`
	ID    int     `bson:"id"`
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

func (record *NumericalTaskRecord) UpdateLastSeen(timeSample time.Time) (err error) {
	record.TaskData.UpdateLastSeen(timeSample)
	return nil
}

// Returns the number of valid samples in the circular buffer
func (record *NumericalTaskRecord) GetNumSamples() uint32 {
	return record.CircBuffer.NumSamples
}

// Returns True if the task is ok, meaning that their values are updated and correct
func (record *NumericalTaskRecord) IsOK() bool {
	if record.MeanScore+record.MedianScore+record.StdScore != 0 {
		// we have some values, so this task is ok
		return true
	} else {
		return false
	}
}

// Calculate task statistics
func (record *NumericalTaskRecord) ProcessData(l *zerolog.Logger) (err error) {

	// Slice the buffer and cast
	var auxData []float64
	idxNow := record.CircBuffer.Indexes.Start
	for true {
		// run until we complete the circular buffer
		if idxNow == record.CircBuffer.Indexes.End {
			break
		}
		// Add sample to data array
		auxData = append(auxData, float64(record.ScoresSamples[idxNow].Score))
		// perform the step
		nextVal := int(idxNow) + 1
		// Check limits and assign value
		idxNow = record.CircBuffer.BufferLimitCheck(nextVal, record.CircBuffer.Indexes.End)
	}
	length := len(auxData)
	if length == 0 {
		record.MeanScore = 0
		record.StdScore = 0
		record.MedianScore = 0
	} else if length == 1 {
		record.MeanScore = record.ScoresSamples[record.CircBuffer.Indexes.Start].Score
		record.StdScore = 0
		record.MedianScore = record.ScoresSamples[record.CircBuffer.Indexes.Start].Score
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
func (record *NumericalTaskRecord) stepIndex(step int, marker string) error {
	return record.CircBuffer.StepIndex(step, marker)
}

// Updates the indexes making them point to the initial and final samples in a given time window.
func (record *NumericalTaskRecord) CycleIndexes(l *zerolog.Logger) error {
	return record.CircBuffer.CycleIndexes(NumericalSampleTTLDays, l)
}
func (record *NumericalTaskRecord) InsertSample(timeSample time.Time, data interface{}) (err error) {
	// Assert data type
	dataOk, ok := data.(ScoresSample)
	if !ok {
		return fmt.Errorf("invalid sample data type")
	}

	// Increment the end
	err = record.stepIndex(1, "end")
	// Save sample
	record.ScoresSamples[record.CircBuffer.Indexes.End].Score = dataOk.Score
	record.ScoresSamples[record.CircBuffer.Indexes.End].ID = dataOk.ID
	record.CircBuffer.Times[record.CircBuffer.Indexes.End] = timeSample

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
const SignatureMinSamplesPerTask uint32 = 50

// Maximum size of result buffer and also maximum number of samples to ask per task
const SignatureMaxConcurrentSamplesPerTask uint32 = 10

// This is the length of the buffer and will set the maximum accuracy of the metric.
const SignatureCircularBufferLength uint32 = NumericalMinSamplesPerTask

// Signatures task data
type SignatureTaskRecord struct {
	TaskData BaseTaskRecord `bson:"task_data"`
	// Specific fields
	LastSignature string `bson:"last_signature"`
	// buffers
	Signatures []SignatureSample `bson:"signatures"`
	// circular buffer control
	CircBuffer types.CircularBuffer `bson:"circ_buffer_control"`
}

type SignatureSample struct {
	Signature string `bson:"signature"`
	ID        int    `bson:"id"`
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

func (record *SignatureTaskRecord) UpdateLastSeen(timeSample time.Time) (err error) {
	record.TaskData.UpdateLastSeen(timeSample)
	return nil
}

// Gets the sample index given a step direction (positive: 1 or negative: -1) and for a given marker (start or end of buffer)
func (record *SignatureTaskRecord) stepIndex(step int, marker string) error {
	return record.CircBuffer.StepIndex(step, marker)
}

// Updates the indexes making them point to the initial and final samples in a given time window.
func (record *SignatureTaskRecord) CycleIndexes(l *zerolog.Logger) error {
	return record.CircBuffer.CycleIndexes(NumericalSampleTTLDays, l)
}

// Returns the number of valid samples in the circular buffer
func (record *SignatureTaskRecord) GetNumSamples() uint32 {
	return record.CircBuffer.NumSamples
}

// insert a new signature into the circular buffer
func (record *SignatureTaskRecord) InsertSample(timeSample time.Time, data interface{}) (err error) {
	// Assert data type
	dataOk, ok := data.(SignatureSample)
	if !ok {
		return fmt.Errorf("invalid sample data type")
	}
	// Increment the end
	err = record.stepIndex(1, "end")
	// Save sample
	record.Signatures[record.CircBuffer.Indexes.End].Signature = dataOk.Signature
	record.Signatures[record.CircBuffer.Indexes.End].ID = dataOk.ID
	record.CircBuffer.Times[record.CircBuffer.Indexes.End] = timeSample

	return nil
}

// Returns True if the task is ok, meaning that their values are updated and correct
func (record *SignatureTaskRecord) IsOK() bool {
	if record.LastSignature != "" {
		// there is a signature available, so it is OK
		return true
	} else {
		return false
	}
}

// Process the buffer data to produce the signature metrics
func (record *SignatureTaskRecord) ProcessData(l *zerolog.Logger) (err error) {
	// Just update the last signature
	record.LastSignature = record.Signatures[record.CircBuffer.Indexes.End].Signature
	return nil
}

func (record *SignatureTaskRecord) GetResultStruct() ResultInterface {
	var thisTaskResults SignatureResultRecord
	return &thisTaskResults
}
