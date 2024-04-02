package types

import (
	"github.com/rs/zerolog"
)

type App struct {
	Logger *zerolog.Logger
	Config *Config
	// todo: implement here the connection to postgres
	Postgres interface{}
	Mongodb  interface{}
}
