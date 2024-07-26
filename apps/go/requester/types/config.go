package types

import (
	"encoding/json"
	"go.temporal.io/sdk/worker"
	"time"
)

type TemporalWorkerOptions struct {
	// Optional: To set the maximum concurrent activity executions this worker can have.
	// The zero value of this uses the default value.
	// default: defaultMaxConcurrentActivityExecutionSize(1k)
	MaxConcurrentActivityExecutionSize int `json:"max_concurrent_activity_execution_size"`

	// Optional: Sets the rate limiting on number of activities that can be executed per second per
	// worker. This can be used to limit resources used by the worker.
	// Notice that the number is represented in float, so that you can set it to less than
	// 1 if needed. For example, set the number to 0.1 means you want your activity to be executed
	// once for every 10 seconds. This can be used to protect down stream services from flooding.
	// The zero value of this uses the default value
	// default: 100k
	WorkerActivitiesPerSecond float64 `json:"worker_activities_per_second"`

	// Optional: To set the maximum concurrent local activity executions this worker can have.
	// The zero value of this uses the default value.
	// default: 1k
	MaxConcurrentLocalActivityExecutionSize int `json:"max_concurrent_local_activity_execution_size"`

	// Optional: Sets the rate limiting on number of local activities that can be executed per second per
	// worker. This can be used to limit resources used by the worker.
	// Notice that the number is represented in float, so that you can set it to less than
	// 1 if needed. For example, set the number to 0.1 means you want your local activity to be executed
	// once for every 10 seconds. This can be used to protect down stream services from flooding.
	// The zero value of this uses the default value
	// default: 100k
	WorkerLocalActivitiesPerSecond float64 `json:"worker_local_activities_per_second"`

	// Optional: Sets the rate limiting on number of activities that can be executed per second.
	// This is managed by the server and controls activities per second for your entire taskqueue
	// whereas WorkerActivityTasksPerSecond controls activities only per worker.
	// Notice that the number is represented in float, so that you can set it to less than
	// 1 if needed. For example, set the number to 0.1 means you want your activity to be executed
	// once for every 10 seconds. This can be used to protect down stream services from flooding.
	// The zero value of this uses the default value.
	// default: 100k
	//
	// Note: Setting this to a non zero value will also disable eager activities.
	TaskQueueActivitiesPerSecond float64 `json:"task_queue_activities_per_second"`

	// Optional: Sets the maximum number of goroutines that will concurrently poll the
	// temporal-server to retrieve activity tasks. Changing this value will affect the
	// rate at which the worker is able to consume tasks from a task queue.
	// default: 2
	MaxConcurrentActivityTaskPollers int `json:"max_concurrent_activity_task_pollers"`

	// Optional: To set the maximum concurrent workflow task executions this worker can have.
	// The zero value of this uses the default value. Due to internal logic where pollers
	// alternate between stick and non-sticky queues, this
	// value cannot be 1 and will panic if set to that value.
	// default: defaultMaxConcurrentTaskExecutionSize(1k)
	MaxConcurrentWorkflowTaskExecutionSize int `json:"max_concurrent_workflow_task_execution_size"`

	// Optional: Sets the maximum number of goroutines that will concurrently poll the
	// temporal-server to retrieve workflow tasks. Changing this value will affect the
	// rate at which the worker is able to consume tasks from a task queue. Due to
	// internal logic where pollers alternate between stick and non-sticky queues, this
	// value cannot be 1 and will panic if set to that value.
	// default: 2
	MaxConcurrentWorkflowTaskPollers int `json:"max_concurrent_workflow_task_pollers"`

	// Optional: Enable logging in replay.
	// In the workflow code you can use workflow.GetLogger(ctx) to write logs. By default, the logger will skip log
	// entry during replay mode so you won't see duplicate logs. This option will enable the logging in replay mode.
	// This is only useful for debugging purpose.
	// default: false
	EnableLoggingInReplay bool `json:"enable_logging_in_replay"`

	// Optional: Disable sticky execution.
	// Sticky Execution is to run the workflow tasks for one workflow execution on same worker host. This is an
	// optimization for workflow execution. When sticky execution is enabled, worker keeps the workflow state in
	// memory. New workflow task contains the new history events will be dispatched to the same worker. If this
	// worker crashes, the sticky workflow task will timeout after StickyScheduleToStartTimeout, and temporal server
	// will clear the stickiness for that workflow execution and automatically reschedule a new workflow task that
	// is available for any worker to pick up and resume the progress.
	// default: false
	//
	// Deprecated: DisableStickyExecution harms performance. It will be removed soon. See SetStickyWorkflowCacheSize
	// instead.
	DisableStickyExecution bool `json:"disable_sticky_execution"`

	// Optional: Sticky schedule to start timeout.
	// The resolution is seconds. See details about StickyExecution on the comments for DisableStickyExecution.
	// default: 5s
	StickyScheduleToStartTimeout float64 `json:"sticky_schedule_to_start_timeout"`

	// Optional: Sets how workflow worker deals with non-deterministic history events
	// (presumably arising from non-deterministic workflow definitions or non-backward compatible workflow
	// definition changes) and other panics raised from workflow code.
	// default: BlockWorkflow, which just logs error but doesn't fail workflow.
	// BlockWorkflow is the default policy for handling workflow panics and detected non-determinism.
	// This option causes workflow to get stuck in the workflow task retry loop.
	//
	// It is expected that after the problem is discovered and fixed the workflows are going to continue
	// without any additional manual intervention.
	// BlockWorkflow = 0
	// FailWorkflow immediately fails workflow execution if workflow code throws panic or detects non-determinism.
	// This feature is convenient during development.
	// WARNING: enabling this in production can cause all open workflows to fail on a single bug or bad deployment.
	// FailWorkflow = 1
	WorkflowPanicPolicy int `json:"workflow_panic_policy"`

	// Optional: worker graceful stop timeout
	// default: 0s
	WorkerStopTimeout int `json:"worker_stop_timeout"`

	// Optional: Enable running session workers.
	// Session workers is for activities within a session.
	// Enable this option to allow worker to process sessions.
	// default: false
	EnableSessionWorker bool `json:"enable_session_worker"`

	// Uncomment this option when we support automatic restablish failed sessions.
	// Optional: The identifier of the resource consumed by sessions.
	// It's the user's responsibility to ensure there's only one worker using this resourceID.
	// For now, if user doesn't specify one, a new uuid will be used as the resourceID.
	// SessionResourceID string

	// Optional: Sets the maximum number of concurrently running sessions the resource support.
	// default: 1000
	MaxConcurrentSessionExecutionSize int `json:"max_concurrent_session_execution_size"`

	// Optional: If set to true, a workflow worker is not started for this
	// worker and workflows cannot be registered with this worker. Use this if
	// you only want your worker to execute activities.
	// default: false
	DisableWorkflowWorker bool `json:"disable_workflow_worker"`

	// Optional: If set to true worker would only handle workflow tasks and local activities.
	// Non-local activities will not be executed by this worker.
	// default: false
	LocalActivityWorkerOnly bool `json:"local_activity_worker_only"`

	// Optional: If set overwrites the client level Identify value.
	// default: client identity
	Identity string `json:"identity"`

	// Optional: If set defines maximum amount of time that workflow task will be allowed to run. Defaults to 1 sec.
	DeadlockDetectionTimeout int `json:"deadlock_detection_timeout"`

	// Optional: The maximum amount of time between sending each pending heartbeat to the server. Regardless of
	// heartbeat timeout, no pending heartbeat will wait longer than this amount of time to send. To effectively disable
	// heartbeat throttling, this can be set to something like 1 nanosecond, but it is not recommended.
	// default: 60 seconds
	MaxHeartbeatThrottleInterval int `json:"max_heartbeat_throttle_interval"`

	// Optional: The default amount of time between sending each pending heartbeat to the server. This is used if the
	// ActivityOptions do not provide a HeartbeatTimeout. Otherwise, the interval becomes a value a bit smaller than the
	// given HeartbeatTimeout.
	// default: 30 seconds
	DefaultHeartbeatThrottleInterval int `json:"default_heartbeat_throttle_interval"`

	// Optional: Disable eager activities. If set to true, activities will not
	// be requested to execute eagerly from the same workflow regardless of
	// MaxConcurrentEagerActivityExecutionSize.
	//
	// Eager activity execution means the server returns requested eager
	// activities directly from the workflow task back to this worker which is
	// faster than non-eager which may be dispatched to a separate worker.
	//
	// Note: Eager activities will automatically be disabled if TaskQueueActivitiesPerSecond is set.
	DisableEagerActivities bool `json:"disable_eager_activities"`

	// Optional: Maximum number of eager activities that can be running.
	//
	// When non-zero, eager activity execution will not be requested for
	// activities schedule by the workflow if it would cause the total number of
	// running eager activities to exceed this value. For example, if this is
	// set to 1000 and there are already 998 eager activities executing and a
	// workflow task schedules 3 more, only the first 2 will request eager
	// execution.
	//
	// The default of 0 means unlimited and therefore only bound by
	// MaxConcurrentActivityExecutionSize.
	//
	// See DisableEagerActivities for a description of eager activity execution.
	MaxConcurrentEagerActivityExecutionSize int `json:"max_concurrent_eager_activity_execution_size"`

	// Optional: Disable allowing workflow and activity functions that are
	// registered with custom names from being able to be called with their
	// function references.
	//
	// Users are strongly recommended to set this as true if they register any
	// workflow or activity functions with custom names. By leaving this as
	// false, the historical default, ambiguity can occur between function names
	// and aliased names when not using string names when executing child
	// workflow or activities.
	DisableRegistrationAliasing bool `json:"disable_registration_aliasing"`

	// Assign a BuildID to this worker. This replaces the deprecated binary checksum concept,
	// and is used to provide a unique identifier for a set of worker code, and is necessary
	// to opt in to the Worker Versioning feature. See UseBuildIDForVersioning.
	// NOTE: Experimental
	BuildID string `json:"build_id"`

	// Optional: If set, opts this worker into the Worker Versioning feature. It will only
	// operate on workflows it claims to be compatible with. You must set BuildID if this flag
	// is true.
	// NOTE: Experimental
	// Note: Cannot be enabled at the same time as EnableSessionWorker
	UseBuildIDForVersioning bool `json:"use_build_id_for_versioning"`
}

