package main

import (
	"github.com/rs/zerolog"
	"go.temporal.io/sdk/worker"
	"requester/activities"
	"requester/workflows"
	"requester/x"
)

func main() {
	// Initialize application things like logger/configs/etc
	ac := x.Initialize()

	defer ac.TemporalClient.Close()
	defer ac.Mongodb.CloseConnection()

	// Create ac new Worker
	w := worker.New(ac.TemporalClient, ac.Config.Temporal.TaskQueue, worker.Options{
		// turn on replay logs only when debug level is on
		EnableLoggingInReplay: ac.Logger.GetLevel() == zerolog.DebugLevel,
	})

	// Register Workflows
	workflows.Workflows.Register(w)

	// Register Activities
	activities.Activities.Register(w)

	// Start the Worker Process
	err := w.Run(worker.InterruptCh())
	if err != nil {
		ac.Logger.Fatal().Err(err).Msg("unable to start the Worker Process")
	}
}
