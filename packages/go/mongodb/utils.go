package mongodb

import (
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"reflect"
)

func GetDocFromBsonSetUpdateOperation[T interface{}](update interface{}) (*T, error) {
	tType := reflect.TypeOf((*T)(nil)).Elem()
	if updateDoc, ok := update.(bson.M); !ok || updateDoc["$set"] == nil {
		return nil, errors.New("response collection call UpdateOne without $set operation")
	} else if responseDoc, okDoc := updateDoc["$set"].(*T); !okDoc {
		return nil, fmt.Errorf("$set document is not a pointer of the expected type %s", tType)
	} else {
		return responseDoc, nil
	}
}
