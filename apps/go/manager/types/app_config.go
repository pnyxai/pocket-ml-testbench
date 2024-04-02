package types

import (
	"packages/mongodb"

	"github.com/rs/zerolog"
)

type App struct {
	Logger  *zerolog.Logger
	Config  *Config
	Mongodb *mongodb.MongoDb
}
