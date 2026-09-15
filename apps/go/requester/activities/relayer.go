package activities

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"packages/logger"
	"packages/mongodb"
	"requester/types"
	"strings"
	"time"

	"packages/pocket"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	temporalLogger "go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
)

type RelayerParams struct {
	// inflated version of the data to avoid calling again the supplier when the activity is really called
	TargetEndpoint  pocket.Endpoint `json:"target_endpoint"`
	SupplierAddress string          `json:"supplier_address"`
	AppAddress      string          `json:"app_address"`

	// pocket relay data related that do not need to be inflated
	Service string `json:"service"`
	// SessionHeight is the block height the prompt's session STARTS at, as the
	// chain reports it in the signed session header.
	//
	// It used to be `NumBlocksPerSession * SessionNumber`, which is not a height
	// at all: poktroll numbers sessions on an anchored grid
	// (x/shared/types/session.go GetSessionNumber), so once
	// num_blocks_per_session has been changed the session number is a monotonic
	// counter and multiplying it by a block count produces a meaningless value.
	SessionHeight int64 `json:"session_height"`
	// SessionEndHeight is the last block of that session, also from the header.
	SessionEndHeight int64 `json:"session_end_height"`
	BlocksPerSession int64 `json:"blocks_per_session"`

	// requester data related
	PromptId          string  `json:"prompt_id"`
	RelayTimeout      float64 `json:"relay_timeout"`
	RelayTriggerDelay float64 `json:"relay_trigger_delay"`
}

type RelayerResponse struct {
	ResponseId string `json:"response_id"`
}

var RelayerName = "relayer"
var RelayRetries = 3
var (
	ErrPromptNotFound = errors.New("prompt not found")
)

