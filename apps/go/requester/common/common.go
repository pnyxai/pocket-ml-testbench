package common

import (
	"context"
	"packages/mongodb"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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
