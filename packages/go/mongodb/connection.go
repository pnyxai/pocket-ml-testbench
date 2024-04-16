package mongodb

import (
	"context"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/url"
	"time"
)

func GetDatabaseName(uri string, defaultName string) string {
	u, err := url.Parse(uri)
	if err != nil {
		panic(err)
	}

	// The database name resides in the path, trim the leading slash
	dbName := u.Path[1:]

	if dbName == "" {
		return defaultName
	}

	return dbName
}

func Initialize(uri string, collections []string, l *zerolog.Logger) (client *mongo.Client, collectionsMap map[string]*mongo.Collection) {
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

	_db := client.Database(GetDatabaseName(uri, "test"))
	collectionsMap = make(map[string]*mongo.Collection)
	for _, collectionName := range collections {
		collection := _db.Collection(collectionName)
		if e := _db.CreateCollection(ctx, collectionName); e != nil {
			if err.(mongo.CommandError).Name != "NamespaceExists" {
				l.Fatal().Err(err).Str("collection", collectionName).Msg("Errored preparing collection")
			}
		}

		collectionsMap[collectionName] = collection
	}

	return
}

func CloseConnection(client *mongo.Client, l *zerolog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			l.Fatal().Err(err).Msg("Errored closing MongoDB connection")
		}
		l.Info().Msg("MongoDB connection successfully closed.")
	}()
}
