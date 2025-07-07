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
	SupplierID primitive.ObjectID `bson:"supplier_id"`
	Framework  string             `bson:"framework"`
	Task       string             `bson:"task"`

	LastSeen   time.Time `bson:"last_seen"`
	LastHeight int64     `bson:"last_height"`
}

func (record *BaseTaskRecord) GetSupplierID() primitive.ObjectID {
	return record.SupplierID
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
	InsertSample(timeSample time.Time, data interface{}, l *zerolog.Logger) (ok bool, err error)
	GetNumSamples() uint32
	GetNumOkSamples() uint32
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
	NewTask(supplierID primitive.ObjectID, framework string, task string, date time.Time, l *zerolog.Logger)
	LoadTask(supplierID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error)
	UpdateTask(supplierID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error)
}

// Get specific task data from a supplier record
func GetTaxonomyData(
	supplierID primitive.ObjectID,
	taxonomy string,
	mongoDB mongodb.MongoDb,
	l *zerolog.Logger) (taxonomySummary types.TaxonomySummary, found bool) {

	task_filter := bson.D{
		{Key: "supplier_id", Value: supplierID},
		{Key: "taxonomy_name", Value: taxonomy},
	}
	taxonomyCollection := mongoDB.GetCollection(types.TaxonomySummariesCollection)
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Retrieve this supplier entry
	found = true
	cursor := taxonomyCollection.FindOne(ctxM, task_filter, opts)
	err := cursor.Decode(&taxonomySummary)
	if err != nil {
		found = false
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("supplier_id", supplierID.String()).Str("taxonomy", taxonomy).Msg("Taxonomy summary not found")
		} else {
			l.Error().Err(err).Str("supplierID", supplierID.String()).Str("taxonomy", taxonomy).Msg("Could not retrieve taxonomy summary data from MongoDB")
		}
	}
	return taxonomySummary, found

}

