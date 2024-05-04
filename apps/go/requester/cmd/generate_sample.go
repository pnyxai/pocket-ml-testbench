package main

import (
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"packages/mongodb"
	"packages/utils"
	"requester/types"
	"requester/x"
	"time"
)

func GetTask(node string) *types.Task {
	return &types.Task{
		Id: primitive.NewObjectID(),
		RequesterArgs: types.RequesterArgs{
			// found in localnet
			Address: node,
			Service: "0001",
			Path:    "/v1/query/height",
		},
		Done: false,
	}
}

func GetTasks(count int, node string) []*types.Task {
	docs := make([]*types.Task, count)
	for i := 0; i < count; i++ {
		docs[i] = GetTask(node)
	}
	return docs
}

func GetInstance(task *types.Task) *types.Instance {
	instance := types.Instance{
		Id:   primitive.NewObjectID(),
		Done: false,
	}
	if task != nil {
		instance.TaskId = task.Id
	}
	return &instance
}

func GetInstances(count int, task *types.Task) []*types.Instance {
	docs := make([]*types.Instance, count)
	for i := 0; i < count; i++ {
		docs[i] = GetInstance(task)
	}
	return docs
}

func GetPrompt(task *types.Task, instance *types.Instance) *types.Prompt {
	prompt := types.Prompt{
		Id: primitive.NewObjectID(),
		// ethereum mainnet mock
		Data:    "{}",
		Timeout: 120,
		Done:    false,
	}
	if task != nil {
		prompt.TaskId = task.Id
	}
	if instance != nil {
		prompt.InstanceId = instance.Id
	}
	return &prompt
}

func GetPrompts(count int, task *types.Task, instance *types.Instance) []*types.Prompt {
	prompts := make([]*types.Prompt, count)
	for i := 0; i < count; i++ {
		prompts[i] = GetPrompt(task, instance)
	}
	return prompts
}

func main() {
	// get App config
	cfg := x.LoadConfigFile()
	// initialize logger
	l := x.InitLogger(cfg)
	// initialize mongodb
	m := mongodb.NewClient(cfg.MongodbUri, []string{
		types.TaskCollection,
		types.InstanceCollection,
		types.PromptsCollection,
		types.ResponseCollection,
	}, l)
	defer m.CloseConnection()

	// start data generation
	l.Info().Msg("generating data")
	allPrompts := make([]*types.Prompt, 0)
	allInstances := make([]*types.Instance, 0)

	// this node is the one on the localnet repository
	nodes := []string{
		"7c08e2e1265246a66d7d022b163970114dda124e",
		//"9ab105b900c4633657f60974ad0e243c8f50ae1e",
		//"cb85946c8171e3bbe78f5dbc01469053419b7be1",
		//"5e6949faf0a176fd0f3a0e2ef948d7a70ee2867b",
		//"4202057f345d63b0af02f76dcb42aa46bf9b6d43",
		//"a31eba7042bd2c87c5dc0462d92dd1c961c81249",
		//"d1dd513de5a3c1f05b6c534c840f76e60caf3662",
		//"a4357688f25b1daa3270c287c0fbb75bb020c1ce",
		//"b0c626b04d5f0ab76e764409fc9bafb6cab2c1b1",
		//"b3f65b5c8da10132b107aaa1c38542ffb73dea35",
		//"e441b6024deb682291abff461bf9cc855f5ae659",
		//"d8f7226ec86e62739b84aaa8898d8b7b8c2e3025",
		//"f89f49b6a978ddfc7402b7bd0efca8715c1d7d5e",
		//"6fa859c95b450a589d1a837338c0b7ffbde6872b",
		//"3c107bcbd07db3a43882fa20c41bae5904aa0677",
		//"580751119d154cb508ac024bcab772e04c4714e2",
		//"56f4af690d1ac39b8f4c4fb9892ede2757e94624",
		//"34755f065d73a7743bf3f149660e0392b878317b",
		//"621993ee115ad88682ed401e213e7b389e296832",
		//"80f930617802d4496376b1663e91cafb515e21ad",
		//"9c3e3919baa75d8ea4d11989f6ffc25e4190d5ce",
		//"7853706d177a233401065eb09d849a77f61f153e",
		//"6260f3c4306dcf88668ceb4108621459f36f0798",
		//"111675de8e13fde1ce4da5fc236ab98ed478cc20",
	}

	allTasks := make([]*types.Task, 0)
	for _, address := range nodes {
		tasks := GetTasks(3, address)
		for j, task := range tasks {
			allTasks = append(allTasks, tasks[j])
			instances := GetInstances(12, task)
			allInstances = append(allInstances, instances...)
			for _, instance := range instances {
				prompts := GetPrompts(100, task, instance)
				allPrompts = append(allPrompts, prompts...)
			}
		}
	}

	// create a data session to ensure it will be an all or nothing
	session, sessionErr := m.StartSession()
	if sessionErr != nil {
		panic(sessionErr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, transactionErr := session.WithTransaction(ctx, func(sessCtx mongo.SessionContext) (interface{}, error) {
		tasksCollection := m.GetCollection(types.TaskCollection)
		instancesCollection := m.GetCollection(types.InstanceCollection)
		promptsCollection := m.GetCollection(types.PromptsCollection)
		opts := options.InsertManyOptions{}
		l.Info().Msg("writing tasks")
		_, err := tasksCollection.InsertMany(sessCtx, utils.InterfaceSlice[*types.Task](allTasks), opts.SetOrdered(false))
		if err != nil {
			return nil, err
		}
		l.Info().Msg("writing instances")
		_, err = instancesCollection.InsertMany(sessCtx, utils.InterfaceSlice[*types.Instance](allInstances), opts.SetOrdered(false))
		if err != nil {
			return nil, err
		}
		l.Info().Msg("writing prompts")
		_, err = promptsCollection.InsertMany(sessCtx, utils.InterfaceSlice[*types.Prompt](allPrompts), opts.SetOrdered(false))
		if err != nil {
			return nil, err
		}
		return nil, nil
	})
	if transactionErr != nil {
		panic(transactionErr)
	}

	println("done, bye!")
	os.Exit(0)
}
