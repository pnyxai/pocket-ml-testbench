package main

import (
	"fmt"
	"manager/activities"
	"packages/logger"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"manager/workflows"
	"manager/x"
)

func main() {

	// Initialize application things like logger/configs/etc
	ac := x.Initialize()

	// Initialize Temporal Client
	// using the provided namespace and logger
	clientOptions := client.Options{
		HostPort:  fmt.Sprintf("%s:%d", ac.Config.Temporal.Host, ac.Config.Temporal.Port),
		Namespace: ac.Config.Temporal.Namespace,
		Logger:    logger.NewZerologAdapter(*ac.Logger),
	}
	// Connect to Temporal server
	temporalClient, err := client.Dial(clientOptions)
	if err != nil {
		ac.Logger.Fatal().Err(err).Msg("unable to create ac Temporal Client")
	}
	defer temporalClient.Close()

	// Create new Temporal worker
	w := worker.New(temporalClient, ac.Config.Temporal.TaskQueue, worker.Options{})

	// Register Workflows
	workflows.Workflows.Register(w)

	// Register Activities
	activities.Activities.Register(w)

	// Start the Worker Process
	err = w.Run(worker.InterruptCh())
	if err != nil {
		ac.Logger.Fatal().Err(err).Msg("unable to start the Worker Process")
	}
}