type EvaluatorConfig struct {
	WorkflowName string `json:"workflow_name"`
	TaskQueue    string `json:"task_queue"`
}

type TemporalConfig struct {
	Host      string                 `json:"host"`
	Port      uint                   `json:"port"`
	Namespace string                 `json:"namespace"`
	TaskQueue string                 `json:"task_queue"`
	Worker    *TemporalWorkerOptions `json:"worker"`
	Evaluator *EvaluatorConfig       `json:"evaluator"`
}

func (tc *TemporalConfig) GetWorkerOptions() worker.Options {
	return worker.Options{
		MaxConcurrentActivityExecutionSize:      tc.Worker.MaxConcurrentActivityExecutionSize,
		WorkerActivitiesPerSecond:               tc.Worker.WorkerActivitiesPerSecond,
		MaxConcurrentLocalActivityExecutionSize: tc.Worker.MaxConcurrentLocalActivityExecutionSize,
		WorkerLocalActivitiesPerSecond:          tc.Worker.WorkerLocalActivitiesPerSecond,
		TaskQueueActivitiesPerSecond:            tc.Worker.TaskQueueActivitiesPerSecond,
		MaxConcurrentActivityTaskPollers:        tc.Worker.MaxConcurrentActivityTaskPollers,
		MaxConcurrentWorkflowTaskExecutionSize:  tc.Worker.MaxConcurrentWorkflowTaskExecutionSize,
		MaxConcurrentWorkflowTaskPollers:        tc.Worker.MaxConcurrentWorkflowTaskPollers,
		EnableLoggingInReplay:                   tc.Worker.EnableLoggingInReplay,
		StickyScheduleToStartTimeout:            time.Duration(tc.Worker.StickyScheduleToStartTimeout) * time.Second,
		WorkflowPanicPolicy:                     worker.WorkflowPanicPolicy(tc.Worker.WorkflowPanicPolicy),
		WorkerStopTimeout:                       time.Duration(tc.Worker.WorkerStopTimeout) * time.Second,
		EnableSessionWorker:                     tc.Worker.EnableSessionWorker,
		MaxConcurrentSessionExecutionSize:       tc.Worker.MaxConcurrentSessionExecutionSize,
		DisableWorkflowWorker:                   tc.Worker.DisableWorkflowWorker,
		Identity:                                tc.Worker.Identity,
		DeadlockDetectionTimeout:                time.Duration(tc.Worker.DeadlockDetectionTimeout) * time.Second,
		MaxHeartbeatThrottleInterval:            time.Duration(tc.Worker.MaxHeartbeatThrottleInterval) * time.Second,
		DefaultHeartbeatThrottleInterval:        time.Duration(tc.Worker.DefaultHeartbeatThrottleInterval) * time.Second,
		DisableEagerActivities:                  tc.Worker.DisableEagerActivities,
		MaxConcurrentEagerActivityExecutionSize: tc.Worker.MaxConcurrentEagerActivityExecutionSize,
		DisableRegistrationAliasing:             tc.Worker.DisableRegistrationAliasing,
		BuildID:                                 tc.Worker.BuildID,
		UseBuildIDForVersioning:                 tc.Worker.UseBuildIDForVersioning,
		// we always use ExecuteActivity to ensure any available worker handle the job
		LocalActivityWorkerOnly: false,
	}
}

