package logger

import (
	"github.com/rs/zerolog"
	"packages/utils"
)

// ZerologAdapter represents an adapter for the Zerolog logging library.
// This adapter allows to use zerolog in one of the following ways:
//
// logger := app.GetActivityLogger(ActivityName, ctx, params)
// logger := app.GetActivityLogger(ActivityName, ctx, nil)
// logger := app.GetWorkflowLogger(WorkflowName, ctx, params)
// logger := app.GetWorkflowLogger(WorkflowName, ctx, nil)
//
// so the logger will or not have additional context properties (params) plus name and type [activity or workflow]
// and these are the ways you can use it on your activity or workflow
// logger.Info("freaking awesome...")
// logger.Info("freaking awesome...", app.Fields{"foo": 1, "bar": ""})
// logger.InfoEvent().
//
//	Int("foo", 1).
//	Str("bar", "").
//	Msg("freaking awesome...")
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

// InfoEvent returns a pointer to a zerolog.Event for logging info messages.
func (zl *ZerologAdapter) InfoEvent() *zerolog.Event {
	return zl.logger.Info()
}

// Debug This is the adapter function for zerolog's Debug().
func (zl *ZerologAdapter) Debug(msg string, kvs ...interface{}) {
	zl.logger.Debug().Fields(utils.KeyValToMap(kvs...)).Msg(msg)
}

// DebugEvent returns a pointer to a zerolog.Event for logging debug messages.
func (zl *ZerologAdapter) DebugEvent() *zerolog.Event {
	return zl.logger.Debug()
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

// ErrorEvent returns a pointer to a zerolog.Event for logging error messages.
func (zl *ZerologAdapter) ErrorEvent() *zerolog.Event {
	return zl.logger.Error()
}

// Warn This is the adapter function for zerolog's Warn().
func (zl *ZerologAdapter) Warn(msg string, kvs ...interface{}) {
	zl.logger.Warn().Fields(utils.KeyValToMap(kvs...)).Msg(msg)
}

// WarnEvent returns a pointer to a zerolog.Event for logging warning messages.
func (zl *ZerologAdapter) WarnEvent() *zerolog.Event {
	return zl.logger.Warn()
}

// WithContext This method creates a new ZerologAdapter with additional context fields.
// The "name" parameter specifies the name of the context.
// The "componentType" parameter specifies the type of the component (e.g., "activity", "workflow").
// The "params" parameter is optional and can be used to add additional key-value pairs to the context.
// The method returns a new ZerologAdapter instance with the specified context fields.
func (zl *ZerologAdapter) WithContext(name, componentType string, params ...interface{}) *ZerologAdapter {
	lCtx := zl.logger.With().
		Str("name", name).
		Str("component", componentType)

	if params != nil {
		lCtx = lCtx.Fields(utils.KeyValToMap(params...))
	}

	return &ZerologAdapter{logger: lCtx.Logger()}
}
