package common

import (
	"context"
	poktGoSdk "github.com/pokt-foundation/pocket-go/provider"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.temporal.io/sdk/temporal"
	"packages/mongodb"
	"strconv"
)

func GetBlocksPerSession(params *poktGoSdk.AllParams) (int64, error) {
	blocksPerSessionStr, ok := params.NodeParams.Get("pos/BlocksPerSession")
	if !ok {
		return 0, temporal.NewApplicationError("unable to get pos/BlocksPerSession from block params", "GetBlockParam")
	}
	blocksPerSession, parseIntErr := strconv.ParseInt(blocksPerSessionStr, 10, 64)
	if parseIntErr != nil {
		return 0, temporal.NewApplicationErrorWithCause("unable to parse to int the value provided by pos/BlocksPerSession from block params", "ParseInt", parseIntErr, blocksPerSessionStr)
	}
	return blocksPerSession, nil
}

func GetRecord[T interface{}](ctx context.Context, collection mongodb.CollectionAPI, filter interface{}, opts ...*options.FindOneOptions) (doc *T, e error) {
	err := collection.FindOne(ctx, filter, opts...).Decode(&doc)
	if err != nil {
		e = err
		return
	}
	return
}

func GetRecords[T interface{}](ctx context.Context, collection mongodb.CollectionAPI, filter interface{}, opts ...*options.FindOptions) (docs []*T, e error) {
	cursor, err := collection.Find(ctx, filter, opts...)
	if err != nil {
		e = err
		return
	}
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		er := cursor.Close(ctx)
		if er != nil {
		}
	}(cursor, ctx)
	if e = cursor.All(context.Background(), &docs); e != nil {
		return nil, e
	}
	return
}
