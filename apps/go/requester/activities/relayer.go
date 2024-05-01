package activities

import (
	"context"
	"errors"
	poktGoProvider "github.com/pokt-foundation/pocket-go/provider"
	poktGoRelayer "github.com/pokt-foundation/pocket-go/relayer"
	poktGoSigner "github.com/pokt-foundation/pocket-go/signer"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.temporal.io/sdk/temporal"
	"packages/logger"
	"packages/mongodb"
	poktRpcCommon "packages/pocket_rpc/common"
	"requester/types"
	"time"
)

type RelayerParams struct {
	// inflated version of the data to avoid calling again the node when the activity is really called
	Session *poktGoProvider.Session `json:"session"`
	Node    *poktGoProvider.Node    `json:"node"`
	App     *poktGoProvider.App     `json:"app"`

	// pocket relay data related that do not need to be inflated
	Service          string `json:"service"`
	SessionHeight    int64  `json:"session_height"`
	BlocksPerSession int64  `json:"blocks_per_session"`

	// requester data related
	PromptId string `json:"prompt_id"`
}

type RelayerResponse struct {
	ResponseId string `json:"response_id"`
}

type RelayResponseCodesEnum struct {
	Ok             int
	Relay          int
	Node           int
	OutOfSession   int
	BadParams      int
	PromptNotFound int
	DatabaseRead   int
	PocketRpc      int
	SignerNotFound int
	SignerError    int
	AATSignature   int
}

var RelayResponseCodes = RelayResponseCodesEnum{
	Ok:             0,
	Relay:          1,
	Node:           2,
	OutOfSession:   3,
	BadParams:      4,
	PromptNotFound: 5,
	DatabaseRead:   6,
	PocketRpc:      7,
	SignerNotFound: 8,
	SignerError:    9,
	AATSignature:   10,
}

var RelayerName = "relayer"

var (
	ErrSignerNotFound = errors.New("signer not found")
	ErrPromptNotFound = errors.New("prompt not found")
)

func GetSignerOfApp(app *poktGoProvider.App, apps []string) (*poktGoSigner.Signer, error) {
	for _, privKey := range apps {
		if signer, err := poktGoSigner.NewSignerFromPrivateKey(privKey); err != nil {
			continue
		} else if signer.GetAddress() == app.Address {
			return signer, nil
		}
	}
	return nil, ErrSignerNotFound
}

func GetCurrentSession(currentHeight, blocksPerSession int64) int64 {
	currentSessionHeight := int64(0)

	if currentHeight%blocksPerSession == 0 {
		currentSessionHeight = currentHeight - blocksPerSession + 1
	} else {
		// calculate the latest session block height by diving the current block height by the blocksPerSession
		currentSessionHeight = (currentHeight/blocksPerSession)*blocksPerSession + 1
	}

	return currentSessionHeight
}

func CanHandleRelayWithinTolerance(currentSessionHeight, requestedSessionHeight, blocksPerSession, sessionTolerance int64) bool {
	tolerance := sessionTolerance * blocksPerSession
	minHeight := requestedSessionHeight - tolerance
	return minHeight <= currentSessionHeight && currentSessionHeight <= currentSessionHeight
}

func GetPromptWithRequesterArgs(ctx context.Context, promptsCollection, tasksCollection mongodb.CollectionAPI, promptId *primitive.ObjectID) (*types.Prompt, error) {
	matchStage := bson.D{
		{"$match", bson.D{{"_id", promptId}}},
	}
	lookupStage := bson.D{
		{"$lookup", bson.M{
			"from":         tasksCollection.Name(),
			"localField":   "task_id",
			"foreignField": "_id",
			"as":           "task",
		}},
		{"$unwind", bson.M{
			"path":                       "task",
			"preserveNullAndEmptyArrays": false,
		}},
	}
	limit := bson.D{
		{"$limit", 1}, // we just should load 1 document
	}
	pipeline := []bson.D{matchStage, lookupStage, limit}
	cursor, err := promptsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	docs := make([]*types.Prompt, 0)
	if e := cursor.All(ctx, &docs); e != nil {
		return nil, e
	}
	if len(docs) == 0 {
		return nil, ErrPromptNotFound
	}
	return docs[0], nil
}

