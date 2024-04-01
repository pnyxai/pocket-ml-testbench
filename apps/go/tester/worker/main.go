package main

import (
	"fmt"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"tester/activities"
	"tester/app"
	"tester/workflows"
)

func main() {
	// Initialize application things like logger/configs/etc
	ac := app.Initialize()
	// logger with tagged as worker
	logger := ac.GetLoggerByComponent("worker")
	// Initialize ac Temporal Client
	// Specify the Namespace in the Client options
	clientOptions := client.Options{
		HostPort:  fmt.Sprintf("%s:%d", ac.Config.Temporal.Host, ac.Config.Temporal.Port),
		Namespace: ac.Config.Temporal.Namespace,
		Logger: &app.TemporalLogger{
			Logger: &logger,
		},
	}
	temporalClient, err := client.Dial(clientOptions)
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to create ac Temporal Client")
	}
	defer temporalClient.Close()

	// Create ac new Worker
	w := worker.New(temporalClient, ac.Config.Temporal.TaskQueue, worker.Options{})

	// Register Workflows
	workflows.Workflows.Register(w)

	// Register Activities
	activities.Activities.Register(w)

	// Start the Worker Process
	err = w.Run(worker.InterruptCh())
	if err != nil {
		logger.Fatal().Err(err).Msg("unable to start the Worker Process")
	}
}
