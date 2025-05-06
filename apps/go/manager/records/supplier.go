package records

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"manager/types"
	"packages/mongodb"
	"time"

	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//------------------------------------------------------------------------------
// SupplierRecord
//------------------------------------------------------------------------------

// DB entry of a given supplier-service pair
// The "Tasks" array will hold as many entries as tasks being tested
type SupplierRecord struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	Address        string             `bson:"address"`
	Service        string             `bson:"service"`
	LastSeenHeight int64              `bson:"last_seen_height"`
	LastSeenTime   time.Time          `bson:"last_seen_time"`
	LastPoolHeight int64              `bson:"last_pool_height"`
	LastPoolTime   time.Time          `bson:"last_pool_time"`
}

func (record *SupplierRecord) FindAndLoadSupplier(supplier types.SupplierData, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	// Get suppliers collection
	suppliersCollection := mongoDB.GetCollection(types.SuppliersCollection)

	// Set filtering for this supplier-service pair data
	supplier_filter := bson.D{{Key: "address", Value: supplier.Address}, {Key: "service", Value: supplier.Service}}
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Retrieve this supplier entry
	var found bool = true
	cursor := suppliersCollection.FindOne(ctxM, supplier_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("address", supplier.Address).Str("service", supplier.Service).Msg("Supplier entry not found (FindAndLoadSupplier).")
			found = false
			// }else if err == mongo. {
			return found, nil
		} else {
			l.Error().Err(err).Str("address", supplier.Address).Str("service", supplier.Service).Msg("Could not retrieve supplier data from MongoDB (FindAndLoadSupplier).")
			return false, err
		}
	}

	return found, nil
}

func (record *SupplierRecord) AppendTask(supplierID primitive.ObjectID, framework string, task string, date time.Time, frameworkConfigMap map[string]types.FrameworkConfig, mongoDB mongodb.MongoDb, l *zerolog.Logger) TaskInterface {

	taskType, err := GetTaskType(framework, task, frameworkConfigMap, l)
	if err != nil {
		return nil
	}
	// Get the task, wich will create it if not found
	taskRecord, found := GetTaskData(supplierID, taskType, framework, task, mongoDB, l)
	if !found {
		return nil
	} else {
		return taskRecord
	}

}

func (record *SupplierRecord) Init(params types.AnalyzeSupplierParams, frameworkConfigMap map[string]types.FrameworkConfig, mongoDB mongodb.MongoDb, l *zerolog.Logger) error {
	// Initialize empty record

	// Set supplier data
	record.Address = params.Supplier.Address
	record.Service = params.Supplier.Service

	// Create a hash of the strings
	hash := sha256.New()
	hash.Write([]byte(record.Address))
	hash.Write([]byte(record.Service))
	hashBytes := hash.Sum(nil)

	// Convert the hash to a hexadecimal string
	hashHex := hex.EncodeToString(hashBytes)

	// Convert the hexadecimal string to a primitive.ObjectID
	// We'll only take the first 24 characters of the hash (which is 12 bytes)
	hashObjectId, err := primitive.ObjectIDFromHex(hashHex[:24])
	if err != nil {
		return err
	}

	record.ID = hashObjectId
	record.LastSeenHeight = 0
	defaultDate := time.Date(2018, 1, 1, 00, 00, 00, 100, time.Local)
	record.LastSeenTime = defaultDate

	// Create all tests
	if len(params.Tests) == 0 {
		return errors.New(`tests array cannot be empty`)
	}
	for _, test := range params.Tests {

		for _, task := range test.Tasks {
			// Add all tasks with the current date as maker for creation
			_ = record.AppendTask(record.ID, test.Framework, task, time.Now(), frameworkConfigMap, mongoDB, l)
		}
	}

	_, err = record.UpdateSupplier(mongoDB, l)

	return err

}

func (record *SupplierRecord) UpdateSupplier(mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	// Get suppliers collection
	suppliersCollection := mongoDB.GetCollection(types.SuppliersCollection)

	opts := options.FindOneAndUpdate().SetUpsert(true)
	suppliers_filter := bson.D{{Key: "address", Value: record.Address}, {Key: "service", Value: record.Service}}
	ctxM, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Update given struct
	update := bson.D{{Key: "$set", Value: record}}
	// Get collection and update
	var found bool = true
	err := suppliersCollection.FindOneAndUpdate(ctxM, suppliers_filter, update, opts).Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("address", record.Address).Str("service", record.Service).Msg("Supplier entry not found (UpdateSupplier). New entry created.")
			found = false
		} else {
			l.Error().Err(err).Str("address", record.Address).Str("service", record.Service).Msg("Could not retrieve supplier data from MongoDB (UpdateSupplier).")
			return false, err
		}
	}

	return found, nil
}
