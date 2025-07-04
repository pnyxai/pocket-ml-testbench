package activities

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"packages/logger"
	"packages/mongodb"
	"requester/types"
	"strings"
	"time"

	"packages/pocket_shannon"
	shannon_types "packages/pocket_shannon/types"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.temporal.io/sdk/temporal"
)

type RelayerParams struct {
	// inflated version of the data to avoid calling again the supplier when the activity is really called
	TargetEndpoint  pocket_shannon.Endpoint `json:"target_endpoint"`
	SupplierAddress string                  `json:"supplier_address"`
	AppAddress      string                  `json:"app_address"`

	// pocket relay data related that do not need to be inflated
	Service          string `json:"service"`
	SessionHeight    int64  `json:"session_height"`
	BlocksPerSession int64  `json:"blocks_per_session"`

	// requester data related
	PromptId          string  `json:"prompt_id"`
	RelayTimeout      float64 `json:"relay_timeout"`
	RelayTriggerDelay float64 `json:"relay_trigger_delay"`
}

type RelayerResponse struct {
	ResponseId string `json:"response_id"`
}

type RelayResponseCodesEnum struct {
	Ok             int
	Relay          int
	Supplier       int
	OutOfSession   int
	BadParams      int
	PromptNotFound int
	DatabaseRead   int
	PocketRpc      int
	SignerNotFound int
	SignerError    int
	AATSignature   int
	Evaluation     int
}

var RelayResponseCodes = RelayResponseCodesEnum{
	Ok:             0,
	Relay:          1,
	Supplier:       2,
	OutOfSession:   3,
	BadParams:      4,
	PromptNotFound: 5,
	DatabaseRead:   6,
	PocketRpc:      7,
	SignerNotFound: 8,
	SignerError:    9,
	AATSignature:   10,
	Evaluation:     11,
}

