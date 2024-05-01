package mongodb

import (
	"context"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/url"
	"time"
)

type MongoDb interface {
	GetDatabaseName(uri string, defaultName string) string
	GetCollection(name string) CollectionAPI
	StartSession(opts ...*options.SessionOptions) (mongo.Session, error)
	CloseConnection()
}

type Client struct {
	Uri         string
	Client      *mongo.Client
	Collections *xsync.MapOf[string, CollectionAPI]
	Logger      *zerolog.Logger
}

func (m *Client) GetDatabaseName(uri string, defaultName string) string {
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

func (m *Client) GetCollection(name string) CollectionAPI {
	if coll, ok := m.Collections.Load(name); !ok {
		panic("collection " + name + " not exists")
	} else {
		return coll
	}
}

func (m *Client) StartSession(opts ...*options.SessionOptions) (mongo.Session, error) {
	return m.Client.StartSession(opts...)
}

func (m *Client) CloseConnection() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer func() {
		if err := m.Client.Disconnect(ctx); err != nil {
			m.Logger.Fatal().Err(err).Msg("Errored closing MongoDB connection")
		}
		m.Logger.Info().Msg("MongoDB connection successfully closed.")
	}()
}

func NewClient(uri string, collections []string, l *zerolog.Logger) MongoDb {
	m := Client{
		Uri:         uri,
		Client:      nil,
		Collections: xsync.NewMapOf[string, CollectionAPI](),
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
		m.Collections.Store(collectionName, &Collection{collection: collection})
	}

	return &m
}
