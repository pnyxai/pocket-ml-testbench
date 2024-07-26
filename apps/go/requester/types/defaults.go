package types

var (
	DefaultMongodbUri            = "mongodb://127.0.0.1:27017/pocket-ml-testbench?replicaSet=devRs"
	DefaultLogLevel              = "info"
	DefaultTemporalHost          = "localhost"
	DefaultTemporalPort          = uint(7233)
	DefaultTemporalNamespace     = "pocket-ml-testbench"
	DefaultTemporalTaskQueue     = "requester"
	DefaultEvaluatorWorkflowName = "evaluator"
	DefaultEvaluatorTaskQueue    = "evaluator"
	DefaultRpcRetries            = 3
	DefaultMinBackoff            = 10
	DefaultMaxBackoff            = 60
	DefaultReqPerSec             = 10
	DefaultSessionTolerance      = int64(1)
	DefaultRelayPerSession       = int64(200)
	DefaultBlockInterval         = int64(30)
	DefaultRpc                   = RPCConfig{
		Urls:             []string{},
		Retries:          DefaultRpcRetries,
		MinBackoff:       DefaultMinBackoff,
		MaxBackoff:       DefaultMaxBackoff,
		ReqPerSec:        DefaultReqPerSec,
		SessionTolerance: DefaultSessionTolerance,
		RelayPerSession:  DefaultRelayPerSession,
		BlockInterval:    DefaultBlockInterval,
	}
	DefaultTemporal = TemporalConfig{
		Host:      DefaultTemporalHost,
		Port:      DefaultTemporalPort,
		Namespace: DefaultTemporalNamespace,
		TaskQueue: DefaultTemporalTaskQueue,
		Evaluator: &EvaluatorConfig{
			WorkflowName: DefaultEvaluatorWorkflowName,
			TaskQueue:    DefaultEvaluatorTaskQueue,
		},
		Worker: &TemporalWorkerOptions{},
	}
)