var RelayerName = "relayer"
var RelayRetries = 3
var (
	ErrPromptNotFound = errors.New("prompt not found")
)

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
		{"$match", bson.M{"_id": promptId, "done": false}},
	}
	lookupStage := bson.D{
		{"$lookup", bson.M{
			"from":         tasksCollection.Name(),
			"localField":   "task_id",
			"foreignField": "_id",
			"as":           "task",
		}},
	}
	unwindStage := bson.D{
		{"$unwind", bson.M{
			"path": "$task",
		}},
	}
	limit := bson.D{
		{"$limit", 1}, // we just should load 1 document
	}
	pipeline := mongo.Pipeline{matchStage, lookupStage, unwindStage, limit}
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
	response := types.RelayResponse{Id: primitive.NewObjectID(), SessionHeight: params.SessionHeight}
	result.ResponseId = response.Id.Hex()
	defer func() {
		if response.TaskId.IsZero() {
			// we do not have to save the record here because this is before we are able to read the task id
			// so this will be created with a garbage taskId which leads to orphan response records.
			return
		}
		// persist response
		collection := aCtx.App.Mongodb.GetCollection(types.ResponseCollection)
		if err := response.Save(ctx, collection); err != nil {
			data, err2 := bson.MarshalExtJSON(response, true, false)
			if err2 != nil {
				l.Error("Error marshaling relayer response using bson", "error", err2)
			} else {
				l.Error("Error saving relayer response", "error", err, "response", data)
			}
		}
	}()

	var promptId primitive.ObjectID
	var e error

	if promptId, e = primitive.ObjectIDFromHex(params.PromptId); e != nil {
		err := temporal.NewNonRetryableApplicationError("prompt_id must be a valid ObjectId", "BadParams", nil, params.PromptId)
		response.SetError(RelayResponseCodes.BadParams, err)
		return
	}

	response.PromptId = promptId

	if params.SessionHeight <= 0 {
		err := temporal.NewNonRetryableApplicationError("session height <= 0", "BadParams", nil, params.SessionHeight)
		response.SetError(RelayResponseCodes.BadParams, err)
		return
	}

	// load prompt+task before call supplier
	promptCollection := aCtx.App.Mongodb.GetCollection(types.PromptsCollection)
	taskCollection := aCtx.App.Mongodb.GetCollection(types.TaskCollection)
	getPromptCtx, cancelFn := context.WithTimeout(ctx, 20*time.Second)
	defer cancelFn()
	prompt, getPromptError := GetPromptWithRequesterArgs(getPromptCtx, promptCollection, taskCollection, &promptId)
	if getPromptError != nil {
		if errors.Is(getPromptError, ErrPromptNotFound) {
			err := temporal.NewNonRetryableApplicationError(getPromptError.Error(), "PromptNotFound", getPromptError, params.PromptId)
			response.SetError(RelayResponseCodes.PromptNotFound, err)
			return
		}
		err := temporal.NewApplicationErrorWithCause("unexpected error reading prompt", "GetPromptWithRequesterArgs", getPromptError, params.PromptId)
		response.SetError(RelayResponseCodes.DatabaseRead, err)
		return
	}

	// fill response id ref once we have from where get them
	response.TaskId = prompt.TaskId
	response.InstanceId = prompt.InstanceId

	// get_height
	height, getHeightErr := aCtx.App.PocketFullNode.GetLatestBlockHeight()
	if getHeightErr != nil {
		err := temporal.NewApplicationErrorWithCause("unable to get height", "GetHeight", getHeightErr)
		response.SetError(RelayResponseCodes.PocketRpc, err)
		return
	}

	response.Height = height
	currentSessionHeight := GetCurrentSession(height, params.BlocksPerSession)

	// -------------------------------------------------------------------------
	// -------------------------------------------------------------------------
	// Now we will relay using a given method
	// -------------------------------------------------------------------------
	// -------------------------------------------------------------------------
	var statusCode int
	var responseString string
	if strings.HasPrefix(params.SupplierAddress, types.ExternalSupplierIdentifier) {
		// -------------------------------------------------------------------------
		// EXTERNAL
		// -------------------------------------------------------------------------
		// Send and external relay using the provided config for this supplier

		// Retrieve supplier data
		supplierData, ok := aCtx.App.ExternalSuppliers[params.SupplierAddress]
		if !ok {
			err := temporal.NewApplicationErrorWithCause("cannot retrieve external supplier data", "BadParams", nil, params.SupplierAddress)
			response.SetError(RelayResponseCodes.PocketRpc, err)
			return
		}

		// Define the endpoint with the target path
		endURL := params.TargetEndpoint.Url + prompt.Task.RequesterArgs.Path
		// Create a new request with the url, method and body
		newReq, err := http.NewRequest(prompt.Task.RequesterArgs.Method, endURL, bytes.NewBuffer([]byte(prompt.Data)))
		if err != nil {
			response.Ok = false
			response.Code = RelayResponseCodes.Relay
			response.Error = fmt.Sprintf("cannot create new http request for external provider: %w", err)
			return
		}
		// Add the needed headers
		for headerName, headerContent := range supplierData.Headers {
			newReq.Header.Set(headerName, headerContent)
		}
		// Do the relay
		startTime := time.Now()
		resp, err := aCtx.App.ExternalHttpClient.Do(newReq)
		if err != nil {
			response.Ok = false
			response.Code = RelayResponseCodes.Relay
			response.Error = fmt.Sprintf("unable to send the new request: %w", err)
			return
		}
		defer resp.Body.Close()

		// Get the response
		respBody, err := io.ReadAll(resp.Body)
		response.Ms = time.Since(startTime).Milliseconds()
		if err != nil {
			response.Code = RelayResponseCodes.Supplier
			response.Error = fmt.Sprintf("unable to copy the response body: %w", err)
			return
		}
		// Decode and assign
		statusCode = resp.StatusCode
		responseString = string(respBody)

	} else {
		// -------------------------------------------------------------------------
		// POKT NETWORK
		// -------------------------------------------------------------------------
		// Send a POKT Network relay

		// Verify if the relay is able to be dispatched base on the current session height (calculated by the height) and
		// the session height in the params. Also, contemplate the session tolerance, basically how many sessions out it will
		// anyway try to dispatch the relay.
		if !CanHandleRelayWithinTolerance(currentSessionHeight, params.SessionHeight, params.BlocksPerSession, aCtx.App.Config.Relay.SessionTolerance) {
			err := temporal.NewNonRetryableApplicationError("out of session", "OutOfSession", nil)
			response.SetError(RelayResponseCodes.OutOfSession, err)
			return
		}

		// Create a signer
		signerApp := pocket_shannon.RelayRequestSigner{
			AccountClient: *aCtx.App.PocketFullNode.GetAccountClient(),
			PrivateKeyHex: aCtx.App.PocketApps[params.AppAddress],
		}

		// Build the payload
		thisPayload := shannon_types.Payload{
			Data:    prompt.Data,
			Method:  prompt.Task.RequesterArgs.Method,
			Path:    prompt.Task.RequesterArgs.Path,
			Timeout: prompt.GetTimeoutDuration() * time.Duration(RelayRetries+1),
		}

		// Send the relay
		startTime := time.Now()
		relay, relayErr := pocket_shannon.SendRelay(thisPayload,
			params.TargetEndpoint,
			shannon_types.ServiceID(params.Service),
			*aCtx.App.PocketFullNode,
			signerApp)

		if relay == nil {
			// An error occurred
			// not an rpc error
			response.Ok = false
			response.Error = relayErr.Message
			response.Ms = time.Since(startTime).Milliseconds()

			switch relayErr.Code {
			case pocket_shannon.InvalidSessionError:
				response.Code = RelayResponseCodes.OutOfSession
			case pocket_shannon.HTTPExecutionError:
				response.Code = RelayResponseCodes.Relay
			case pocket_shannon.UnsignedRequestBuildError:
				response.Code = RelayResponseCodes.Relay
			case pocket_shannon.RequestSigningError:
				response.Code = RelayResponseCodes.SignerError
			case pocket_shannon.InvalidRelayError:
				response.Code = RelayResponseCodes.AATSignature
			default:
				response.Code = RelayResponseCodes.Relay
			}

		} else {
			// Get backend response
			relayResponse, errDeserialize := pocket_shannon.DeserializeRelayResponse(relay.Payload)
			if errDeserialize != nil {
				response.Ok = false
				response.Code = RelayResponseCodes.Supplier
				response.Error = fmt.Sprintf("Error unmarshalling endpoint response into a POKTHTTP response: %w", errDeserialize)
				return
			}
			// Decode and assign
			statusCode = relayResponse.HTTPStatusCode
			responseString = string(relayResponse.Bytes)
			response.Ms = time.Since(startTime).Milliseconds()
		}
	}

	// Analyze successful response
	response.Ok = true
	response.Response = responseString
	if statusCode == 200 {
		// All ok
		response.Code = RelayResponseCodes.Ok
		response.Error = ""
	} else if statusCode > 200 && statusCode < 300 {
		// Non 200 success?
		response.Code = RelayResponseCodes.Ok
		response.Error = "non 200 success"
	} else if statusCode >= 400 && statusCode < 500 {
		// Client error
		response.Code = RelayResponseCodes.BadParams
		response.Error = response.Response

	} else {
		// Some other error of the supplier
		response.Code = RelayResponseCodes.Supplier
		response.Error = response.Response
	}

	return
}