type RPCConfig struct {
	Urls             []string `json:"urls"`
	Retries          int      `json:"retries"`
	MinBackoff       int      `json:"min_backoff"`
	MaxBackoff       int      `json:"max_backoff"`
	ReqPerSec        int      `json:"req_per_sec"`
	SessionTolerance int64    `json:"session_tolerance"`
	RelayPerSession  int64    `json:"relay_per_session"`
	BlockInterval    int64    `json:"block_interval"`
}

type Config struct {
	MongodbUri string          `json:"mongodb_uri"`
	Apps       []string        `json:"apps"`
	Rpc        *RPCConfig      `json:"rpc"`
	LogLevel   string          `json:"log_level"`
	Temporal   *TemporalConfig `json:"temporal"`
}

// UnmarshalJSON implement the Unmarshaler interface on Config
func (c *Config) UnmarshalJSON(b []byte) error {
	// We create an alias for the Config type to avoid recursive calls to the UnmarshalJSON method
	type Alias Config
	defaultValues := &Alias{
		MongodbUri: DefaultMongodbUri,
		Apps:       []string{},
		Rpc:        &DefaultRpc,
		LogLevel:   DefaultLogLevel,
		Temporal:   &DefaultTemporal,
	}

	if err := json.Unmarshal(b, &defaultValues); err != nil {
		return err
	}

	// If the incoming JSON has values, it will override the defaults. If not, the default values will be used.
	*c = Config(*defaultValues)

	return nil
}
