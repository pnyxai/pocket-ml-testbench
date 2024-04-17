package mongodb

import (
	"context"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/url"
	"time"
)

type Collections map[string]*mongo.Collection

type MongoDb struct {
	Uri         string
	Client      *mongo.Client
	Collections Collections
	Logger      *zerolog.Logger
}

func (m *MongoDb) GetDatabaseName(uri string, defaultName string) string {
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

func (m *MongoDb) GetCollection(name string) *mongo.Collection {
	coll := m.Collections[name]
	if coll == nil {
		panic("collection " + name + " not exists")
	}
	return coll
}

func (m *MongoDb) CloseConnection() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if err := m.Client.Disconnect(ctx); err != nil {
			m.Logger.Fatal().Err(err).Msg("Errored closing MongoDB connection")
		}
		m.Logger.Info().Msg("MongoDB connection successfully closed.")
	}()
}

func Initialize(uri string, collections []string, l *zerolog.Logger) *MongoDb {
	m := MongoDb{
		Uri:         uri,
		Client:      nil,
		Collections: make(map[string]*mongo.Collection),
		Logger:      l,
	}
	// Set client options
	clientOptions := options.Client().ApplyURI(uri)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)

	if err != nil {
		l.Fatal().Err(err).Msg("error creating mongodb client connection")
	}

	// Check the connection
	err = client.Ping(ctx, nil)
	if err != nil {
		l.Fatal().Err(err).Msg("error pinging mongodb server")
	}

	l.Info().Msg("Connected to MongoDB!")

	m.Client = client

	_db := client.Database(m.GetDatabaseName(uri, "test"))
	for _, collectionName := range collections {
		collection := _db.Collection(collectionName)
		if e := _db.CreateCollection(ctx, collectionName); e != nil {
			if err.(mongo.CommandError).Name != "NamespaceExists" {
				l.Fatal().Err(err).Str("collection", collectionName).Msg("Errored preparing collection")
			}
		}

		m.Collections[collectionName] = collection
	}

	return &m
}