func (aCtx *Ctx) Relayer(ctx context.Context, params RelayerParams) (result RelayerResponse, _ error) {
	l := logger.GetActivityLogger(RelayerName, ctx, nil)
	// create the response record id and assign to the activity result,
	// so no mater the result it will contain at least that
	response := types.RelayResponse{Id: primitive.NewObjectID()}
	result.ResponseId = response.Id.Hex()
	defer func() {
		// persist response
		collection := aCtx.App.Mongodb.GetCollection(types.ResponseCollection)
		if err := response.Save(ctx, collection); err != nil {
			data, err2 := bson.MarshalExtJSON(response, true, false)
			if err2 != nil {
				l.Error("Error marshalling relayer response using bson", "error", err2)
			} else {
				l.Error("Error saving relayer response", "error", err, "response", data)
			}
		}
	}()

	var promptId primitive.ObjectID
	var e error

	if promptId, e = primitive.ObjectIDFromHex(params.PromptId); e != nil {
		err := temporal.NewNonRetryableApplicationError("prompt_id must be a valid ObjectId", "BadParams", nil)
		response.SetError(RelayResponseCodes.BadParams, err)
		return
	}

	response.PromptId = promptId

	if params.SessionHeight <= 0 {
		err := temporal.NewNonRetryableApplicationError("session height <= 0", "BadParams", nil)
		response.SetError(RelayResponseCodes.BadParams, err)
		return
	}

	// load prompt+task before call node
	promptCollection := aCtx.App.Mongodb.GetCollection(types.PromptsCollection)
	taskCollection := aCtx.App.Mongodb.GetCollection(types.TaskCollection)
	getPromptCtx, cancelFn := context.WithTimeout(ctx, 10*time.Second)
	defer cancelFn()
	prompt, getPromptError := GetPromptWithRequesterArgs(getPromptCtx, promptCollection, taskCollection, &promptId)
	if getPromptError != nil {
		if errors.Is(getPromptError, ErrPromptNotFound) {
			err := temporal.NewNonRetryableApplicationError(getPromptError.Error(), "PromptNotFound", getPromptError, params.PromptId)
			response.SetError(RelayResponseCodes.PromptNotFound, err)
			return
		}
		err := temporal.NewApplicationErrorWithCause("unexpected error reading prompt", "GetPromptWithRequesterArgs", getPromptError)
		response.SetError(RelayResponseCodes.DatabaseRead, err)
		return
	}

	// fill response id ref once we have from where get them
	response.TaskId = prompt.TaskId
	response.InstanceId = prompt.InstanceId

	// get_height
	height, getHeightErr := aCtx.App.PocketRpc.GetHeight()
	if getHeightErr != nil {
		err := temporal.NewApplicationErrorWithCause("unable to get height", "GetHeight", getHeightErr)
		response.SetError(RelayResponseCodes.PocketRpc, err)
		return
	}

	currentSessionHeight := GetCurrentSession(height, params.BlocksPerSession)

	// Verify if the relay is able to be dispatched base on the current session height (calculated by the height) and
	// the session height in the params. Also, contemplate the session tolerance, basically how many sessions out it will
	// anyway try to dispatch the relay.
	if !CanHandleRelayWithinTolerance(currentSessionHeight, params.SessionHeight, params.BlocksPerSession, aCtx.App.Config.Rpc.SessionTolerance) {
		err := temporal.NewNonRetryableApplicationError("out of session", "OutOfSession", nil)
		response.SetError(RelayResponseCodes.OutOfSession, err)
		return
	}

	// here we get all the data needed to dispatch the relay
	signer, signerErr := GetSignerOfApp(params.App, aCtx.App.Config.Apps)

	if signerErr != nil {
		if errors.Is(signerErr, ErrSignerNotFound) {
			err := temporal.NewNonRetryableApplicationError(signerErr.Error(), "SignerNotFoundErrorCode", signerErr)
			response.SetError(RelayResponseCodes.SignerNotFound, err)
			return
		}
		err := temporal.NewApplicationErrorWithCause(signerErr.Error(), "SignerError", signerErr)
		response.SetError(RelayResponseCodes.SignerError, err)
		return
	}

	servicerUrl := params.Node.ServiceURL
	provider := poktGoProvider.NewProvider(servicerUrl, []string{servicerUrl})

	aat, aatErr := poktRpcCommon.NewPocketAATFromPrivKey(signer.GetPrivateKey())
	if aatErr != nil {
		response.SetError(RelayResponseCodes.AATSignature, aatErr)
		return
	}

	relayer := poktGoRelayer.NewRelayer(signer, provider)

	relayInput := poktGoRelayer.Input{
		Blockchain: params.Service,
		Data:       prompt.Data,
		Headers:    nil,
		Method:     prompt.Task.RequesterArgs.Method,
		Node:       params.Node,
		Path:       prompt.Task.RequesterArgs.Path,
		PocketAAT:  aat,
		Session:    params.Session,
	}
	relayOpts := &poktGoProvider.RelayRequestOptions{
		RejectSelfSignedCertificates: true,
	}
	startTime := time.Now()
	relayerCtx, cancelRelayerFn := context.WithTimeout(ctx, prompt.GetTimeoutDuration())
	defer cancelRelayerFn()
	relay, relayErr := relayer.RelayWithCtx(relayerCtx, &relayInput, relayOpts)
	response.Ms = time.Since(startTime).Milliseconds()
	if relayErr != nil {
		// not an rpc error
		response.Ok = false
		response.Error = relayErr.Error()
		var rpcError *poktGoProvider.RPCError
		var relayError *poktGoProvider.RelayError
		if errors.As(relayErr, &rpcError) {
			if rpcError.Code == 90 {
				response.Code = RelayResponseCodes.OutOfSession
			} else {
				response.Code = RelayResponseCodes.Relay
			}
		} else if errors.As(relayErr, &relayError) {
			if relayError.Code == 90 {
				response.Code = RelayResponseCodes.OutOfSession
			} else {
				response.Code = RelayResponseCodes.Relay
			}
		} else {
			response.Code = RelayResponseCodes.Node
		}
	} else {
		response.Code = RelayResponseCodes.Ok
		response.Ok = true
		response.Response = relay.RelayOutput.Response
	}

	return
}
