package logger

import (
	"context"
	"errors"
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/activity"
	temporalLogger "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

type Fields map[string]interface{}

// GetZerologAdapter retrieves the ZerologAdapter from the provided temporalLogger.Logger.
// It performs a type assertion to check if the logger is of type *ZerologAdapter.
// If the assertion is successful, it returns the *ZerologAdapter.
// If the assertion fails, it terminates the program with a fatal log message indicating that the logger is not a zerolog adapter.
func GetZerologAdapter(l temporalLogger.Logger) *ZerologAdapter {
	// Type assertion
	extLogger, ok := l.(*ZerologAdapter)

	if !ok {
		// Handle the case where the logger is not an ExtendedLogger
		log.Fatal().
			Err(errors.New("logger is not a zerolog adapter")).
			Msg("unable to get activity logger")
	}

	return extLogger
}

// GetActivityLogger retrieves the ZerologAdapter from the provided context
// and creates a new logger with the specified name, and additional parameters (if any)
func GetActivityLogger(name string, ctx context.Context, params ...interface{}) *ZerologAdapter {
	logger := GetZerologAdapter(activity.GetLogger(ctx))
	return logger.WithContext(name, "activity", params)
}

// GetWorkflowLogger retrieves the ZerologAdapter from the provided context
// and creates a new logger with the specified name, and additional parameters (if any)
func GetWorkflowLogger(name string, ctx workflow.Context, params ...interface{}) *ZerologAdapter {
	logger := GetZerologAdapter(workflow.GetLogger(ctx))
	return logger.WithContext(name, "workflow", params)
}