// CanHandleRelayWithinTolerance reports whether a prompt belonging to the
// session [sessionStartHeight, sessionEndHeight] may still be relayed now that
// the chain is at currentHeight.
//
// Everything here is a real block height, taken from the signed session header
// rather than computed. That is deliberate: poktroll measures session
// boundaries against a grid anchor so that changing num_blocks_per_session does
// not misalign in-flight sessions (x/shared/types/session.go
// GetSessionStartHeight), so any boundary we derive ourselves from a plain
// height/N division is wrong on every network where that param has ever moved.
//
// The window is the session itself, widened by sessionTolerance sessions either
// side. The upper bound is the one that does the work — a prompt is queued
// during one session and relayed some time later, so the chain has usually
// moved on by the time we get here, and the tolerance says how many sessions
// past its own a relay is still worth attempting. The lower bound only fires if
// a prompt were somehow scheduled for a session that has not started.
func CanHandleRelayWithinTolerance(currentHeight, sessionStartHeight, sessionEndHeight, blocksPerSession, sessionTolerance int64, l temporalLogger.Logger) (can_handle bool) {
	tolerance := sessionTolerance * blocksPerSession
	minHeight := sessionStartHeight - tolerance
	maxHeight := sessionEndHeight + tolerance
	can_handle = minHeight <= currentHeight && currentHeight <= maxHeight
	if !can_handle {
		// Which bound failed matters: a prompt whose session has passed is the
		// ordinary case (it sat in the queue while the chain moved on, and the
		// tolerance was not generous enough), whereas one scheduled for a
		// session that has not started points at the scheduling side instead.
		// "out of session" on its own does not tell those two apart.
		reason := "session has not started yet"
		if currentHeight > maxHeight {
			reason = "session is too far in the past"
		}

		// blocksPerSession arrives in the activity params, so it can be 0 on a
		// workflow queued before it was read from chain. Guarding keeps a log
		// line from panicking on the divide.
		blocksOffBy := currentHeight - sessionEndHeight
		if currentHeight < minHeight {
			blocksOffBy = currentHeight - sessionStartHeight
		}
		sessionsOffBy := int64(0)
		if blocksPerSession > 0 {
			sessionsOffBy = blocksOffBy / blocksPerSession
		}

		l.Debug("Relay falls outside the session tolerance window",
			"reason", reason,
			"current_height", currentHeight,
			"session_start_height", sessionStartHeight,
			"session_end_height", sessionEndHeight,
			"min_allowed_height", minHeight,
			"max_allowed_height", maxHeight,
			"blocks_off_by", blocksOffBy,
			"sessions_off_by", sessionsOffBy,
			"blocks_per_session", blocksPerSession,
			"session_tolerance", sessionTolerance,
		)
	}

	return can_handle
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

func (aCtx *Ctx) Relayer(ctx context.Context, params RelayerParams) (result RelayerResponse, err error) {
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
		err = temporal.NewNonRetryableApplicationError("prompt_id must be a valid ObjectId", "BadParams", nil, params.PromptId)
		response.SetError(pocket.RelayResponseCodes.BadParams, err)
		return
	}

	response.PromptId = promptId

	if params.SessionHeight <= 0 {
		err = temporal.NewNonRetryableApplicationError("session height <= 0", "BadParams", nil, params.SessionHeight)
		response.SetError(pocket.RelayResponseCodes.BadParams, err)
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
			err = temporal.NewNonRetryableApplicationError(getPromptError.Error(), "PromptNotFound", getPromptError, params.PromptId)
			response.SetError(pocket.RelayResponseCodes.PromptNotFound, err)
			return
		}
		err = temporal.NewApplicationErrorWithCause("unexpected error reading prompt", "GetPromptWithRequesterArgs", getPromptError, params.PromptId)
		response.SetError(pocket.RelayResponseCodes.DatabaseRead, err)
		return
	}

	// fill response id ref once we have from where get them
	response.TaskId = prompt.TaskId
	response.InstanceId = prompt.InstanceId

	// get_height
	height, getHeightErr := aCtx.App.PocketClient.GetLatestBlockHeight()
	if getHeightErr != nil {
		err = temporal.NewApplicationErrorWithCause("unable to get height", "GetHeight", getHeightErr)
		response.SetError(pocket.RelayResponseCodes.PocketRpc, err)
		return
	}

	response.Height = height

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
			err = temporal.NewApplicationErrorWithCause("cannot retrieve external supplier data", "BadParams", nil, params.SupplierAddress)
			response.SetError(pocket.RelayResponseCodes.PocketRpc, err)
			return
		}

		// Edit the prompt data to change model and/or add new fields
		l.Debug("Original request", "request", prompt.Data)
		var modPromptData map[string]any
		if err = json.Unmarshal([]byte(prompt.Data), &modPromptData); err != nil {
			response.Ok = false
			response.Code = pocket.RelayResponseCodes.Relay
			response.Error = fmt.Sprintf("cannot unmarshal prompt data: %v", err)
			return
		}
		modPromptData["model"] = supplierData.ModelName
		if supplierData.ServiceTier != "" {
			// Add service tier field
			modPromptData["service_tier"] = supplierData.ServiceTier
		}
		if supplierData.TemperatureOverride >= 0 {
			// Override temperature
			modPromptData["temperature"] = supplierData.TemperatureOverride
		}
		if supplierData.NoStop != false {
			// Remove stop
			delete(modPromptData, "stop")
		}
		if supplierData.NoSeed != false {
			// Remove seed
			delete(modPromptData, "seed")
		}
		// Encode back
		modPromptDataBytes, e := json.Marshal(modPromptData)
		if e != nil {
			err = e
			response.Ok = false
			response.Code = pocket.RelayResponseCodes.Relay
			response.Error = fmt.Sprintf("cannot marshal modified prompt data: %v", err)
			return
		}
		l.Debug("Sending modified external request", "request", string(modPromptDataBytes))

		// Define the endpoint with the target path
		if supplierData.CustomApiPath != "" {
			// Replace API path
			prompt.Task.RequesterArgs.Path = strings.Replace(prompt.Task.RequesterArgs.Path, "v1", supplierData.CustomApiPath, 1)
		}
		endURL := params.TargetEndpoint.Url + prompt.Task.RequesterArgs.Path

		// Create a new request with the url, method and body
		newReq, e := http.NewRequest(prompt.Task.RequesterArgs.Method, endURL, bytes.NewBuffer(modPromptDataBytes))
		if e != nil {
			err = e
			response.Ok = false
			response.Code = pocket.RelayResponseCodes.Relay
			response.Error = fmt.Sprintf("cannot create new http request for external provider: %v", err)
			return
		}
		// Add the needed headers
		newReq.Header.Set("Content-Type", "application/json")
		for headerName, headerContent := range supplierData.Headers {
			newReq.Header.Set(headerName, headerContent)
		}
		// Do the relay
		startTime := time.Now()
		resp, e := aCtx.App.ExternalHttpClient.Do(newReq)
		if e != nil {
			err = e
			response.Ok = false
			response.Code = pocket.RelayResponseCodes.Relay
			response.Error = fmt.Sprintf("unable to send the new request: %v", err)
			return
		}
		defer resp.Body.Close()

		// Get the response
		respBody, e := io.ReadAll(resp.Body)
		response.Ms = time.Since(startTime).Milliseconds()
		if e != nil {
			err = e
			response.Ok = false
			response.Code = pocket.RelayResponseCodes.Supplier
			response.Error = fmt.Sprintf("unable to copy the response body: %v", err)
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
		if !CanHandleRelayWithinTolerance(height, params.SessionHeight, params.SessionEndHeight, params.BlocksPerSession, aCtx.App.Config.Relay.SessionTolerance, l) {
			err = temporal.NewNonRetryableApplicationError("out of session", "OutOfSession", nil)
			response.SetError(pocket.RelayResponseCodes.OutOfSession, err)
			return
		}

		// Build the payload
		thisPayload := pocket.Payload{
			Data:    prompt.Data,
			Method:  prompt.Task.RequesterArgs.Method,
			Path:    prompt.Task.RequesterArgs.Path,
			Timeout: prompt.GetTimeoutDuration() * time.Duration(RelayRetries+1),
		}

		// Send the relay to the supplier we were told to measure. Session
		// lookup, ring signing, transport and response validation all happen
		// inside the client; the supplier is pinned by a one-entry allow list,
		// so a supplier missing from the session is reported rather than
		// failed over.
		relayResponse, relayErr := aCtx.App.PocketClient.SendRelay(
			ctx,
			params.AppAddress,
			pocket.ServiceID(params.Service),
			params.SupplierAddress,
			thisPayload,
		)
		response.Ms = relayResponse.Ms

		if relayErr != nil {
			// An error occurred
			// not an rpc error
			response.Ok = false
			response.Error = relayErr.Error()
			response.Code = pocket.ResponseCode(relayErr)
			// Return, like the external branch does. Falling through would run
			// the status-code analysis below against a zero status and overwrite
			// everything set here with ok=true / Supplier / "".
			return
		}

		// Decode and assign
		statusCode = relayResponse.HTTPStatusCode
		responseString = string(relayResponse.Bytes)
	}

	// Analyze successful response
	response.Ok = true
	// TODO : Make sure that the string being written is utf8 compat
	response.Response = responseString
	if statusCode == 200 {
		// All ok
		response.Code = pocket.RelayResponseCodes.Ok
		response.Error = ""
	} else if statusCode > 200 && statusCode < 300 {
		// Non 200 success?
		response.Code = pocket.RelayResponseCodes.Ok
		response.Error = "non 200 success"
	} else if statusCode >= 400 && statusCode < 500 {
		// Client error
		response.Code = pocket.RelayResponseCodes.BadParams
		response.Error = response.Response

	} else {
		// Some other error of the supplier
		response.Code = pocket.RelayResponseCodes.Supplier
		response.Error = response.Response
	}

	return
}
