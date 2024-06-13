package records

import (
	"context"
	"errors"
	"fmt"
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
// NodeRecord
//------------------------------------------------------------------------------

// DB entry of a given node-service pair
// The "Tasks" array will hold as many entries as tasks being tested
type NodeRecord struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	Address        string             `bson:"address"`
	Service        string             `bson:"service"`
	LastSeenHeight int64              `bson:"last_seen_height"`
	LastSeenTime   time.Time          `bson:"last_seen_time"`
}

func (record *NodeRecord) FindAndLoadNode(node types.NodeData, mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	// Get nodes collection
	nodesCollection := mongoDB.GetCollection(types.NodesCollection)

	// Set filtering for this node-service pair data
	node_filter := bson.D{{Key: "address", Value: node.Address}, {Key: "service", Value: node.Service}}
	opts := options.FindOne()

	// Set mongo context
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Retrieve this node entry
	var found bool = true
	cursor := nodesCollection.FindOne(ctxM, node_filter, opts)
	err := cursor.Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("address", node.Address).Str("service", node.Service).Msg("Node entry not found.")
			found = false
			// }else if err == mongo. {
		} else {
			l.Error().Msg("Could not retrieve node data from MongoDB.")
			fmt.Print(err)
			return false, err
		}
	}

	return found, nil
}

func (record *NodeRecord) AppendTask(nodeID primitive.ObjectID, framework string, task string, date time.Time, frameworkConfigMap map[string]types.FrameworkConfig, mongoDB mongodb.MongoDb, l *zerolog.Logger) TaskInterface {

	taskType, err := GetTaskType(framework, task, frameworkConfigMap, l)
	if err != nil {
		return nil
	}
	// Get the task, wich will create it if not found
	taskRecord, found := GetTaskData(nodeID, taskType, framework, task, mongoDB, l)
	if !found {
		return nil
	} else {
		return taskRecord
	}

}

func (record *NodeRecord) Init(params types.AnalyzeNodeParams, frameworkConfigMap map[string]types.FrameworkConfig, mongoDB mongodb.MongoDb, l *zerolog.Logger) error {
	// Initialize empty record

	// Set node data
	record.Address = params.Node.Address
	record.Service = params.Node.Service

	record.ID = primitive.NewObjectID()
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

	_, err := record.UpdateNode(mongoDB, l)

	return err

}

func (record *NodeRecord) UpdateNode(mongoDB mongodb.MongoDb, l *zerolog.Logger) (bool, error) {

	// Get nodes collection
	nodesCollection := mongoDB.GetCollection(types.NodesCollection)

	opts := options.FindOneAndUpdate().SetUpsert(true)
	node_filter := bson.D{{Key: "address", Value: record.Address}, {Key: "service", Value: record.Service}}
	ctxM, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Update given struct
	update := bson.D{{Key: "$set", Value: record}}
	// Get collection and update
	var found bool = true
	err := nodesCollection.FindOneAndUpdate(ctxM, node_filter, update, opts).Decode(record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			l.Warn().Str("address", record.Address).Str("service", record.Service).Msg("Node entry not found, a new one was created.")
			found = false
		} else {
			l.Error().Msg("Could not retrieve node data from MongoDB.")
			return false, err
		}
	}

	return found, nil
}
