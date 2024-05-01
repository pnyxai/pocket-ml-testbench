package mongodb

import (
	"context"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MockSession struct {
	mongo.Session
	mock.Mock
}

func (ms *MockSession) EndSession(ctx context.Context) {
	ms.Called(ctx)
	return
}

func (ms *MockSession) WithTransaction(ctx context.Context, fn func(ctx mongo.SessionContext) (interface{}, error),
	opts ...*options.TransactionOptions) (interface{}, error) {
	args := ms.Called(ctx, fn, opts)
	v := args.Get(0)
	if transactionHandler, ok := v.(func(ctx mongo.SessionContext) (interface{}, error)); ok {
		return transactionHandler(mongo.NewSessionContext(ctx, ms))
	} else {
		panic("first argument must be a func(ctx mongo.SessionContext) (interface{}, error)")
	}
}
