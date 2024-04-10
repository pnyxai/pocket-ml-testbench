package mongodb

import (
	"context"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

func Initialize(uri string, l *zerolog.Logger) (client *mongo.Client) {
	var err error
	// Set client options
	clientOptions := options.Client().ApplyURI(uri)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to MongoDB
	client, err = mongo.Connect(ctx, clientOptions)

	if err != nil {
		l.Fatal().Err(err).Msg("error creating mongodb client connection")
	}

	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		l.Fatal().Err(err).Msg("error pinging mongodb server")
	}

	l.Info().Msg("Connected to MongoDB!")

	return
}
