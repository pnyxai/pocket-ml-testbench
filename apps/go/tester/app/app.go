package app

import (
	"github.com/rs/zerolog"
	"tester/activities"
	"tester/workflows"
)

type App struct {
	Logger *zerolog.Logger
	Config *Config
	// todo: implement here the connection to postgres
	Postgres interface{}
}

func Initialize() *App {
	// Get App config
	config := LoadConfigFile()
	// initialize logger
	logger := InitLogger(config)
	// todo: initialize postgresql connection to read sampler requests
	ac := &App{
		Logger: logger,
		Config: config,
	}

	// set this to workflows and activities to avoid use of context.Context
	workflows.SetAppConfig(ac)
	activities.SetAppConfig(ac)

	return ac
}
