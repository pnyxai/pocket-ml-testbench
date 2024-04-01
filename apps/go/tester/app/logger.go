package app

import (
	"fmt"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/url"
	"os"
	"time"
)

// LogFields represents a mapping of string keys to generic interface{} values. It is commonly used in log entry
// data structures to store additional contextual information. The keys should be descriptive names for the
// corresponding values stored. The values can be of any type and can be dynamically added or modified.
type LogFields map[string]interface{}

type LevelHTTPLogger struct {
	retryablehttp.LeveledLogger
}

// Fields create a map of key/value fields based on variadic input parameters.
// The parameters are considered in pairs, where the odd-indexed parameters
// are keys (as strings) and even-indexed parameters are values (of any type).
// The function returns a map[string]interface{} containing the created fields.
func fields(keysAndValues ...interface{}) map[string]interface{} {
	f := make(map[string]interface{})

	for i := 0; i < len(keysAndValues)-1; i += 2 {
		f[keysAndValues[i].(string)] = keysAndValues[i+1]
	}

	return f
}

// addFields - adds key/value fields to a zerolog event object
func addFields(ev *zerolog.Event, f map[string]interface{}) *zerolog.Event {
	if len(f) > 0 {
		for k, v := range f {
			ev = ev.Interface(k, v)
		}
	}

	return ev
}

// Error logs an error message with optional key-value pairs.
// The error message is obtained from the provided `msg` string.
// The `keysAndValues` parameter allows for specifying additional details in key-value format.
//
// If an `url` key is present in the provided key-value pairs, it will attempt to cast the corresponding value
// to an `*url.URL` type. If the casting is successful, it will log the error message with the following fields:
// - "method" (string): the HTTP method specified in the key-value pairs
// - "scheme" (string): the URL scheme specified in the key-value pairs
// - "host" (string): the URL host specified in the key-value pairs
// - "path" (string): the URL path specified in the key-value pairs
// Otherwise, if the casting fails, it will log the error message with the following fields:
// - "error" (error): the error value specified in the key-value pairs
// - "url" (interface{}): the URL value specified in the key-value pairs
//
// If no `url` key is present in the provided key-value pairs, it will log the error message with the following fields:
// - "error" (error): the error value specified in the key-value pairs
//
// Example usage:
//
//	logger := &LevelHTTPLogger{}
//	logger.Error("unable to process request", "error", err, "url", &_url)
//	// Output: {"level":"error","error":"unable to process request","url":"https://example.com","method":"POST","scheme":"https","host":"example.com","path":"/api"}
func (l *LevelHTTPLogger) Error(msg string, keysAndValues ...interface{}) {
	f := fields(keysAndValues...)
	err := f["error"].(error)
	_url := f["url"]
	if _url != nil {
		_url2, ok := _url.(*url.URL)
		if !ok {
			log.Logger.Error().Err(err).Interface("url", _url2).Msg("request error")
			return
		}

		log.Logger.Error().
			Str("method", f["method"].(string)).
			Str("scheme", _url2.Scheme).
			Str("host", _url2.Host).
			Str("path", _url2.Path).
			Msg(msg)

		return
	}
	addFields(log.Logger.Error(), f).Err(err).Msg(msg)
}

// Info logs a message at info level
func (l *LevelHTTPLogger) Info(msg string, keysAndValues ...interface{}) {
	addFields(log.Logger.Info(), fields(keysAndValues...)).Msg(msg)
}

// Debug logs a message at debug level
func (l *LevelHTTPLogger) Debug(msg string, keysAndValues ...interface{}) {
	f := fields(keysAndValues...)
	_url := f["url"]
	if _url != nil {
		_url2, ok := _url.(*url.URL)
		if !ok {
			log.Logger.Error().Msg(fmt.Sprintf("unable to cast to url.URL %v", _url))
			return
		}
		log.Logger.Debug().
			Str("method", f["method"].(string)).
			Str("scheme", _url2.Scheme).
			Str("host", _url2.Host).
			Str("path", _url2.Path).
			Str("query", _url2.RawQuery).
			Msg(msg)

		return
	}

	addFields(log.Logger.Debug(), f).Msg(msg)
}

// Warn logs a warning message with optional key-value pairs.
// The warning message is obtained from the provided `msg` string.
// The `keysAndValues` parameter allows for specifying additional details in key-value format.
// The warning message is logged with the following fields:
// - "error" (error): the error value specified in the key-value pairs
// Example usage:
//
//	logger := &LevelHTTPLogger{}
//	logger.Warn("connection failure", "error", err)
//	// Output: {"level":"debug","error":"connection failure"}
func (l *LevelHTTPLogger) Warn(msg string, keysAndValues ...interface{}) {
	addFields(log.Logger.Debug(), fields(keysAndValues...)).Msg(msg)
}

type TemporalLogger struct {
	Logger *zerolog.Logger
}

// Error a log message at error level
func (l *TemporalLogger) Error(msg string, keysAndValues ...interface{}) {
	f := fields(keysAndValues...)
	ev := addFields(log.Logger.Error(), f)
	if err, ok := f["error"].(error); ok {
		ev = ev.Err(err)
	}
	ev.Msg(msg)
}

// Info a log message at info level
func (l *TemporalLogger) Info(msg string, keysAndValues ...interface{}) {
	addFields(log.Logger.Info(), fields(keysAndValues...)).Msg(msg)
}

// Debug a log message at debug level
func (l *TemporalLogger) Debug(msg string, keysAndValues ...interface{}) {
	addFields(log.Logger.Debug(), fields(keysAndValues...)).Msg(msg)
}

// Warn a log message at warning level
func (l *TemporalLogger) Warn(msg string, keysAndValues ...interface{}) {
	addFields(log.Logger.Warn(), fields(keysAndValues...)).Msg(msg)
}

// InitLogger - initialize logger
func InitLogger(config *Config) *zerolog.Logger {
	lvl := zerolog.InfoLevel

	if config.LogLevel != "" {
		if l, err := zerolog.ParseLevel(config.LogLevel); err != nil {
			log.Fatal().Err(err).Msg("unable to parse log_level value")
		} else {
			lvl = l
		}
	}

	log.Logger.Level(lvl)

	ctx := zerolog.New(
		zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
		},
	).Level(lvl).With().Timestamp()

	if lvl >= zerolog.DebugLevel {
		ctx = ctx.Caller()
	}

	logger := ctx.Logger()

	zerolog.TimestampFieldName = "t"
	zerolog.MessageFieldName = "msg"
	zerolog.LevelFieldName = "lvl"

	return &logger
}

// GetLoggerByComponent - return a logger instance with a modified context that will attach the retrieved name as component tag
func (app *App) GetLoggerByComponent(name string) zerolog.Logger {
	return app.Logger.With().Str("component", name).Logger()
}
