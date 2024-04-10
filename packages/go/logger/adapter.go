package logger

import (
	"github.com/rs/zerolog"
	"packages/utils"
)

// ZerologAdapter represents an adapter for the Zerolog logging library.
// This adapter allows to use zerolog in one of the following ways:
//
// l := app.GetActivityLogger(ActivityName, ctx, logger.NewFieldsFromStruct(params))
// l := app.GetActivityLogger(ActivityName, ctx, nil)
// l := app.GetWorkflowLogger(WorkflowName, ctx, logger.NewFieldsFromStruct(params))
// l := app.GetWorkflowLogger(WorkflowName, ctx, nil)
//
// so the logger will or not have additional context properties (params)
// and these are the ways you can use it on your activity or workflow
// l.Info("foo", logger.NewFieldsFromStruct(logger.Fields{"foo": "bar"}).GetLoggerFields()...)
// l.Info("foo", logger.NewFieldsFromStruct(anyStructInstance).GetLoggerFields()...)
// l.Info("bar", "foo", "1", "bar", 1)
type ZerologAdapter struct {
	logger zerolog.Logger
}

// NewZerologAdapter creates a new instance of ZerologAdapter with the provided logger.
// It takes a zerolog.Logger as input and returns a pointer to the ZerologAdapter.
func NewZerologAdapter(logger zerolog.Logger) *ZerologAdapter {
	return &ZerologAdapter{logger: logger}
}

// Info This is the adapter function for zerolog's Info().
func (zl *ZerologAdapter) Info(msg string, kvs ...interface{}) {
	zl.logger.Info().Fields(utils.KeyValToMap(kvs...)).Msg(msg)
}

// Debug This is the adapter function for zerolog's Debug().
func (zl *ZerologAdapter) Debug(msg string, kvs ...interface{}) {
	zl.logger.Debug().Fields(utils.KeyValToMap(kvs...)).Msg(msg)
}

// Error This is the adapter function for zerolog's Error().
func (zl *ZerologAdapter) Error(msg string, kvs ...interface{}) {
	var e error

	f := utils.KeyValToMap(kvs...)

	if err, ok := f["error"].(error); ok {
		e = err
		delete(f, "error")
	}
	if err, ok := f["err"].(error); ok {
		e = err
		delete(f, "err")
	}

	logEvt := zl.logger.Error()
	if e != nil {
		logEvt = logEvt.Err(e)
	}

	logEvt.Fields(f).Msg(msg)
}

// Warn This is the adapter function for zerolog's Warn().
func (zl *ZerologAdapter) Warn(msg string, kvs ...interface{}) {
	zl.logger.Warn().Fields(utils.KeyValToMap(kvs...)).Msg(msg)
}
