package logger

import (
	"context"
	"encoding/json"
	"github.com/iancoleman/strcase"
	"github.com/rs/zerolog/log"
	"go.temporal.io/sdk/activity"
	temporalLogger "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/workflow"
)

type Fields map[string]interface{}

func (f Fields) GetLoggerFields() []interface{} {
	loggerFields := make([]interface{}, 0)
	for k, v := range f {
		loggerFields = append(loggerFields, strcase.ToCamel(k))
		loggerFields = append(loggerFields, v)
	}
	return loggerFields
}

func NewFieldsFromStruct(s interface{}) *Fields {
	var fields Fields

	if v, ok := s.(*Fields); ok {
		return v
	}

	data, err := json.Marshal(s)
	if err != nil {
		log.Fatal().Err(err)
	}
	err = json.Unmarshal(data, &fields)
	if err != nil {
		log.Fatal().Err(err)
	}
	return &fields
}

func GetActivityLogger(name string, ctx context.Context, params interface{}) temporalLogger.Logger {
	return temporalLogger.With(activity.GetLogger(ctx), NewFieldsFromStruct(params).GetLoggerFields()...)
}

func GetWorkflowLogger(name string, ctx workflow.Context, params interface{}) temporalLogger.Logger {
	return temporalLogger.With(workflow.GetLogger(ctx), NewFieldsFromStruct(params).GetLoggerFields()...)
}
