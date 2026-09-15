package types

import (
	"packages/pocket"
)

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
	// Matches pocket.DefaultSenderTimeout, so leaving it unset changes nothing.
	DefaultMaxRelayTimeout = int64(1800)
	DefaultRpc             = "http://localhost:26657"
	DefaultGRpc            = pocket.GRPCConfig{
		HostPort: "localhost:9090",
		Insecure: true,
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