// Get specific task data from a supplier record
func GetTaskData(
	supplierID primitive.ObjectID,
	taskType string,
	framework string,
	task string,
	create_new bool,
	mongoDB mongodb.MongoDb,
	l *zerolog.Logger) (TaskInterface, bool) {

	// Look for entry
	if taskType == NumericalTaskTypeName {
		// get task record
		var record NumericalTaskRecord
		found, err := record.LoadTask(supplierID, framework, task, mongoDB, l)
		if err != nil {
			l.Error().Str("supplierID", supplierID.String()).Str("framework", framework).Str("task", task).Msg("cannot find default task buffer")
			return nil, false
		}
		if !found {
			if create_new {
				// Initialize and save
				record.NewTask(supplierID, framework, task, types.EpochStart.UTC(), l)
				record.UpdateTask(supplierID, framework, task, mongoDB, l)
			} else {
				return nil, false
			}
		}
		return &record, true
	} else if taskType == SignatureTaskTypeName {
		// set task record
		var record SignatureTaskRecord
		found, err := record.LoadTask(supplierID, framework, task, mongoDB, l)
		if err != nil {
			l.Error().Str("supplierID", supplierID.String()).Str("framework", framework).Str("task", task).Msg("cannot find default task buffer")
			return nil, false
		}
		if !found {
			if create_new {
				// Initialize and save
				record.NewTask(supplierID, framework, task, types.EpochStart.UTC(), l)
				record.UpdateTask(supplierID, framework, task, mongoDB, l)
			} else {
				return nil, false
			}
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

// Analyzes the taxonomy dependencies and returns if it is possible to proceed with this task triggering/analysis
// A task can depend on some taxonomies to be passed at a certain level, here we check for that
func CheckTaxonomyDependency(
	supplierData *SupplierRecord,
	framework string,
	task string,
	configMap map[string]types.FrameworkConfig,
	mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	// Get Framework config
	frameworkCfg, ok := configMap[framework]
	if !ok {
		l.Error().Str("framework", framework).Msg("framework config not found")
		err := fmt.Errorf("framework config not found")
		return false, err
	}

	// Get taxonomy dependency
	taskDep, ok := frameworkCfg.TaxonomyDependency[task]
	if !ok {
		// Search for the "any" field
		taskDep, ok = frameworkCfg.TaxonomyDependency["any"]
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
		frameworkTaxonomyAndStatus := strings.Split(taskDep[idxDep], ":")
		if len(frameworkTaxonomyAndStatus) != 4 {
			l.Error().Str("framework", framework).Str("task", task).Msg("malformed taxonomy dependency configuration, expected four elements separated by \":\" ")
			depOK = false
			break
		}
		if frameworkTaxonomyAndStatus[0] == "none" {
			// No dependencies
			l.Debug().Str("address", supplierData.Address).Str("service", supplierData.Service).Str("framework", framework).Str("task", task).Msg("No taxonomy dependency: Dependency OK")
			continue
		}
		// All other three must be numerical entries
		scoreMin, err := strconv.ParseFloat(frameworkTaxonomyAndStatus[1], 64)
		if err != nil {
			l.Error().Str("framework", framework).Str("task", task).Str("taxonomy", frameworkTaxonomyAndStatus[0]).Msg("malformed taxonomy dependency configuration, cannot convert string to float for first element.")
			depOK = false
			break
		}
		// TODO : Implement success rate tracking
		// successRateMin, err := strconv.ParseFloat(frameworkTaxonomyAndStatus[2], 64)
		// if err != nil {
		// 	l.Error().Str("framework", framework).Str("task", task).Str("taxonomy", frameworkTaxonomyAndStatus[0]).Msg("malformed taxonomy dependency configuration, cannot convert string to float for second element.")
		// 	depOK = false
		// 	break
		// }
		samplesMin, err := strconv.ParseFloat(frameworkTaxonomyAndStatus[3], 64)
		if err != nil {
			l.Error().Str("framework", framework).Str("task", task).Str("taxonomy", frameworkTaxonomyAndStatus[0]).Msg("malformed taxonomy dependency configuration, cannot convert string to float for third element.")
			depOK = false
			break
		}

		// Get the taxonomy to evaluate
		thisTaxonomySummary, found := GetTaxonomyData(supplierData.ID, frameworkTaxonomyAndStatus[0], mongoDB, l)
		if !found {
			// The task is not even created, we must fail
			depOK = false
			break
		} else {
			// Check the condition over the root_c node
			taxonomyRootNode, found := thisTaxonomySummary.TaxonomyNodesScores["root_c"]
			if !found {
				l.Error().Str("framework", framework).Str("task", task).Str("taxonomy", frameworkTaxonomyAndStatus[0]).Msg("malformed taxonomy summary, no root_c node!")
				depOK = false
				break
			}

			if (taxonomyRootNode.Score < scoreMin) ||
				// (taxonomyRootNode.ErrorRate > (1-successRateMin)) ||
				(float64(taxonomyRootNode.SampleMin) < samplesMin) {
				// Condition not met
				depOK = false
				break
			}
		}
	}

	return depOK, nil
}

// Analyzes the configuration and returns if it is possible to proceed with this task triggering/analysis
// A task can depend on others (such as having a tokenizer signature), here we check for that
func CheckTaskDependency(supplierData *SupplierRecord, framework string, task string, configMap map[string]types.FrameworkConfig, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

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
			l.Debug().Str("address", supplierData.Address).Str("service", supplierData.Service).Str("framework", framework).Str("task", task).Msg("No dependency: Dependency OK")
			continue
		}
		taskType, err := GetTaskType(frameworkTaskandStatus[0], frameworkTaskandStatus[1], configMap, l)
		if err != nil {
			l.Error().Str("framework", framework).Str("task", task).Str("task type", taskType).Msg("Error getting task type")
			return false, err
		}
		thisTaskRecord, found := GetTaskData(supplierData.ID, taskType, frameworkTaskandStatus[0], frameworkTaskandStatus[1], false, mongoDB, l)
		if !found {
			// The task is not even created, we must fail
			depOK = false
			break
		} else {
			// Check the condition
			if frameworkTaskandStatus[2] == "present" {
				// Task is present, so OK
				l.Debug().Str("address", supplierData.Address).Str("service", supplierData.Service).Str("framework", framework).Str("task", task).Msg("Present: Dependency OK")
				continue
			} else if frameworkTaskandStatus[2] == "ok" {
				// Check for it having a correct value
				if thisTaskRecord.IsOK() {
					l.Debug().Str("address", supplierData.Address).Str("service", supplierData.Service).Str("framework", framework).Str("task", task).Msg("OK: Dependency OK")
					continue
				} else {
					l.Debug().Str("address", supplierData.Address).Str("service", supplierData.Service).Str("framework", framework).Str("task", task).Msg("OK: Dependency NOT OK")
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
		} else {
			return false, nil
		}

	} else if frameworkTaskandSchedule[1] == "block" {
		// Check if amount of blocks have passed
		lastHeight := taskData.GetLastHeight()
		if (block.Height - lastHeight) >= value {
			return true, nil
		} else {
			return false, nil
		}

	} else {
		l.Error().Str("framework", framework).Str("task", task).Str("second_element", frameworkTaskandSchedule[1]).Msg("schedule configuration cannot be processed (second element type unknown)")
		return false, nil
	}

	return true, nil
}

// Analyzes the configuration and checks whether the task should be triggered
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
const NumericalSampleTTLDays uint32 = 3

// Minimum number of samples to have in a task to consider that it does not require more samples
// According to "tinyBenchmarks: evaluating LLMs with fewer examples" 100 is enough, but also 50 seems adequate.
const NumericalMinSamplesPerTask uint32 = 50

// Maximum size of result buffer and also maximum number of samples to ask per task
const NumericalMaxConcurrentSamplesPerTask uint32 = 10

// This is the length of the buffer and will set the maximum accuracy of the metric.
const NumericalCircularBufferLength uint32 = NumericalMinSamplesPerTask * 2

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
	Score       float64 `bson:"score"`
	ID          int     `bson:"id"`
	RunTime     float32 `bson:"run_time"`
	StatusCode  int     `bson:"status_code"`
	ErrorString string  `bson:"error_str"`
}

func (record *NumericalTaskRecord) NewTask(supplierID primitive.ObjectID, framework string, task string, date time.Time, l *zerolog.Logger) {
	// TODO: Get default values from framework-task
	bufferLen := NumericalCircularBufferLength
	timeArray := make([]time.Time, bufferLen)
	for i := range timeArray {
		timeArray[i] = date
	}

	record.TaskData.SupplierID = supplierID
	record.TaskData.Framework = framework
	record.TaskData.Task = task
	record.TaskData.LastSeen = time.Now().UTC()

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

func (record *NumericalTaskRecord) LoadTask(supplierID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	task_filter := bson.D{{Key: "task_data.supplier_id", Value: supplierID}, {Key: "task_data.framework", Value: framework}, {Key: "task_data.task", Value: task}}
	tasksCollection := mongoDB.GetCollection(types.NumericalTaskCollection)
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Retrieve this supplier entry
	var found bool = true
	cursor := tasksCollection.FindOne(ctxM, task_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("supplier_id", supplierID.String()).Str("framework", framework).Str("task", task).Msg("Numerical Task not found")
			found = false
		} else {
			l.Error().Msg("Could not retrieve task data from MongoDB.")
			fmt.Print(err)
			return false, err
		}
	}

	return found, nil
}

func (record *NumericalTaskRecord) UpdateTask(supplierID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	tasksCollection := mongoDB.GetCollection(types.NumericalTaskCollection)

	opts := options.FindOneAndUpdate().SetUpsert(true)
	task_filter := bson.D{{Key: "task_data.supplier_id", Value: supplierID}, {Key: "task_data.framework", Value: framework}, {Key: "task_data.task", Value: task}}
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Update given struct
	update := bson.D{{Key: "$set", Value: record}}
	// Get collection and update
	var found bool = true
	err := tasksCollection.FindOneAndUpdate(ctxM, task_filter, update, opts).Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("supplier_id", supplierID.String()).Str("framework", framework).Str("task", task).Msg("Numerical Task not found, creating one.")
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

// Returns the number of correct (not errored) samples in the circular buffer
func (record *NumericalTaskRecord) GetNumOkSamples() uint32 {
	// Initialize a counter for the number of OK samples
	var okSamples uint32 = 0

	// Loop through all the elements of `ScoresSamples` and count the number that have `StatusCode==0`
	// and checking if the element index is within the valid range using the function "IsIndexInRange(uint32)"
	for i, sample := range record.ScoresSamples {
		if record.CircBuffer.IsIndexInRange(uint32(i)) && sample.StatusCode == 0 {
			okSamples++
		}
	}

	return okSamples
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
		} else if sampleStatus == RelayResponseCodes.Supplier || sampleStatus == RelayResponseCodes.Evaluation {
			// This is a Supplier or Evaluation (response) error, we should punish the supplier
			totalPunibleErrors += 1
			punibleErrorsCodes[sampleStatus] += 1
		}
	}

	// Total valid samples
	length := len(auxDataScores)

	// Set errors
	record.ErrorCodes = punibleErrorsCodes
	record.ErrorRate = 0.0
	if float32(length+totalPunibleErrors) > 0 {
		record.ErrorRate = float32(totalPunibleErrors) / float32(length+totalPunibleErrors)
	}

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
func (record *NumericalTaskRecord) InsertSample(timeSample time.Time, data interface{}, l *zerolog.Logger) (statusOK bool, err error) {
	// Assert data type
	dataOk, ok := data.(ScoresSample)
	if !ok {
		return ok, fmt.Errorf("invalid sample data type")
	}

	// Save sample if it is OK or it is an error imputable to the supplier
	// the rest are ignored on purpose to avoid polluting the buffer with information
	// that is not important to the servicer supplier. To debug other errors, check the logs...
	if dataOk.StatusCode == RelayResponseCodes.Ok ||
		dataOk.StatusCode == RelayResponseCodes.Supplier ||
		dataOk.StatusCode == RelayResponseCodes.Evaluation {

		// Increment the end (only on valid data)
		err = record.StepIndex(1, "end", true, l)

		// Update the buffer
		record.ScoresSamples[record.CircBuffer.Indexes.End].Score = dataOk.Score
		record.ScoresSamples[record.CircBuffer.Indexes.End].ID = dataOk.ID
		record.ScoresSamples[record.CircBuffer.Indexes.End].RunTime = dataOk.RunTime
		record.ScoresSamples[record.CircBuffer.Indexes.End].StatusCode = dataOk.StatusCode
		record.CircBuffer.Times[record.CircBuffer.Indexes.End] = timeSample
	}
	if dataOk.StatusCode == RelayResponseCodes.Ok {
		// Sample was ok
		statusOK = true
	}

	return statusOK, nil
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
const SignatureSampleTTLDays uint32 = 3

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
	Signature   string `bson:"signature"`
	ID          int    `bson:"id"`
	StatusCode  int    `bson:"status_code"`
	ErrorString string `bson:"error_str"`
}

func (record *SignatureTaskRecord) NewTask(supplierID primitive.ObjectID, framework string, task string, date time.Time, l *zerolog.Logger) {
	// TODO: Get default values from framework-task
	bufferLen := SignatureCircularBufferLength
	timeArray := make([]time.Time, bufferLen)
	for i := range timeArray {
		timeArray[i] = date
	}

	record.TaskData.SupplierID = supplierID
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

func (record *SignatureTaskRecord) LoadTask(supplierID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	task_filter := bson.D{{Key: "task_data.supplier_id", Value: supplierID}, {Key: "task_data.framework", Value: framework}, {Key: "task_data.task", Value: task}}
	tasksCollection := mongoDB.GetCollection(types.SignaturesTaskCollection)
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Retrieve this supplier entry
	var found bool = true
	cursor := tasksCollection.FindOne(ctxM, task_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("supplier_id", supplierID.String()).Str("framework", framework).Str("task", task).Msg("Signature Task not found")
			found = false
		} else {
			l.Error().Msg("Could not retrieve task data from MongoDB.")
			fmt.Print(err)
			return false, err
		}
	}

	return found, nil
}

func (record *SignatureTaskRecord) UpdateTask(supplierID primitive.ObjectID, framework string, task string, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	tasksCollection := mongoDB.GetCollection(types.SignaturesTaskCollection)

	opts := options.FindOneAndUpdate().SetUpsert(true)
	task_filter := bson.D{{Key: "task_data.supplier_id", Value: supplierID}, {Key: "task_data.framework", Value: framework}, {Key: "task_data.task", Value: task}}
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Update given struct
	update := bson.D{{Key: "$set", Value: record}}
	// Get collection and update
	var found bool = true
	err := tasksCollection.FindOneAndUpdate(ctxM, task_filter, update, opts).Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("supplier_id", supplierID.String()).Str("framework", framework).Str("task", task).Msg("Signature Task not found, creating one.")
			found = false
		} else {
			l.Error().Str("supplier_id", supplierID.String()).Str("framework", framework).Str("task", task).Msg("Could not retrieve signature task data from MongoDB.")
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

// Returns the number of correct (not errored) samples in the circular buffer
func (record *SignatureTaskRecord) GetNumOkSamples() uint32 {
	// Initialize a counter for the number of OK samples
	var okSamples uint32 = 0

	// Loop through all the elements of `Signatures` and count the number that have `StatusCode==0`
	// and checking if the element index is within the valid range using the function "IsIndexInRange(uint32)"
	for i, sample := range record.Signatures {
		if record.CircBuffer.IsIndexInRange(uint32(i)) && sample.StatusCode == 0 {
			okSamples++
		}
	}

	return okSamples
}

// insert a new signature into the circular buffer
func (record *SignatureTaskRecord) InsertSample(timeSample time.Time, data interface{}, l *zerolog.Logger) (statusOK bool, err error) {
	// Assert data type
	dataOk, ok := data.(SignatureSample)
	if !ok {
		return ok, fmt.Errorf("invalid sample data type")
	}

	l.Debug().Str("signature", dataOk.Signature).Int("ID", dataOk.ID).Msg("Inserting sample.")

	// Increment the end
	err = record.StepIndex(1, "end", true, l)
	// Save sample if it is OK or it is an error imputable to the supplier
	if dataOk.StatusCode == RelayResponseCodes.Ok ||
		dataOk.StatusCode == RelayResponseCodes.Supplier ||
		dataOk.StatusCode == RelayResponseCodes.Evaluation {

		record.Signatures[record.CircBuffer.Indexes.End].Signature = dataOk.Signature
		record.Signatures[record.CircBuffer.Indexes.End].ID = dataOk.ID
		record.Signatures[record.CircBuffer.Indexes.End].StatusCode = dataOk.StatusCode
		record.CircBuffer.Times[record.CircBuffer.Indexes.End] = timeSample
	}
	if dataOk.StatusCode == RelayResponseCodes.Ok {
		// Sample was ok
		statusOK = true
	}

	return statusOK, nil
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
