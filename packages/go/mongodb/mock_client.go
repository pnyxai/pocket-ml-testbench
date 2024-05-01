package mongodb

import (
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MockClient struct {
	mock.Mock
	Client
	Uri         string
	Collections *xsync.MapOf[string, CollectionAPI]
	Logger      *zerolog.Logger
}

func (mc *MockClient) GetDatabaseName(uri string, defaultName string) string {
	// mock this if needed
	args := mc.Called(uri, defaultName)
	return args.String(0)
}

func (mc *MockClient) GetCollection(name string) (response CollectionAPI) {
	// this is the best point to mock and return a MockCollection
	args := mc.Called(name)
	firstResponseArg := args.Get(0)

	if firstResponseArg != nil {
		if v, ok := firstResponseArg.(CollectionAPI); !ok {
			panic(UnexpectedType)
		} else {
			response = v
		}
	}

	return
}

func (mc *MockClient) StartSession(opts ...*options.SessionOptions) (mongo.Session, error) {
	args := mc.Called(opts)
	return args.Get(0).(mongo.Session), args.Error(1)
}

func NewMockClient(uri string, l *zerolog.Logger) *MockClient {
	// will not handle collection or client because how use the Mongodb interface instance should
	// never need to access the client directly.
	// if that becomes a use case, this will need to be revisited or the code that does that
	return &MockClient{
		Uri:    uri,
		Logger: l,
	}
}
